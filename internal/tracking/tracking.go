package tracking

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// schemaVersion is incremented when the misses table schema changes.
// On version mismatch the misses table is dropped and recreated (dev tool,
// data is not precious).
const schemaVersion = 2

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

type MissDetail struct {
	Command string
	Count   int
	AvgTok  int
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

// MissDetail returns per-command breakdown for a given prefix.
func MissDetails(prefix string) ([]MissDetail, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT command,
		       COUNT(*) as cnt,
		       CAST(AVG(output_tok) AS INTEGER) as avg_tok
		FROM misses
		WHERE prefix = ?
		GROUP BY command
		ORDER BY cnt * AVG(output_tok) DESC
	`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MissDetail
	for rows.Next() {
		var d MissDetail
		if err := rows.Scan(&d.Command, &d.Count, &d.AvgTok); err == nil {
			out = append(out, d)
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

func PrintMisses(prefix string) error {
	if prefix != "" {
		return printMissDetail(prefix)
	}
	return printMissSummary()
}

func printMissSummary() error {
	misses, err := Misses()
	if err != nil {
		return fmt.Errorf("read misses: %w", err)
	}
	if len(misses) == 0 {
		fmt.Println("no misses recorded yet")
		return nil
	}

	fmt.Printf("%-24s  %5s  %10s  %10s\n", "prefix", "seen", "avg tokens", "potential")
	fmt.Printf("%-24s  %5s  %10s  %10s\n", "------", "----", "----------", "---------")
	for _, m := range misses {
		fmt.Printf("%-24s  %5d  %10s  %10s\n",
			m.Prefix, m.Count, fmtTokens(m.AvgTok), fmtTokens(m.Potential))
	}
	fmt.Println()
	fmt.Println("zoom in: tp misses '<prefix>'")
	return nil
}

func printMissDetail(prefix string) error {
	details, err := MissDetails(prefix)
	if err != nil {
		return fmt.Errorf("read misses: %w", err)
	}
	if len(details) == 0 {
		fmt.Printf("no misses for prefix %q\n", prefix)
		return nil
	}

	total := 0
	for _, d := range details {
		total += d.Count
	}
	fmt.Printf("%s — %d occurrences\n\n", prefix, total)

	maxCmd := 0
	for _, d := range details {
		if len(d.Command) > maxCmd {
			maxCmd = len(d.Command)
		}
	}
	if maxCmd > 60 {
		maxCmd = 60
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%5s  %%10s\n", maxCmd)
	fmt.Printf(fmtStr, "command", "seen", "avg tokens")
	fmt.Printf(fmtStr, strings.Repeat("-", maxCmd), "----", "----------")
	for _, d := range details {
		cmd := d.Command
		if len(cmd) > maxCmd {
			cmd = cmd[:maxCmd-1] + "…"
		}
		fmt.Printf(fmtStr, cmd, fmt.Sprintf("%dx", d.Count), fmtTokens(d.AvgTok))
	}
	return nil
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

	// Stable tables
	if _, err := db.Exec(createCompressions); err != nil {
		db.Close()
		return nil, err
	}

	// Migrate misses table when schema changes
	var ver int
	_ = db.QueryRow("PRAGMA user_version").Scan(&ver)
	if ver < schemaVersion {
		_, _ = db.Exec("DROP TABLE IF EXISTS misses")
		_, _ = db.Exec("DROP INDEX IF EXISTS idx_miss_prefix")
		_, _ = db.Exec("DROP INDEX IF EXISTS idx_miss_ts")
		if _, err := db.Exec(createMisses); err != nil {
			db.Close()
			return nil, err
		}
		_, _ = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
	} else {
		if _, err := db.Exec(createMisses); err != nil {
			db.Close()
			return nil, err
		}
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
