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
		{[]string{"/path", "-type", "f"}, true, "find /path -type f"},
		{[]string{"/path", "-maxdepth", "2", "-name", "*.md", "-type", "f"}, true, "maxdepth + name"},
		{[]string{"/path", "-type", "d"}, true, "find -type d"},
		{[]string{"/path"}, true, "path only"},
		{[]string{"/a", "-o", "-name", "*.txt"}, true, "with -o operator"},
		{[]string{"/path", "-not", "-name", "*.git"}, true, "with -not"},
		// rejected: output modifiers
		{[]string{"/path", "-exec", "echo", "{}", ";"}, false, "-exec rejected"},
		{[]string{"/path", "-execdir", "ls", "{}", ";"}, false, "-execdir rejected"},
		{[]string{"/path", "-printf", "%p\\n"}, false, "-printf rejected"},
		{[]string{"/path", "-delete"}, false, "-delete rejected"},
		{[]string{"/path", "-ls"}, false, "-ls rejected"},
		{[]string{"/path", "-print0"}, false, "-print0 rejected"},
		{[]string{"/path", "-ok", "rm", "{}", ";"}, false, "-ok rejected"},
		{[]string{"/path", "-fprint", "/tmp/out"}, false, "-fprint rejected"},
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
		{[]string{"-t", "f", "/path"}, true, "fd -t f /path"},
		{[]string{"pattern"}, true, "pattern only"},
		{[]string{"-e", "go"}, true, "-e extension"},
		{[]string{}, true, "no args (list all)"},
		// rejected: output modifiers
		{[]string{"--exec", "echo", "{}"}, false, "--exec rejected"},
		{[]string{"-x", "cat"}, false, "-x rejected"},
		{[]string{"--exec-batch", "rm"}, false, "--exec-batch rejected"},
		{[]string{"-X", "rm"}, false, "-X rejected"},
		{[]string{"--list-details"}, false, "--list-details rejected"},
		{[]string{"-l"}, false, "-l rejected"},
		{[]string{"--print0"}, false, "--print0 rejected"},
		{[]string{"-0"}, false, "-0 rejected"},
		{[]string{"--format=%p"}, false, "--format= rejected"},
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
	raw := "/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/agents/agent-sdk-verifier-ts.md\n" +
		"/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/agents/agent-sdk-verifier-py.md\n" +
		"/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/README.md\n" +
		"/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/.claude-plugin/plugin.json\n" +
		"/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/commands/new-sdk-app.md\n"

	result, err := handlePaths("find", "files", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}

	// Common prefix should appear once in the header.
	prefix := "/Users/roel/projects/clones/claude-code/plugins/agent-sdk-dev/"
	if strings.Count(result.Stdout, prefix) != 1 {
		t.Errorf("expected prefix exactly once in:\n%s", result.Stdout)
	}
	// All filenames must appear.
	for _, name := range []string{"agent-sdk-verifier-ts.md", "agent-sdk-verifier-py.md", "README.md", "plugin.json", "new-sdk-app.md"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	// Count must appear.
	if !strings.Contains(result.Stdout, "5 files") {
		t.Errorf("expected '5 files' in:\n%s", result.Stdout)
	}
}

func TestHandlePaths_NoCommonPrefix(t *testing.T) {
	raw := "/home/user/a.txt\n/tmp/b.txt\n"

	result, err := handlePaths("find", "files", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	if !strings.Contains(result.Stdout, "/home/user/a.txt") {
		t.Errorf("expected full path in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "/tmp/b.txt") {
		t.Errorf("expected full path in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "2 files") {
		t.Errorf("expected '2 files' in:\n%s", result.Stdout)
	}
}

func TestHandlePaths_SinglePath(t *testing.T) {
	raw := "/some/deep/path/file.go\n"

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
	paths := []string{"/a/b/c.txt", "/a/b/d.txt"}
	got := commonDirPrefix(paths)
	if got != "/a/b/" {
		t.Errorf("got %q, want %q", got, "/a/b/")
	}
}

func TestCommonDirPrefix_DifferentDirs(t *testing.T) {
	paths := []string{"/a/b/c.txt", "/a/x/d.txt"}
	got := commonDirPrefix(paths)
	if got != "/a/" {
		t.Errorf("got %q, want %q", got, "/a/")
	}
}

func TestCommonDirPrefix_NoCommon(t *testing.T) {
	paths := []string{"/home/user/file.txt", "/tmp/other.txt"}
	got := commonDirPrefix(paths)
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

func TestCommonDirPrefix_Single(t *testing.T) {
	got := commonDirPrefix([]string{"/a/b/c.txt"})
	if got != "/a/b/" {
		t.Errorf("got %q, want %q", got, "/a/b/")
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
	// All filenames must appear.
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
	// All filenames must appear.
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
