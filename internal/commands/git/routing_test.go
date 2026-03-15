package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSupports_ContextFlags verifies that global git flags (-C, -c, etc.) are
// stripped for routing purposes. The handler must match the same invocations
// whether or not global flags are present.
func TestSupports_ContextFlags(t *testing.T) {
	status := &gitStatusHandler{}
	diff := &gitDiffHandler{}
	log := &gitLogHandler{}
	show := &gitShowHandler{}

	cases := []struct {
		handler interface{ Supports([]string) bool }
		args    []string
		want    bool
		label   string
	}{
		// git status — plain and with -C
		{status, []string{"status"}, true, "plain status"},
		{status, []string{"-C", "/some/path", "status"}, true, "status with -C"},
		{status, []string{"-c", "core.pager=cat", "status"}, true, "status with -c"},
		{status, []string{"--no-pager", "status"}, true, "status with --no-pager"},
		{status, []string{"-C", "/path", "status", "--short"}, false, "status --short rejected"},
		{status, []string{"--unknown-flag", "status"}, false, "unknown global flag rejected"},

		// git diff — plain and with -C
		{diff, []string{"diff"}, true, "plain diff"},
		{diff, []string{"-C", "/some/path", "diff"}, true, "diff with -C"},
		{diff, []string{"-C", "/some/path", "diff", "--cached"}, true, "diff --cached with -C"},
		{diff, []string{"-C", "/path", "diff", "--word-diff"}, false, "diff --word-diff rejected"},

		// git log — plain and with -C
		{log, []string{"log"}, true, "plain log"},
		{log, []string{"-C", "/some/path", "log"}, true, "log with -C"},
		{log, []string{"log", "--oneline"}, true, "log --oneline"},
		{log, []string{"log", "-n", "5"}, true, "log -n 5"},
		{log, []string{"log", "--format=%H"}, false, "log --format rejected"},
		{log, []string{"log", "HEAD~5..HEAD"}, false, "log range rejected"},

		// git show — plain and with -C
		{show, []string{"show"}, true, "plain show"},
		{show, []string{"-C", "/some/path", "show"}, true, "show with -C"},
		{show, []string{"show", "HEAD"}, true, "show HEAD"},
		{show, []string{"show", "--stat"}, false, "show --stat rejected"},
		{show, []string{"show", "HEAD:file.go"}, false, "show colon notation rejected"},
	}

	for _, c := range cases {
		got := c.handler.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.label, c.args, got, c.want)
		}
	}
}

// TestIntegration_GitStatus_WithC verifies that git -C <path> status produces
// the same compressed output as git status run from that directory.
// This also acts as a contract test: the handler must not modify the command —
// the runner executes it as-is with the original args.
func TestIntegration_GitStatus_WithC(t *testing.T) {
	dir, branch := newTestRepo(t)

	// Run via -C from a different directory
	cmd := exec.Command("git", "-C", dir, "status")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git -C status: %v", err)
	}

	s, err := parseStatus(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	out := formatStatus(s)

	assertEq(t, "branch", branch, s.branch)
	if !strings.Contains(out, "staged(3):") {
		t.Errorf("expected staged(3) in:\n%s", out)
	}
	if !strings.Contains(out, "unstaged(1):") {
		t.Errorf("expected unstaged(1) in:\n%s", out)
	}
}
