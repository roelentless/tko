package wc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: Supports() --------------------------------------------------

func TestSupports(t *testing.T) {
	h := &wcHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"-l", "foo.txt", "bar.txt"}, true, "-l with files"},
		{[]string{"-l", "src/a.go"}, true, "-l single file"},
		// rejected: no paths
		{[]string{"-l"}, false, "-l no paths"},
		// rejected: other counting flags
		{[]string{"-lc", "foo.txt"}, false, "-lc rejected"},
		{[]string{"-lw", "foo.txt"}, false, "-lw rejected"},
		{[]string{"-c", "foo.txt"}, false, "-c only"},
		{[]string{"-w", "foo.txt"}, false, "-w only"},
		{[]string{"--lines", "foo.txt"}, false, "--lines rejected"},
		// rejected: empty / no -l
		{nil, false, "nil args"},
		{[]string{"foo.txt"}, false, "no flags"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: handleWC ----------------------------------------------------

func TestHandleWC_Empty(t *testing.T) {
	result, err := handleWC("")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
}

func TestHandleWC_SingleFile(t *testing.T) {
	// Single file: no total line — pass through unchanged.
	raw := "  312 foo/bar.md\n"
	result, err := handleWC(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	if result.Stdout != raw {
		t.Errorf("single file should pass through unchanged; got %q", result.Stdout)
	}
}

func TestHandleWC_CommonPrefix(t *testing.T) {
	raw := "  312 rfc/RFC-001.md\n" +
		"  361 rfc/RFC-002.md\n" +
		"  673 total\n"

	result, err := handleWC(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}

	// Prefix should appear once in header.
	if strings.Count(result.Stdout, "rfc/") != 1 {
		t.Errorf("expected prefix exactly once in:\n%s", result.Stdout)
	}
	// Total in header.
	if !strings.Contains(result.Stdout, "673 total") {
		t.Errorf("expected total in:\n%s", result.Stdout)
	}
	// File count in header.
	if !strings.Contains(result.Stdout, "2 files") {
		t.Errorf("expected '2 files' in:\n%s", result.Stdout)
	}
	// All filenames must appear.
	for _, name := range []string{"RFC-001.md", "RFC-002.md"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	// All counts must appear.
	for _, count := range []string{"312", "361"} {
		if !strings.Contains(result.Stdout, count) {
			t.Errorf("expected count %q in:\n%s", count, result.Stdout)
		}
	}
}

func TestHandleWC_NoCommonPrefix(t *testing.T) {
	raw := "  10 foo/a.go\n" +
		"  20 bar/b.go\n" +
		"  30 total\n"

	result, err := handleWC(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lossless {
		t.Error("expected lossless")
	}
	// Full paths must appear when no common prefix.
	for _, path := range []string{"foo/a.go", "bar/b.go"} {
		if !strings.Contains(result.Stdout, path) {
			t.Errorf("expected %q in:\n%s", path, result.Stdout)
		}
	}
	if !strings.Contains(result.Stdout, "30 total") {
		t.Errorf("expected total in:\n%s", result.Stdout)
	}
}

func TestHandleWC_BadLine(t *testing.T) {
	_, err := handleWC("no-space-here\n")
	if err == nil {
		t.Error("expected error for malformed line")
	}
}

// --- integration tests: real wc binary ---------------------------------------

func TestIntegration_WC_L(t *testing.T) {
	dir := t.TempDir()

	writeLines := func(name string, n int) {
		t.Helper()
		lines := strings.Repeat("line\n", n)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeLines("a.txt", 10)
	writeLines("b.txt", 25)
	writeLines("c.txt", 5)

	cmd := exec.Command("wc", "-l",
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
		filepath.Join(dir, "c.txt"),
	)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("wc -l: %v", err)
	}
	t.Logf("raw wc -l:\n%s", raw)

	result, err := handleWC(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("compressed:\n%s", result.Stdout)

	if !result.Lossless {
		t.Error("expected lossless")
	}
	// Dir should appear once (in header).
	if strings.Count(result.Stdout, dir) != 1 {
		t.Errorf("expected dir path exactly once in:\n%s", result.Stdout)
	}
	// All filenames must appear.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	// Counts must appear.
	for _, count := range []string{"10", "25", "5"} {
		if !strings.Contains(result.Stdout, count) {
			t.Errorf("expected count %q in:\n%s", count, result.Stdout)
		}
	}
	if !strings.Contains(result.Stdout, "3 files") {
		t.Errorf("expected '3 files' in:\n%s", result.Stdout)
	}
}
