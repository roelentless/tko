package du

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: parseDUFlags -------------------------------------------------

func TestParseDUFlags_SH(t *testing.T) {
	if !parseDUFlags([]string{"-sh"}) {
		t.Error("-sh should be accepted")
	}
}

func TestParseDUFlags_HS(t *testing.T) {
	if !parseDUFlags([]string{"-hs"}) {
		t.Error("-hs should be accepted")
	}
}

func TestParseDUFlags_SeparateFlags(t *testing.T) {
	if !parseDUFlags([]string{"-s", "-h"}) {
		t.Error("-s -h should be accepted")
	}
}

func TestParseDUFlags_WithPaths(t *testing.T) {
	if !parseDUFlags([]string{"-sh", "/some/path", "/other/path"}) {
		t.Error("-sh with paths should be accepted")
	}
}

func TestParseDUFlags_SOnly_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-s"}) {
		t.Error("-s alone should be rejected (no -h)")
	}
}

func TestParseDUFlags_HOnly_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-h"}) {
		t.Error("-h alone should be rejected (no -s)")
	}
}

func TestParseDUFlags_NoFlags_Rejected(t *testing.T) {
	if parseDUFlags([]string{"/some/path"}) {
		t.Error("no flags should be rejected")
	}
}

func TestParseDUFlags_UnknownFlag_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-sha"}) {
		t.Error("-sha (unknown 'a') should be rejected")
	}
}

func TestParseDUFlags_Depth_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-sh", "-d", "2"}) {
		t.Error("-d flag should be rejected")
	}
}

func TestParseDUFlags_LongFlag_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-sh", "--max-depth=1"}) {
		t.Error("--max-depth should be rejected")
	}
}

func TestParseDUFlags_Total_Rejected(t *testing.T) {
	if parseDUFlags([]string{"-sh", "--total"}) {
		t.Error("--total should be rejected")
	}
}

// --- unit tests: Supports() routing ------------------------------------------

func TestSupports(t *testing.T) {
	h := &duHandler{}
	cases := []struct {
		args []string
		want bool
		name string
	}{
		{[]string{"-sh"}, true, "du -sh"},
		{[]string{"-hs"}, true, "du -hs"},
		{[]string{"-s", "-h"}, true, "du -s -h"},
		{[]string{"-sh", "/path"}, true, "du -sh /path"},
		{[]string{"-sh", "/a", "/b"}, true, "du -sh /a /b"},
		{[]string{"-s"}, false, "du -s rejected"},
		{[]string{"-h"}, false, "du -h rejected"},
		{nil, false, "du (no args) rejected"},
		{[]string{"-sha"}, false, "du -sha rejected"},
		{[]string{"--summarize"}, false, "du --summarize rejected"},
	}
	for _, c := range cases {
		got := h.Supports(c.args)
		if got != c.want {
			t.Errorf("%s: Supports(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

// --- unit tests: handleDU ----------------------------------------------------

func TestHandleDU_Empty(t *testing.T) {
	result, err := handleDU("")
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "empty") {
		t.Errorf("expected 'empty' in: %q", result.Stdout)
	}
}

func TestHandleDU_CommonPrefix(t *testing.T) {
	raw := "1.3G\t/models/whisper/AudioEncoder.mlmodelc\n" +
		"4.0K\t/models/whisper/config.json\n" +
		"1.7G\t/models/whisper/TextDecoder.mlmodelc\n"

	result, err := handleDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)

	// Common prefix should appear once in the header.
	if strings.Count(result.Stdout, "/models/whisper/") != 1 {
		t.Errorf("expected prefix once in output:\n%s", result.Stdout)
	}
	// All three filenames must appear.
	for _, name := range []string{"AudioEncoder.mlmodelc", "config.json", "TextDecoder.mlmodelc"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
	// All sizes must appear.
	for _, size := range []string{"1.3G", "4.0K", "1.7G"} {
		if !strings.Contains(result.Stdout, size) {
			t.Errorf("expected size %q in:\n%s", size, result.Stdout)
		}
	}
	// Item count in header.
	if !strings.Contains(result.Stdout, "3 items") {
		t.Errorf("expected '3 items' in:\n%s", result.Stdout)
	}
}

func TestHandleDU_NoCommonPrefix(t *testing.T) {
	raw := "100G\t/home\n" +
		"1.2G\t/tmp\n" +
		"500M\t/var\n"

	result, err := handleDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)

	// All paths and sizes must appear.
	for _, want := range []string{"/home", "/tmp", "/var", "100G", "1.2G", "500M"} {
		if !strings.Contains(result.Stdout, want) {
			t.Errorf("expected %q in:\n%s", want, result.Stdout)
		}
	}
	if !strings.Contains(result.Stdout, "3 items") {
		t.Errorf("expected '3 items' in:\n%s", result.Stdout)
	}
}

func TestHandleDU_SingleEntry(t *testing.T) {
	raw := "3.6G\t/some/long/path/dir/\n"

	result, err := handleDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)

	if !strings.Contains(result.Stdout, "3.6G") {
		t.Errorf("expected size in:\n%s", result.Stdout)
	}
}

func TestHandleDU_TrailingSlashPaths(t *testing.T) {
	// du on directories often includes trailing slash.
	raw := "256M\t/cache/24G517/AAA/\n" +
		"4.0K\t/cache/24G517/BBB/\n" +
		"3.6G\t/cache/24G517/CCC/\n"

	result, err := handleDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "lossless", true, result.Lossless)

	// Common prefix /cache/24G517/ should appear once.
	if strings.Count(result.Stdout, "/cache/24G517/") != 1 {
		t.Errorf("expected prefix exactly once:\n%s", result.Stdout)
	}
	for _, name := range []string{"AAA/", "BBB/", "CCC/"} {
		if !strings.Contains(result.Stdout, name) {
			t.Errorf("expected %q in:\n%s", name, result.Stdout)
		}
	}
}

func TestHandleDU_TabSeparator_BadLine(t *testing.T) {
	raw := "no-tab-here\n"
	_, err := handleDU(raw)
	if err == nil {
		t.Error("expected error for line without tab")
	}
}

// --- integration tests: real du binary ----------------------------------------

func TestIntegration_DU_SH(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("du", "-sh", filepath.Join(dir, "subdir1"), filepath.Join(dir, "subdir2"))
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("du -sh: %v", err)
	}

	t.Logf("raw du -sh:\n%s", raw)

	result, err := handleDU(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("compressed:\n%s", result.Stdout)

	assertEq(t, "lossless", true, result.Lossless)

	// Both subdirs should appear in output.
	if !strings.Contains(result.Stdout, "subdir1") {
		t.Errorf("expected subdir1 in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "subdir2") {
		t.Errorf("expected subdir2 in:\n%s", result.Stdout)
	}
	// Common prefix (the temp dir) should appear once.
	if strings.Count(result.Stdout, dir) != 1 {
		t.Errorf("expected temp dir path exactly once in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "2 items") {
		t.Errorf("expected '2 items' in:\n%s", result.Stdout)
	}
}

func TestIntegration_DU_SingleDir(t *testing.T) {
	dir := newTestDir(t)

	cmd := exec.Command("du", "-sh", dir)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("du -sh: %v", err)
	}

	t.Logf("raw du -sh:\n%s", raw)

	result, err := handleDU(string(raw))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("compressed:\n%s", result.Stdout)

	assertEq(t, "lossless", true, result.Lossless)
	if !strings.Contains(result.Stdout, "1 item") {
		t.Errorf("expected '1 item' in:\n%s", result.Stdout)
	}
}

// newTestDir creates a temp dir with two subdirectories containing files.
func newTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, sub := range []string{"subdir1", "subdir2"} {
		subPath := filepath.Join(dir, sub)
		if err := os.Mkdir(subPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subPath, "file.txt"), []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func assertEq[T comparable](t *testing.T, label string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", label, want, got)
	}
}
