package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// --- unit tests: supportsShow -------------------------------------------------

func TestSupportsShow_Plain(t *testing.T) {
	assertEq(t, "plain show", true, supportsShow(nil))
}

func TestSupportsShow_WithHash(t *testing.T) {
	assertEq(t, "with hash", true, supportsShow([]string{"a1b2c3d"}))
}

func TestSupportsShow_WithHashAndPath(t *testing.T) {
	assertEq(t, "hash -- path", true, supportsShow([]string{"a1b2c3d", "--", "pkg/foo.go"}))
}

func TestSupportsShow_FlagRejected(t *testing.T) {
	assertEq(t, "--stat rejected", false, supportsShow([]string{"--stat"}))
	assertEq(t, "--name-only rejected", false, supportsShow([]string{"--name-only"}))
	assertEq(t, "-s rejected", false, supportsShow([]string{"-s"}))
}

func TestSupportsShow_ColonNotation_Rejected(t *testing.T) {
	// git show HEAD:path/to/file shows raw file content — different format.
	assertEq(t, "colon notation rejected", false, supportsShow([]string{"HEAD:path/to/file"}))
}

// --- Supports() routing tests for git show ------------------------------------

func TestSupports_GitShow(t *testing.T) {
	h := &gitShowHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"show"}, true, "plain show"},
		{[]string{"show", "HEAD"}, true, "show HEAD"},
		{[]string{"show", "a1b2c3d"}, true, "show <hash>"},
		{[]string{"show", "HEAD", "--", "file.go"}, true, "show HEAD -- path"},
		{[]string{"-C", "/path", "show"}, true, "show with -C"},
		{[]string{"show", "--stat"}, false, "show --stat rejected"},
		{[]string{"show", "HEAD:file.go"}, false, "show colon notation rejected"},
		{[]string{"log"}, false, "wrong subcommand"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: handleShow ---------------------------------------------------

func TestHandleShow_Empty(t *testing.T) {
	result, err := handleShow("")
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	assertEq(t, "stdout", "", result.Stdout)
}

func TestHandleShow_CommitHeaderOnly(t *testing.T) {
	// git show on a commit with no diff (e.g., empty commit)
	raw := `commit a1b2c3d4e5f6789
Author: Jane Doe <jane@example.com>
Date:   Thu Mar 15 10:23:01 2026 +0000

    feat: add git diff handler

`
	result, err := handleShow(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "commit a1b2c3d") {
		t.Errorf("expected commit hash in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "Jane Doe") {
		t.Errorf("expected author in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "2026-03-15") {
		t.Errorf("expected formatted date in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "feat: add git diff handler") {
		t.Errorf("expected subject in:\n%s", result.Stdout)
	}
}

func TestHandleShow_WithDiff(t *testing.T) {
	raw := `commit a1b2c3d4e5f6789
Author: Jane Doe <jane@example.com>
Date:   Thu Mar 15 10:23:01 2026 +0000

    feat: add git diff handler

diff --git a/foo.go b/foo.go
index 111..222 100644
--- a/foo.go
+++ b/foo.go
@@ -1 +1 @@
-old
+new
`
	result, err := handleShow(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "commit a1b2c3d") {
		t.Errorf("expected commit hash in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "diff:") {
		t.Errorf("expected diff summary in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "+new") {
		t.Errorf("expected diff content in:\n%s", result.Stdout)
	}
}

// --- integration tests: real git repo ----------------------------------------

func TestIntegration_GitShow_HEAD(t *testing.T) {
	dir, _ := newTestRepo(t)
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	cmd := exec.Command("git", "show", "HEAD")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show HEAD: %v", err)
	}

	result, err := handleShow(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw git show HEAD (%d lines):\n%s", strings.Count(string(raw), "\n")+1, raw)
	t.Logf("compressed (%d lines):\n%s", strings.Count(result.Stdout, "\n")+1, result.Stdout)

	if !strings.Contains(result.Stdout, "commit ") {
		t.Errorf("expected 'commit' in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "author:") {
		t.Errorf("expected 'author:' in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "diff:") {
		t.Errorf("expected diff section in:\n%s", result.Stdout)
	}
	// Raw git headers should be stripped.
	if strings.Contains(result.Stdout, "diff --git") {
		t.Errorf("raw diff --git header should be stripped:\n%s", result.Stdout)
	}
}

func TestIntegration_GitShow_StatRejected(t *testing.T) {
	h := &gitShowHandler{}
	if h.Supports([]string{"show", "--stat"}) {
		t.Error("show --stat should be rejected by Supports()")
	}
}
