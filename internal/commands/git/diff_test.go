package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: parser -------------------------------------------------------

func TestParseDiff_Simple(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index 83db48f..4d1cd80 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 line1
 line2
-line3
+modified
+line4
`
	files := parseDiffFiles(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	assertEq(t, "path", "foo.go", f.path)
	assertEq(t, "added", 2, f.added)
	assertEq(t, "removed", 1, f.removed)
	assertEq(t, "isNew", false, f.isNew)
	assertEq(t, "isDeleted", false, f.isDeleted)
	assertEq(t, "isBinary", false, f.isBinary)
	if !strings.Contains(f.content, "@@ -1,3 +1,4 @@") {
		t.Errorf("expected hunk header in content, got: %q", f.content)
	}
	if !strings.Contains(f.content, "+modified") {
		t.Errorf("expected +modified in content, got: %q", f.content)
	}
}

func TestParseDiff_NewFile(t *testing.T) {
	raw := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..b66ba06
--- /dev/null
+++ b/new.go
@@ -0,0 +1 @@
+new content
`
	files := parseDiffFiles(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	assertEq(t, "path", "new.go", f.path)
	assertEq(t, "isNew", true, f.isNew)
	assertEq(t, "added", 1, f.added)
	assertEq(t, "removed", 0, f.removed)
}

func TestParseDiff_DeletedFile(t *testing.T) {
	raw := `diff --git a/old.go b/old.go
deleted file mode 100644
index abc1234..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
`
	files := parseDiffFiles(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	assertEq(t, "path", "old.go", f.path)
	assertEq(t, "isDeleted", true, f.isDeleted)
	assertEq(t, "removed", 3, f.removed)
	assertEq(t, "added", 0, f.added)
}

func TestParseDiff_RenamedFile(t *testing.T) {
	raw := `diff --git a/old.go b/new.go
similarity index 80%
rename from old.go
rename to new.go
--- a/old.go
+++ b/new.go
@@ -1,3 +1,3 @@
 line1
-oldline
+newline
 line3
`
	files := parseDiffFiles(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	assertEq(t, "path", "new.go", f.path)
	assertEq(t, "oldPath", "old.go", f.oldPath)
	assertEq(t, "added", 1, f.added)
	assertEq(t, "removed", 1, f.removed)
}

func TestParseDiff_Binary(t *testing.T) {
	raw := `diff --git a/img.png b/img.png
index abc1234..def5678 100644
Binary files a/img.png and b/img.png differ
`
	files := parseDiffFiles(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	assertEq(t, "path", "img.png", f.path)
	assertEq(t, "isBinary", true, f.isBinary)
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	raw := `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1 +1 @@
-old
+new
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/b.go
@@ -0,0 +1 @@
+brand new
`
	files := parseDiffFiles(raw)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	assertEq(t, "file[0].path", "a.go", files[0].path)
	assertEq(t, "file[1].path", "b.go", files[1].path)
	assertEq(t, "file[1].isNew", true, files[1].isNew)
}

func TestHandleDiff_Empty(t *testing.T) {
	result, err := handleDiff("")
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "stdout", "", result.Stdout)
	assertEq(t, "lossless", true, result.Lossless)
}

func TestHandleDiff_Lossless(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index 83db48f..4d1cd80 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 line1
 line2
-line3
+modified
+line4
diff --git a/bar.go b/bar.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/bar.go
@@ -0,0 +1 @@
+hello
`
	result, err := handleDiff(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.HasPrefix(result.Stdout, "diff: 2 files +3 -1\n") {
		t.Errorf("unexpected header: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "--- foo.go +2 -1\n") {
		t.Errorf("missing foo.go header in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "--- bar.go +1 -0 (new)\n") {
		t.Errorf("missing bar.go header in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "+modified") {
		t.Errorf("missing +modified in:\n%s", result.Stdout)
	}
}

func TestHandleDiff_LargeTruncated(t *testing.T) {
	// Build a file diff that exceeds diffFileTruncateLines
	var sb strings.Builder
	sb.WriteString("diff --git a/big.go b/big.go\nindex 111..222 100644\n--- a/big.go\n+++ b/big.go\n@@ -1,1 +1,400 @@\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "+line%d\n", i)
	}

	result, err := handleDiff(sb.String())
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", false, result.Lossless)
	if !strings.Contains(result.Stdout, "truncated") {
		t.Errorf("expected 'truncated' in output:\n%s", result.Stdout)
	}
}

func TestHandleDiff_Binary(t *testing.T) {
	raw := `diff --git a/img.png b/img.png
index abc..def 100644
Binary files a/img.png and b/img.png differ
`
	result, err := handleDiff(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "[binary]") {
		t.Errorf("expected [binary] in output:\n%s", result.Stdout)
	}
}

func TestHandleDiff_Rename(t *testing.T) {
	raw := `diff --git a/old.go b/new.go
similarity index 80%
rename from old.go
rename to new.go
--- a/old.go
+++ b/new.go
@@ -1 +1 @@
-foo
+bar
`
	result, err := handleDiff(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "old.go → new.go") {
		t.Errorf("expected rename arrow in output:\n%s", result.Stdout)
	}
}

// --- integration tests: real git repo -----------------------------------------

func TestIntegration_GitDiff_Unstaged(t *testing.T) {
	dir, gitEnv := newDiffTestRepo(t)

	cmd := exec.Command("git", "diff")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}

	result, err := handleDiff(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw git diff (%d lines):\n%s", strings.Count(string(raw), "\n")+1, raw)
	t.Logf("compressed (%d lines):\n%s", strings.Count(result.Stdout, "\n")+1, result.Stdout)

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.HasPrefix(result.Stdout, "diff: 1 file") {
		t.Errorf("expected single-file diff header, got: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "modified.go") {
		t.Errorf("expected modified.go in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "+changed") {
		t.Errorf("expected +changed line in:\n%s", result.Stdout)
	}
	if strings.Contains(result.Stdout, "diff --git") {
		t.Errorf("raw git headers should be stripped from:\n%s", result.Stdout)
	}
	if strings.Contains(result.Stdout, "index ") {
		t.Errorf("index lines should be stripped from:\n%s", result.Stdout)
	}
}

func TestIntegration_GitDiff_Staged(t *testing.T) {
	dir, gitEnv := newDiffTestRepo(t)

	cmd := exec.Command("git", "diff", "--staged")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff --staged: %v", err)
	}

	result, err := handleDiff(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "new.go") {
		t.Errorf("expected new.go in staged diff:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "(new)") {
		t.Errorf("expected (new) flag for new file:\n%s", result.Stdout)
	}
}

func TestIntegration_GitDiff_Head(t *testing.T) {
	dir, gitEnv := newDiffTestRepo(t)

	// Compare first and second commit
	cmd := exec.Command("git", "diff", "HEAD~1", "HEAD")
	cmd.Dir = dir
	cmd.Env = gitEnv
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff HEAD~1 HEAD: %v", err)
	}

	result, err := handleDiff(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "diff:") {
		t.Errorf("expected diff: header:\n%s", result.Stdout)
	}
}

// newDiffTestRepo creates a temp git repo with staged/unstaged changes.
//
//	staged:   new.go (new file)
//	unstaged: modified.go (changed content)
func newDiffTestRepo(t *testing.T) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()

	env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
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

	// Initial commit
	write("modified.go", "original\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Second commit (for HEAD~1 test)
	write("second.go", "second commit\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "second")

	// Staged: new file
	write("new.go", "new content\n")
	run("git", "add", "new.go")

	// Unstaged: modified file
	write("modified.go", "changed\n")

	return dir, env
}
