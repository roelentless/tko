package git

import (
	"fmt"
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

