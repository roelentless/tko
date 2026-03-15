package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: parser + formatter -------------------------------------------

func TestParseStatus_Clean(t *testing.T) {
	raw := `On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean
`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "branch", "main", s.branch)
	assertEq(t, "upstream", "origin/main", s.upstream)
	assertEq(t, "ahead", 0, s.ahead)
	assertEq(t, "behind", 0, s.behind)
	assertEq(t, "staged len", 0, len(s.staged))
	assertEq(t, "unstaged len", 0, len(s.unstaged))
	assertEq(t, "untracked len", 0, len(s.untracked))

	out := formatStatus(s)
	assertEq(t, "output", "branch:main=origin/main clean", out)
}

func TestParseStatus_Ahead(t *testing.T) {
	raw := `On branch feat
Your branch is ahead of 'origin/feat' by 3 commits.
  (use "git push" to publish your local commits)

nothing to commit, working tree clean
`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "ahead", 3, s.ahead)
	assertEq(t, "behind", 0, s.behind)
	out := formatStatus(s)
	assertEq(t, "output", "branch:feat=origin/feat ↑3 clean", out)
}

func TestParseStatus_Behind(t *testing.T) {
	raw := `On branch main
Your branch is behind 'origin/main' by 2 commits, and can be fast-forwarded.
  (use "git pull" to update your local branch)

nothing to commit, working tree clean
`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "behind", 2, s.behind)
	out := formatStatus(s)
	assertEq(t, "output", "branch:main=origin/main ↓2 clean", out)
}

func TestParseStatus_Diverged(t *testing.T) {
	raw := `On branch main
Your branch and 'origin/main' have diverged,
and have 3 and 1 different commits each, respectively.
  (use "git pull" to merge the remote branch into yours)

nothing to commit, working tree clean
`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "upstream", "origin/main", s.upstream)
	assertEq(t, "ahead", 3, s.ahead)
	assertEq(t, "behind", 1, s.behind)
	out := formatStatus(s)
	assertEq(t, "output", "branch:main=origin/main ↑3↓1 clean", out)
}

func TestParseStatus_DetachedHEAD(t *testing.T) {
	raw := `HEAD detached at a1b2c3d

nothing to commit, working tree clean
`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !s.detached {
		t.Error("expected detached=true")
	}
	assertEq(t, "detachRef", "a1b2c3d", s.detachRef)
	out := formatStatus(s)
	assertEq(t, "output", "branch:HEAD@a1b2c3d clean", out)
}

func TestParseStatus_AllSections(t *testing.T) {
	raw := `On branch main

Changes to be committed:
  (use "git restore --staged <file>..." to unstage)
	modified:   pkg/foo.go
	modified:   pkg/bar.go
	new file:   pkg/new.go
	deleted:    pkg/old.go
	renamed:    pkg/orig.go -> pkg/renamed.go

Changes not staged for commit:
  (use "git restore <file>..." to discard changes in working directory)
	modified:   main.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	tmp/debug.log
	tmp/notes.txt
	scratch.txt

`
	s, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "staged len", 5, len(s.staged))
	assertEq(t, "unstaged len", 1, len(s.unstaged))
	assertEq(t, "untracked len", 3, len(s.untracked))

	// Check staged entries
	assertEq(t, "staged[0].sym", "~", s.staged[0].symbol)
	assertEq(t, "staged[0].path", "pkg/foo.go", s.staged[0].path)
	assertEq(t, "staged[2].sym", "+", s.staged[2].symbol)
	assertEq(t, "staged[2].path", "pkg/new.go", s.staged[2].path)
	assertEq(t, "staged[3].sym", "-", s.staged[3].symbol)
	assertEq(t, "staged[4].sym", "R", s.staged[4].symbol)
	assertEq(t, "staged[4].path", "pkg/orig.go→pkg/renamed.go", s.staged[4].path)

	out := formatStatus(s)
	if !strings.Contains(out, "branch:main\n") {
		t.Errorf("missing branch line in:\n%s", out)
	}
	// pkg/foo.go and pkg/bar.go — same verb+dir+ext, should be brace grouped
	if !strings.Contains(out, "modified: pkg/{foo,bar}.go") {
		t.Errorf("expected 'modified: pkg/{foo,bar}.go' in:\n%s", out)
	}
	// new file in same dir
	if !strings.Contains(out, "new: pkg/new.go") {
		t.Errorf("expected 'new: pkg/new.go' in:\n%s", out)
	}
	// deleted
	if !strings.Contains(out, "deleted: pkg/old.go") {
		t.Errorf("expected 'deleted: pkg/old.go' in:\n%s", out)
	}
	// renamed — not brace-grouped, verb label used
	if !strings.Contains(out, "renamed: pkg/orig.go→pkg/renamed.go") {
		t.Errorf("expected 'renamed: pkg/orig.go→pkg/renamed.go' in:\n%s", out)
	}
	// untracked grouped by dir
	if !strings.Contains(out, "tmp/{debug.log,notes.txt}") {
		t.Errorf("expected tmp/{debug.log,notes.txt} in:\n%s", out)
	}
	if !strings.Contains(out, "untracked(3)") {
		t.Errorf("expected untracked(3) in:\n%s", out)
	}
}

func TestGroupPaths_BraceGrouping(t *testing.T) {
	paths := []string{
		"agents/memory_extraction/",
		"agents/memory_retrieval/",
		"rfc/",
		"ui/devtools/src/components/SystemLog.tsx",
	}
	out := groupPaths(paths)
	// Two agents dirs should be grouped
	if !strings.Contains(out, "agents/{memory_extraction,memory_retrieval}/") {
		t.Errorf("expected brace grouping for agents dirs, got: %s", out)
	}
}

func TestFormatEntriesYAML_BraceGrouping(t *testing.T) {
	entries := []fileEntry{
		{symbol: "~", path: "pkg/foo.go", groupable: true},
		{symbol: "~", path: "pkg/bar.go", groupable: true},
		{symbol: "~", path: "pkg/baz.go", groupable: true},
		{symbol: "+", path: "pkg/new.go", groupable: true},
	}
	out := formatEntriesYAML(entries)
	want := "  modified: pkg/{foo,bar,baz}.go\n  new: pkg/new.go"
	if out != want {
		t.Errorf("got:  %q\nwant: %q", out, want)
	}
}

func TestFormatEntriesYAML_RootFiles(t *testing.T) {
	entries := []fileEntry{
		{symbol: "~", path: "AGENTS.md", groupable: true},
		{symbol: "~", path: "README.md", groupable: true},
	}
	out := formatEntriesYAML(entries)
	want := "  modified: {AGENTS,README}.md"
	if out != want {
		t.Errorf("got:  %q\nwant: %q", out, want)
	}
}

func TestFormatEntriesYAML_Renamed(t *testing.T) {
	entries := []fileEntry{
		{symbol: "~", path: "pkg/foo.go", groupable: true},
		{symbol: "R", path: "pkg/old.go→pkg/new.go", groupable: false},
	}
	out := formatEntriesYAML(entries)
	want := "  modified: pkg/foo.go\n  renamed: pkg/old.go→pkg/new.go"
	if out != want {
		t.Errorf("got:  %q\nwant: %q", out, want)
	}
}

// --- integration tests: real git repo -----------------------------------------

// newTestRepo creates a temp git repo with a predictable set of changes:
//
//	staged:   ~staged_modified.go  +staged_new.go  -staged_deleted.go
//	unstaged: ~unstaged_modified.go
//	untracked: untracked.txt
func newTestRepo(t *testing.T) (dir string, branch string) {
	t.Helper()
	dir = t.TempDir()

	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git setup %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	// Initial commit: all files that will be modified/deleted later
	write("staged_modified.go", "original\n")
	write("staged_deleted.go", "to be deleted\n")
	write("unstaged_modified.go", "original\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Staged changes
	write("staged_modified.go", "modified\n")
	run("git", "add", "staged_modified.go")

	write("staged_new.go", "new file\n")
	run("git", "add", "staged_new.go")

	run("git", "rm", "staged_deleted.go")

	// Unstaged change
	write("unstaged_modified.go", "changed\n")

	// Untracked file
	write("untracked.txt", "untracked\n")

	// Get actual branch name (varies by git config)
	branch = run("git", "rev-parse", "--abbrev-ref", "HEAD")
	return dir, branch
}

func TestIntegration_GitStatus(t *testing.T) {
	dir, branch := newTestRepo(t)

	// Run real git status and capture
	cmd := exec.Command("git", "status")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}

	// Parse and format
	s, err := parseStatus(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	out := formatStatus(s)

	t.Logf("raw git status (%d lines):\n%s", strings.Count(string(raw), "\n")+1, raw)
	t.Logf("compressed (%d lines):\n%s", strings.Count(out, "\n")+1, out)

	// Assert structure
	assertEq(t, "branch", branch, s.branch)
	assertEq(t, "staged len", 3, len(s.staged))
	assertEq(t, "unstaged len", 1, len(s.unstaged))
	assertEq(t, "untracked len", 1, len(s.untracked))

	// Assert compressed output
	if !strings.HasPrefix(out, "branch:"+branch+"\n") {
		t.Errorf("unexpected branch line in:\n%s", out)
	}
	if !strings.Contains(out, "staged(3):") {
		t.Errorf("expected staged(3) in:\n%s", out)
	}
	if !strings.Contains(out, "unstaged(1):") {
		t.Errorf("expected unstaged(1) in:\n%s", out)
	}
	if !strings.Contains(out, "untracked(1):") {
		t.Errorf("expected untracked(1) in:\n%s", out)
	}
	if !strings.Contains(out, "modified: staged_modified.go") {
		t.Errorf("missing 'modified: staged_modified.go' in:\n%s", out)
	}
	if !strings.Contains(out, "new: staged_new.go") {
		t.Errorf("missing 'new: staged_new.go' in:\n%s", out)
	}
	if !strings.Contains(out, "deleted: staged_deleted.go") {
		t.Errorf("missing 'deleted: staged_deleted.go' in:\n%s", out)
	}
	if !strings.Contains(out, "modified: unstaged_modified.go") {
		t.Errorf("missing 'modified: unstaged_modified.go' in:\n%s", out)
	}
	if !strings.Contains(out, "untracked.txt") {
		t.Errorf("missing untracked.txt in:\n%s", out)
	}
}

func TestIntegration_GitStatus_Clean(t *testing.T) {
	dir := t.TempDir()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	cmd := exec.Command("git", "status")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}

	s, err := parseStatus(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	out := formatStatus(s)
	if !strings.Contains(out, "clean") {
		t.Errorf("expected 'clean' in output: %q", out)
	}
	if strings.Count(out, "\n") > 0 {
		t.Errorf("clean status should be single line, got:\n%s", out)
	}
}

// --- helpers ------------------------------------------------------------------

func assertEq[T comparable](t *testing.T, label string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", label, want, got)
	}
}
