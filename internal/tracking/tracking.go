package tracking

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const createCompressions = `
CREATE TABLE IF NOT EXISTS compressions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         INTEGER NOT NULL,
	command    TEXT    NOT NULL,
	input_tok  INTEGER NOT NULL,
	output_tok INTEGER NOT NULL,
	saved_tok  INTEGER NOT NULL,
	lossless   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comp_ts      ON compressions (ts);
CREATE INDEX IF NOT EXISTS idx_comp_command ON compressions (command);
`

const createMisses = `
CREATE TABLE IF NOT EXISTS misses (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         INTEGER NOT NULL,
	prefix     TEXT    NOT NULL,
	command    TEXT    NOT NULL,
	output_tok INTEGER NOT NULL,
	exit_code  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_miss_prefix ON misses (prefix);
CREATE INDEX IF NOT EXISTS idx_miss_ts     ON misses (ts);
`

// Reset deletes the tracking database (both stats and misses).
func Reset() error {
	path, err := dbPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ResetMisses clears only the misses table, preserving compression stats.
func ResetMisses() error {
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`DELETE FROM misses`)
	return err
}

// Record saves a compression event. Best-effort: errors are silently dropped.
func Record(command string, inputTok, outputTok int, lossless bool) {
	db, err := open()
	if err != nil {
		return
	}
	defer db.Close()

	saved := inputTok - outputTok
	if saved < 0 {
		saved = 0
	}
	loss := 0
	if !lossless {
		loss = 1
	}
	_, _ = db.Exec(
		`INSERT INTO compressions (ts, command, input_tok, output_tok, saved_tok, lossless) VALUES (?,?,?,?,?,?)`,
		time.Now().Unix(), command, inputTok, outputTok, saved, loss,
	)
}

// RecordMiss records a passthrough command that had no handler.
// prefix is the canonical command prefix ("git log"), command is the full
// invocation ("git log --oneline -20"), outputBytes is stdout byte count.
func RecordMiss(prefix, command string, outputBytes, exitCode int) {
	db, err := open()
	if err != nil {
		return
	}
	defer db.Close()
	outputTok := (outputBytes + 3) / 4
	_, _ = db.Exec(
		`INSERT INTO misses (ts, prefix, command, output_tok, exit_code) VALUES (?,?,?,?,?)`,
		time.Now().Unix(), prefix, command, outputTok, exitCode,
	)
}

// --- stats types --------------------------------------------------------------

type Summary struct {
	Runs     int
	SavedTok int
}

type CommandStat struct {
	Command  string
	Runs     int
	SavedTok int
	AvgSaved int
}

type MissSummary struct {
	Prefix    string
	Count     int
	AvgTok    int
	Potential int // Count * AvgTok — total tokens we could have saved
}

// --- queries ------------------------------------------------------------------

func Stats() (allTime, today Summary, byCmd []CommandStat, err error) {
	db, err := open()
	if err != nil {
		return
	}
	defer db.Close()

	row := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(saved_tok),0) FROM compressions`)
	_ = row.Scan(&allTime.Runs, &allTime.SavedTok)

	midnight := time.Now().Truncate(24 * time.Hour).Unix()
	row = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(saved_tok),0) FROM compressions WHERE ts >= ?`, midnight)
	_ = row.Scan(&today.Runs, &today.SavedTok)

	rows, qerr := db.Query(`
		SELECT command, COUNT(*), SUM(saved_tok), CAST(AVG(saved_tok) AS INTEGER)
		FROM compressions
		GROUP BY command
		ORDER BY SUM(saved_tok) DESC
	`)
	if qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var s CommandStat
			if serr := rows.Scan(&s.Command, &s.Runs, &s.SavedTok, &s.AvgSaved); serr == nil {
				byCmd = append(byCmd, s)
			}
		}
	}
	return
}

// Misses returns per-prefix miss summaries, ordered by optimization potential.
func Misses() ([]MissSummary, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT prefix,
		       COUNT(*) as cnt,
		       CAST(AVG(output_tok) AS INTEGER) as avg_tok,
		       CAST(COUNT(*) * AVG(output_tok) AS INTEGER) as potential
		FROM misses
		GROUP BY prefix
		ORDER BY potential DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MissSummary
	for rows.Next() {
		var m MissSummary
		if err := rows.Scan(&m.Prefix, &m.Count, &m.AvgTok, &m.Potential); err == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// --- printers -----------------------------------------------------------------

func PrintStats() error {
	allTime, today, byCmd, err := Stats()
	if err != nil {
		return fmt.Errorf("read stats: %w", err)
	}

	misses, _ := Misses()

	if allTime.Runs == 0 && len(misses) == 0 {
		fmt.Println("no data yet — run some commands through tp")
		return nil
	}

	fmt.Printf("all-time  %4d compressions   %s tokens saved\n",
		allTime.Runs, fmtTokens(allTime.SavedTok))
	fmt.Printf("today     %4d compressions   %s tokens saved\n",
		today.Runs, fmtTokens(today.SavedTok))

	if len(byCmd) > 0 {
		fmt.Println()
		fmt.Printf("%-24s  %5s  %10s  %11s\n", "compressed", "runs", "avg saved", "total saved")
		fmt.Printf("%-24s  %5s  %10s  %11s\n", "----------", "----", "---------", "-----------")
		for _, s := range byCmd {
			fmt.Printf("%-24s  %5d  %9s  %11s\n",
				s.Command, s.Runs, fmtTokens(s.AvgSaved), fmtTokens(s.SavedTok))
		}
	}

	if len(misses) > 0 {
		fmt.Println()
		fmt.Printf("%d missed prefixes — run 'tp misses' for details\n", len(misses))
	}

	return nil
}

type RawMiss struct {
	TS        int64
	Prefix    string
	Command   string
	OutputTok int
	ExitCode  int
}

func PrintMisses(prefix string) error {
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()

	var rows *sql.Rows
	if prefix != "" {
		rows, err = db.Query(`SELECT ts, prefix, command, output_tok, exit_code FROM misses WHERE prefix = ? ORDER BY ts`, prefix)
	} else {
		rows, err = db.Query(`SELECT ts, prefix, command, output_tok, exit_code FROM misses ORDER BY ts`)
	}
	if err != nil {
		return fmt.Errorf("read misses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m RawMiss
		if err := rows.Scan(&m.TS, &m.Prefix, &m.Command, &m.OutputTok, &m.ExitCode); err != nil {
			return err
		}
		fmt.Println(m.Command)
	}
	return rows.Err()
}

// --- internal -----------------------------------------------------------------

func open() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(createCompressions); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(createMisses); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tko", "tracking.db"), nil
}

func fmtTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
