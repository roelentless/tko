package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// --- unit tests: parseLogFlags ------------------------------------------------

func TestParseLogFlags_Plain(t *testing.T) {
	oneline, maxCount, ok := parseLogFlags(nil)
	assertEq(t, "ok", true, ok)
	assertEq(t, "oneline", false, oneline)
	assertEq(t, "maxCount", -1, maxCount)
}

func TestParseLogFlags_OneLine(t *testing.T) {
	oneline, _, ok := parseLogFlags([]string{"--oneline"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "oneline", true, oneline)
}

func TestParseLogFlags_MaxCountLong(t *testing.T) {
	_, maxCount, ok := parseLogFlags([]string{"--max-count=5"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "maxCount", 5, maxCount)
}

func TestParseLogFlags_MaxCountShortSep(t *testing.T) {
	_, maxCount, ok := parseLogFlags([]string{"-n", "10"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "maxCount", 10, maxCount)
}

func TestParseLogFlags_MaxCountShortCombined(t *testing.T) {
	_, maxCount, ok := parseLogFlags([]string{"-n5"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "maxCount", 5, maxCount)
}

func TestParseLogFlags_OneLineWithN(t *testing.T) {
	oneline, maxCount, ok := parseLogFlags([]string{"--oneline", "-n", "3"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "oneline", true, oneline)
	assertEq(t, "maxCount", 3, maxCount)
}

func TestParseLogFlags_UnknownFlag(t *testing.T) {
	_, _, ok := parseLogFlags([]string{"--format=%H"})
	assertEq(t, "ok", false, ok)
}

func TestParseLogFlags_PositionalArg(t *testing.T) {
	_, _, ok := parseLogFlags([]string{"HEAD~5..HEAD"})
	assertEq(t, "ok", false, ok)
}

// --- unit tests: handleLog / parseLogEntries ----------------------------------

func TestHandleLog_Empty(t *testing.T) {
	result, err := handleLog("", false, -1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "0 commits") {
		t.Errorf("expected '0 commits' in: %q", result.Stdout)
	}
}

func TestHandleLog_OneLine(t *testing.T) {
	raw := "a1b2c3d feat: add thing\nb2c3d4e fix: bug\n"
	result, err := handleLog(raw, true, -1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "a1b2c3d") {
		t.Errorf("expected hash in output: %q", result.Stdout)
	}
}

func TestHandleLog_SmallRepo_Lossless(t *testing.T) {
	raw := buildFakeLog(3)
	result, err := handleLog(raw, false, -1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "3 commits") {
		t.Errorf("expected '3 commits' in: %s", result.Stdout)
	}
}

func TestHandleLog_LargeRepo_Lossy(t *testing.T) {
	raw := buildFakeLog(logDisplayThreshold + 5)
	result, err := handleLog(raw, false, -1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", false, result.Lossless)
	if !strings.Contains(result.Stdout, "showing") {
		t.Errorf("expected 'showing' in lossy output: %s", result.Stdout)
	}
}

func TestHandleLog_ExplicitNSmall_Lossless(t *testing.T) {
	raw := buildFakeLog(5)
	result, err := handleLog(raw, false, 5)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
}

func TestHandleLog_ExplicitNLarge_Lossy(t *testing.T) {
	raw := buildFakeLog(logDisplayThreshold + 10)
	result, err := handleLog(raw, false, logDisplayThreshold+10)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", false, result.Lossless)
}

func TestParseGitDate(t *testing.T) {
	got := parseGitDate("Thu Mar 15 10:23:01 2026 +0000")
	assertEq(t, "date", "2026-03-15", got)
}

// buildFakeLog produces n fake git log entries in default format.
func buildFakeLog(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("commit abcdef1234567890\n")
		sb.WriteString("Author: Test User <test@test.com>\n")
		sb.WriteString("Date:   Thu Mar 15 10:23:01 2026 +0000\n")
		sb.WriteString("\n")
		sb.WriteString("    commit message here\n")
		sb.WriteString("\n")
	}
	return sb.String()
}

// --- Supports() routing tests for git log ------------------------------------

func TestSupports_GitLog(t *testing.T) {
	h := &gitLogHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"log"}, true, "plain log"},
		{[]string{"log", "--oneline"}, true, "log --oneline"},
		{[]string{"log", "-n", "5"}, true, "log -n 5"},
		{[]string{"log", "--max-count=10"}, true, "log --max-count=10"},
		{[]string{"log", "--oneline", "-n", "3"}, true, "log --oneline -n 3"},
		{[]string{"-C", "/path", "log"}, true, "log with -C"},
		{[]string{"log", "--format=%H"}, false, "log --format rejected"},
		{[]string{"log", "HEAD~5..HEAD"}, false, "log range rejected"},
		{[]string{"status"}, false, "wrong subcommand"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- integration tests: real git repo ----------------------------------------

func TestIntegration_GitLog_Plain_Lossy(t *testing.T) {
	dir, _ := newTestRepo(t)
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	// Add enough commits to exceed the display threshold.
	for i := 0; i < logDisplayThreshold; i++ {
		cmd := exec.Command("git", "commit", "--allow-empty", "-m", "extra commit")
		cmd.Dir = dir
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, out)
		}
	}

	cmd := exec.Command("git", "log")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}

	result, err := handleLog(string(raw), false, -1)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(t, "lossless", false, result.Lossless)
	if !strings.Contains(result.Stdout, "showing") {
		t.Errorf("expected 'showing N' in lossy output:\n%s", result.Stdout)
	}

	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	// summary line + up to logDisplayThreshold entry lines
	if len(lines) < 2 {
		t.Errorf("expected summary + entries, got:\n%s", result.Stdout)
	}
	// Each entry line should contain a date in YYYY-MM-DD format.
	entryLine := lines[1]
	if !strings.Contains(entryLine, "-") {
		t.Errorf("expected date in entry line: %q", entryLine)
	}
	t.Logf("compressed log:\n%s", result.Stdout)
}

func TestIntegration_GitLog_SmallN_Lossless(t *testing.T) {
	dir, _ := newTestRepo(t)
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	cmd := exec.Command("git", "log", "-n", "1")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log -n 1: %v", err)
	}

	result, err := handleLog(string(raw), false, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if strings.Contains(result.Stdout, "showing") {
		t.Errorf("lossless log should not say 'showing':\n%s", result.Stdout)
	}
}

func TestIntegration_GitLog_OneLine(t *testing.T) {
	dir, _ := newTestRepo(t)
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log --oneline: %v", err)
	}

	result, err := handleLog(string(raw), true, -1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if strings.TrimSpace(result.Stdout) == "" {
		t.Error("expected non-empty oneline output")
	}
	t.Logf("compressed --oneline:\n%s", result.Stdout)
}
