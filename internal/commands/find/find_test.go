package find

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: findHandler.Supports() --------------------------------------

func TestFindSupports(t *testing.T) {
	h := &findHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"src", "-type", "f"}, true, "-type f"},
		{[]string{"src", "-maxdepth", "2", "-name", "*.md", "-type", "f"}, true, "maxdepth + name"},
		{[]string{"src", "-type", "d"}, true, "-type d"},
		{[]string{"src"}, true, "path only"},
		{[]string{"src", "-o", "-name", "*.txt"}, true, "-o operator"},
		{[]string{"src", "-not", "-name", "*.git"}, true, "-not"},
		// rejected: output modifiers
		{[]string{"src", "-exec", "echo", "{}", ";"}, false, "-exec"},
		{[]string{"src", "-execdir", "ls", "{}", ";"}, false, "-execdir"},
		{[]string{"src", "-printf", "%p\\n"}, false, "-printf"},
		{[]string{"src", "-delete"}, false, "-delete"},
		{[]string{"src", "-ls"}, false, "-ls"},
		{[]string{"src", "-print0"}, false, "-print0"},
		{[]string{"src", "-ok", "rm", "{}", ";"}, false, "-ok"},
		{[]string{"src", "-fprint", "out.txt"}, false, "-fprint"},
		// rejected: empty args
		{nil, false, "nil args"},
		{[]string{}, false, "empty args"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: fdHandler.Supports() ----------------------------------------

func TestFdSupports(t *testing.T) {
	h := &fdHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"-t", "f", "src"}, true, "-t f"},
		{[]string{"pattern"}, true, "pattern only"},
		{[]string{"-e", "go"}, true, "-e extension"},
		{[]string{}, true, "no args"},
		// common real-world patterns: dot + path, --extension, --exclude
		{[]string{".", "src", "--type", "f"}, true, "dot + path + --type f"},
		{[]string{"--type", "f", "--extension", "swift", "src", "--exclude", "'.build'"}, true, "--extension + --exclude"},
		{[]string{".", "src", "--type", "f", "--extension", "swift", "--exclude", "'.build'"}, true, "dot + path + --type + --extension + --exclude"},
		// rejected: output modifiers
		{[]string{"--exec", "echo", "{}"}, false, "--exec"},
		{[]string{"-x", "cat"}, false, "-x"},
		{[]string{"--exec-batch", "rm"}, false, "--exec-batch"},
		{[]string{"-X", "rm"}, false, "-X"},
		{[]string{"--list-details"}, false, "--list-details"},
		{[]string{"-l"}, false, "-l"},
		{[]string{"--print0"}, false, "--print0"},
		{[]string{"-0"}, false, "-0"},
		{[]string{"--format=%p"}, false, "--format="},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: typeLabel ---------------------------------------------------

func TestTypeLabel_Find(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-type", "f"}, "files"},
		{[]string{"-type", "d"}, "dirs"},
		{[]string{"-type", "l"}, "items"},
		{[]string{"-name", "*.go"}, "items"},
		{[]string{}, "items"},
	}
	for _, c := range cases {
		got := typeLabel(c.args, "-type")
		if got != c.want {
			t.Errorf("typeLabel(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestFdTypeLabel(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-t", "f"}, "files"},
		{[]string{"--type", "file"}, "files"},
		{[]string{"-t", "d"}, "dirs"},
		{[]string{"--type", "directory"}, "dirs"},
		{[]string{"--type=f"}, "files"},
		{[]string{"--type=dir"}, "dirs"},
		{[]string{"-t=file"}, "files"},
		{[]string{"-e", "go"}, "items"},
		{[]string{}, "items"},
	}
	for _, c := range cases {
		got := fdTypeLabel(c.args)
		if got != c.want {
			t.Errorf("fdTypeLabel(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

// --- unit tests: handlePaths -------------------------------------------------

func TestHandlePaths_Empty(t *testing.T) {
	result, err := handlePaths("find", "files", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	if !strings.Contains(result.Stdout, "empty") {
		t.Errorf("expected 'empty' in: %q", result.Stdout)
	}
}

func TestHandlePaths_CommonPrefix(t *testing.T) {
	raw := "pkg/agents/verifier-ts.md\n" +
		"pkg/agents/verifier-py.md\n" +
		"pkg/README.md\n" +
		"pkg/.config/plugin.json\n" +
		"pkg/commands/init.md\n"

	result, err := handlePaths("find", "files", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}

	// Common prefix should appear once in the header.
	if strings.Count(result.Stdout, "pkg/") != 1 {
		t.Errorf("expected prefix exactly once in:\n%s", result.Stdout)
	}
	// All filenames must appear.
	for _, name := range []string{"verifier-ts.md", "verifier-py.md", "README.md", "plugin.json", "init.md"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	if !strings.Contains(result.Stdout, "5 files") {
		t.Errorf("expected '5 files' in:\n%s", result.Stdout)
	}
}

func TestHandlePaths_NoCommonPrefix(t *testing.T) {
	raw := "foo/a.txt\nbar/b.txt\n"

	result, err := handlePaths("find", "files", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	if !strings.Contains(result.Stdout, "foo/a.txt") {
		t.Errorf("expected full path in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "bar/b.txt") {
		t.Errorf("expected full path in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "2 files") {
		t.Errorf("expected '2 files' in:\n%s", result.Stdout)
	}
}

func TestHandlePaths_SinglePath(t *testing.T) {
	raw := "pkg/sub/file.go\n"

	result, err := handlePaths("find", "files", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	if !strings.Contains(result.Stdout, "file.go") {
		t.Errorf("expected filename in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "1 files") {
		t.Errorf("expected '1 files' in:\n%s", result.Stdout)
	}
}

func TestHandlePaths_RelativePaths(t *testing.T) {
	raw := "./src/main.go\n./src/util.go\n./go.mod\n"

	result, err := handlePaths("find", "items", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	// The "./" prefix should appear once in header.
	if strings.Count(result.Stdout, "./") > 1 {
		t.Errorf("expected './' at most once in:\n%s", result.Stdout)
	}
	for _, name := range []string{"main.go", "util.go", "go.mod"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
}

// --- unit tests: commonDirPrefix ---------------------------------------------

func TestCommonDirPrefix_SharedParent(t *testing.T) {
	paths := []string{"foo/bar/c.txt", "foo/bar/d.txt"}
	got := commonDirPrefix(paths)
	if got != "foo/bar/" {
		t.Errorf("got %q, want %q", got, "foo/bar/")
	}
}

func TestCommonDirPrefix_DifferentDirs(t *testing.T) {
	paths := []string{"foo/bar/c.txt", "foo/baz/d.txt"}
	got := commonDirPrefix(paths)
	if got != "foo/" {
		t.Errorf("got %q, want %q", got, "foo/")
	}
}

func TestCommonDirPrefix_NoCommon(t *testing.T) {
	paths := []string{"foo/a.txt", "bar/b.txt"}
	got := commonDirPrefix(paths)
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

func TestCommonDirPrefix_Single(t *testing.T) {
	got := commonDirPrefix([]string{"foo/bar/c.txt"})
	if got != "foo/bar/" {
		t.Errorf("got %q, want %q", got, "foo/bar/")
	}
}

func TestCommonDirPrefix_Empty(t *testing.T) {
	got := commonDirPrefix(nil)
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

// --- integration tests: real find binary -------------------------------------

func TestIntegration_Find_TypeF(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("find", dir, "-type", "f")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	t.Logf("raw find:\n%s", raw)

	result, err := handlePaths("find", "files", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compressed:\n%s", result.Stdout)

	if !result.Lossless {
		t.Error("expected lossless")
	}
	// Dir path should appear once in header.
	if strings.Count(result.Stdout, dir) != 1 {
		t.Errorf("expected dir path exactly once in:\n%s", result.Stdout)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.go"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	if !strings.Contains(result.Stdout, "3 files") {
		t.Errorf("expected '3 files' in:\n%s", result.Stdout)
	}
}

func TestIntegration_Find_TypeD(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("find", dir, "-type", "d")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	t.Logf("raw find -type d:\n%s", raw)

	result, err := handlePaths("find", "dirs", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compressed:\n%s", result.Stdout)

	if !result.Lossless {
		t.Error("expected lossless")
	}
	if !strings.Contains(result.Stdout, "dirs") {
		t.Errorf("expected 'dirs' label in:\n%s", result.Stdout)
	}
}

func TestIntegration_Find_MaxdepthName(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("find", dir, "-maxdepth", "1", "-name", "*.txt", "-type", "f")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	t.Logf("raw find -maxdepth 1 -name *.txt:\n%s", raw)

	result, err := handlePaths("find", "files", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compressed:\n%s", result.Stdout)

	if !result.Lossless {
		t.Error("expected lossless")
	}
	// Only a.txt should match (b.txt is in subdir, c.go has wrong extension).
	if !strings.Contains(result.Stdout, "a.txt") {
		t.Errorf("expected 'a.txt' in:\n%s", result.Stdout)
	}
}

// --- integration tests: real fd binary ---------------------------------------

func TestIntegration_FD_TypeF(t *testing.T) {
	if _, err := exec.LookPath("fd"); err != nil {
		t.Skip("fd not installed")
	}

	dir := newTestDir(t)

	cmd := exec.Command("fd", "-t", "f", ".", dir)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("fd: %v", err)
	}
	t.Logf("raw fd:\n%s", raw)

	result, err := handlePaths("fd", "files", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compressed:\n%s", result.Stdout)

	if !result.Lossless {
		t.Error("expected lossless")
	}
	// Dir path should appear once.
	if strings.Count(result.Stdout, dir) != 1 {
		t.Errorf("expected dir path exactly once in:\n%s", result.Stdout)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.go"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
}

// newTestDir creates a temp dir with files in different subdirs.
func newTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "c.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
