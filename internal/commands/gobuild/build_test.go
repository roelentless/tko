package gobuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var handler = &goBuildHandler{}

// --- Supports() tests ---------------------------------------------------------

func TestSupports(t *testing.T) {
	cases := []struct {
		args []string
		want bool
		name string
	}{
		// Accepted patterns.
		{[]string{"build", "./..."}, true, "build ./..."},
		{[]string{"build", "."}, true, "build ."},
		{[]string{"build"}, true, "build (no args)"},
		{[]string{"build", "./pkg/foo"}, true, "build specific package"},
		{[]string{"build", "-race", "./..."}, true, "build -race"},
		{[]string{"build", "-a", "./..."}, true, "build -a"},
		{[]string{"build", "-trimpath", "./..."}, true, "build -trimpath"},
		{[]string{"build", "-tags", "integration", "./..."}, true, "build -tags"},
		{[]string{"build", "-o", "/dev/null", "./..."}, true, "build -o"},
		{[]string{"build", "-ldflags", "-s -w", "./..."}, true, "build -ldflags"},

		// Rejected: wrong subcommand.
		{[]string{"test", "./..."}, false, "test subcommand"},
		{[]string{"run", "main.go"}, false, "run subcommand"},
		{[]string{}, false, "no args"},

		// Rejected: output-modifying flags.
		{[]string{"build", "-v", "./..."}, false, "build -v (verbose)"},
		{[]string{"build", "-n", "./..."}, false, "build -n (dry run)"},
		{[]string{"build", "-x", "./..."}, false, "build -x (trace)"},
		{[]string{"build", "-work", "./..."}, false, "build -work"},

		// Rejected: unknown flags.
		{[]string{"build", "--unknown", "./..."}, false, "unknown flag"},
		{[]string{"build", "-json", "./..."}, false, "unknown -json flag"},

		// Rejected: -tags without value.
		{[]string{"build", "-tags"}, false, "build -tags missing value"},
	}
	for _, c := range cases {
		got := handler.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- compressBuildErrors() unit tests -----------------------------------------

func TestCompressBuildErrors_Empty(t *testing.T) {
	got := compressBuildErrors("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCompressBuildErrors_NoHeaders(t *testing.T) {
	raw := "some random output\nwithout package headers\n"
	got := compressBuildErrors(raw)
	if got != raw {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestCompressBuildErrors_SinglePackage(t *testing.T) {
	// Single package — common prefix logic returns "" (nothing to strip).
	raw := "# github.com/example/app/pkg/foo\npkg/foo/bar.go:10:5: undefined: Baz\n"
	got := compressBuildErrors(raw)
	if got != raw {
		t.Errorf("single package: expected passthrough, got:\n%s", got)
	}
}

func TestCompressBuildErrors_MultiPackage(t *testing.T) {
	raw := "# github.com/example/app/pkg/foo\npkg/foo/a.go:1:1: undefined: X\n# github.com/example/app/pkg/bar\npkg/bar/b.go:2:2: undefined: Y\n"
	got := compressBuildErrors(raw)

	if strings.Contains(got, "github.com/example/app") {
		t.Errorf("module prefix not stripped:\n%s", got)
	}
	// Common prefix is "github.com/example/app/pkg/" — stripped to leaf names.
	if !strings.Contains(got, "# ./foo") {
		t.Errorf("expected '# ./foo' in:\n%s", got)
	}
	if !strings.Contains(got, "# ./bar") {
		t.Errorf("expected '# ./bar' in:\n%s", got)
	}
	// Error lines must be preserved exactly.
	if !strings.Contains(got, "pkg/foo/a.go:1:1: undefined: X") {
		t.Errorf("error line missing:\n%s", got)
	}
	if !strings.Contains(got, "pkg/bar/b.go:2:2: undefined: Y") {
		t.Errorf("error line missing:\n%s", got)
	}
}

func TestCompressBuildErrors_NoCommonPrefix(t *testing.T) {
	// Two packages from different modules — no common prefix to strip.
	raw := "# github.com/foo/a\na.go:1:1: err\n# gitlab.com/bar/b\nb.go:2:2: err\n"
	got := compressBuildErrors(raw)
	if got != raw {
		t.Errorf("no common prefix: expected passthrough, got:\n%s", got)
	}
}

// --- commonPathPrefix() unit tests --------------------------------------------

func TestCommonPathPrefix(t *testing.T) {
	cases := []struct {
		paths []string
		want  string
		name  string
	}{
		{[]string{"github.com/a/b/pkg/x", "github.com/a/b/pkg/y"}, "github.com/a/b/pkg/", "shared pkg prefix"},
		{[]string{"github.com/a/b/x", "github.com/a/b/y"}, "github.com/a/b/", "shared module prefix"},
		{[]string{"a/b/c", "a/b/d"}, "a/b/", "simple prefix"},
		{[]string{"a/b/c"}, "", "single path"},
		{[]string{"a/b", "c/d"}, "", "no common prefix"},
		{[]string{"a/b/c", "a/b/c"}, "", "identical paths (full match — nothing to strip)"},
		{[]string{}, "", "empty"},
	}
	for _, c := range cases {
		got := commonPathPrefix(c.paths)
		if got != c.want {
			t.Errorf("%s: commonPathPrefix(%v) = %q, want %q", c.name, c.paths, got, c.want)
		}
	}
}

// --- integration test ---------------------------------------------------------

func TestIntegration_GoBuild_Errors(t *testing.T) {
	dir := t.TempDir()

	// Write go.mod.
	modFile := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modFile, []byte("module example.com/testapp\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create two packages with errors.
	pkgA := filepath.Join(dir, "pkg", "alpha")
	pkgB := filepath.Join(dir, "pkg", "beta")
	if err := os.MkdirAll(pkgA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgA, "a.go"), []byte("package alpha\n\nfunc F() { undeclared() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgB, "b.go"), []byte("package beta\n\nfunc G() string { return 42 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	rawStderr := string(out)

	if !strings.Contains(rawStderr, "example.com/testapp") {
		t.Fatalf("expected module path in build output, got:\n%s", rawStderr)
	}

	compressed := compressBuildErrors(rawStderr)

	// Module prefix must be gone.
	if strings.Contains(compressed, "example.com/testapp") {
		t.Errorf("module prefix still present:\n%s", compressed)
	}
	// Common prefix is "example.com/testapp/pkg/" — stripped to leaf names.
	if !strings.Contains(compressed, "# ./alpha") && !strings.Contains(compressed, "# ./beta") {
		t.Errorf("expected compressed package headers, got:\n%s", compressed)
	}
	// Error lines must survive.
	if !strings.Contains(compressed, ".go:") {
		t.Errorf("error lines missing:\n%s", compressed)
	}
}

func TestIntegration_GoBuild_Success(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ok\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	result, herr := handler.Handle([]string{"build", "./..."}, "", "")
	if herr != nil {
		t.Fatal(herr)
	}
	if !result.Lossless {
		t.Error("expected Lossless=true")
	}
	if result.Stdout != "" || result.Stderr != "" {
		t.Errorf("expected empty output on success, got stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
}
