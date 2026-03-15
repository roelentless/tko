package ls

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: parseLSFlags -------------------------------------------------

func TestParseLSFlags_Plain(t *testing.T) {
	long, ok := parseLSFlags(nil)
	assertEq(t, "ok", true, ok)
	assertEq(t, "long", false, long)
}

func TestParseLSFlags_WithPath(t *testing.T) {
	long, ok := parseLSFlags([]string{"/some/path"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "long", false, long)
}

func TestParseLSFlags_LongFlag(t *testing.T) {
	long, ok := parseLSFlags([]string{"-l"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "long", true, long)
}

func TestParseLSFlags_LA(t *testing.T) {
	long, ok := parseLSFlags([]string{"-la"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "long", true, long)
}

func TestParseLSFlags_AL(t *testing.T) {
	long, ok := parseLSFlags([]string{"-al"})
	assertEq(t, "ok", true, ok)
	assertEq(t, "long", true, long)
}

func TestParseLSFlags_Recursive_Rejected(t *testing.T) {
	_, ok := parseLSFlags([]string{"-R"})
	assertEq(t, "ok", false, ok)
}

func TestParseLSFlags_Color_Rejected(t *testing.T) {
	_, ok := parseLSFlags([]string{"--color=auto"})
	assertEq(t, "ok", false, ok)
}

func TestParseLSFlags_UnknownShort_Rejected(t *testing.T) {
	_, ok := parseLSFlags([]string{"-h"})
	assertEq(t, "ok", false, ok)
}

// --- Supports() routing tests -------------------------------------------------

func TestSupports_LS(t *testing.T) {
	h := &lsHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{nil, true, "plain ls"},
		{[]string{"/path"}, true, "ls <path>"},
		{[]string{"-l"}, true, "ls -l"},
		{[]string{"-la"}, true, "ls -la"},
		{[]string{"-al"}, true, "ls -al"},
		{[]string{"-la", "/path"}, true, "ls -la <path>"},
		{[]string{"-R"}, false, "ls -R rejected"},
		{[]string{"--color=auto"}, false, "ls --color rejected"},
		{[]string{"-h"}, false, "ls -h rejected"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: handleLS -----------------------------------------------------

func TestHandleLS_Empty(t *testing.T) {
	result, err := handleLS("", false)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "empty") {
		t.Errorf("expected 'empty' in: %q", result.Stdout)
	}
}

func TestHandleLS_Plain(t *testing.T) {
	raw := "file1.go\nfile2.go\nREADME.md\n"
	result, err := handleLS(raw, false)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "3 items") {
		t.Errorf("expected '3 items' in: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "file1.go") {
		t.Errorf("expected filenames in: %q", result.Stdout)
	}
	// Should be a single line.
	if strings.Count(result.Stdout, "\n") > 1 {
		t.Errorf("plain ls output should be single line, got:\n%s", result.Stdout)
	}
}

func TestHandleLS_Long(t *testing.T) {
	// Simulate ls -la output (macOS/Linux format)
	raw := `total 48
drwxr-xr-x  5 user group  160 Mar 15 10:23 .
drwxr-xr-x  3 user group   96 Mar 14 09:00 ..
-rw-r--r--  1 user group    0 Mar 15 10:23 .gitignore
drwxr-xr-x  3 user group   96 Mar 15 10:23 cmd
-rw-r--r--  1 user group  234 Mar 15 10:23 go.mod
-rw-r--r--  1 user group 1234 Mar 15 10:23 go.sum
-rw-r--r--  1 user group  100 Mar 15 10:23 README.md
`
	result, err := handleLS(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "dirs(1):") {
		t.Errorf("expected dirs(1) in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "cmd/") {
		t.Errorf("expected cmd/ in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "hidden(1):") {
		t.Errorf("expected hidden(1) in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, ".gitignore") {
		t.Errorf("expected .gitignore in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "total: 5 items") {
		t.Errorf("expected total: 5 items in:\n%s", result.Stdout)
	}
	// Permissions should be stripped.
	if strings.Contains(result.Stdout, "drwx") || strings.Contains(result.Stdout, "-rw-") {
		t.Errorf("permission strings should be stripped:\n%s", result.Stdout)
	}
}

func TestGroupFileNames_Extensions(t *testing.T) {
	names := []string{"foo.go", "bar.go", "baz.go", "README.md", "Makefile"}
	got := groupFileNames(names)
	if !strings.Contains(got, "*.go(3)") {
		t.Errorf("expected *.go(3) in: %q", got)
	}
	if !strings.Contains(got, "README.md") {
		t.Errorf("expected README.md in: %q", got)
	}
	if !strings.Contains(got, "Makefile") {
		t.Errorf("expected Makefile in: %q", got)
	}
}

func TestGroupFileNames_UniqueExt(t *testing.T) {
	names := []string{"go.mod", "go.sum"}
	got := groupFileNames(names)
	// Both have unique extensions — listed individually.
	if !strings.Contains(got, "go.mod") {
		t.Errorf("expected go.mod in: %q", got)
	}
	if !strings.Contains(got, "go.sum") {
		t.Errorf("expected go.sum in: %q", got)
	}
}

// --- integration tests: real ls binary ----------------------------------------

func TestIntegration_LS_Plain(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("ls", dir)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("ls: %v", err)
	}

	result, err := handleLS(string(raw), false)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw ls: %q", raw)
	t.Logf("compressed: %q", result.Stdout)

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "items") {
		t.Errorf("expected item count in: %q", result.Stdout)
	}
	// Verify count matches actual files (5 files + 1 dir = 6 items).
	if !strings.Contains(result.Stdout, "6 items") {
		t.Errorf("expected 6 items in: %q", result.Stdout)
	}
}

func TestIntegration_LS_Long(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("ls", "-la", dir)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("ls -la: %v", err)
	}

	result, err := handleLS(string(raw), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw ls -la:\n%s", raw)
	t.Logf("compressed:\n%s", result.Stdout)

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "dirs(") {
		t.Errorf("expected dirs() in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "total:") {
		t.Errorf("expected total: in:\n%s", result.Stdout)
	}
	// Permissions should not appear.
	if strings.Contains(result.Stdout, "rwx") {
		t.Errorf("permissions should be stripped:\n%s", result.Stdout)
	}
}

// newTestDir creates a temp dir with a predictable set of files and dirs.
//
//	files: foo.go, bar.go, go.mod, README.md, Makefile  (5 items)
//	dirs: subdir/
func newTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir := func(name string) {
		t.Helper()
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write("foo.go")
	write("bar.go")
	write("go.mod")
	write("README.md")
	write("Makefile")
	mkdir("subdir")
	return dir
}

func assertEq[T comparable](t *testing.T, label string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", label, want, got)
	}
}
