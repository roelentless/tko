package gotest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- unit tests: supportsGoTest -----------------------------------------------

func TestSupportsGoTest_Plain(t *testing.T) {
	assertEq(t, "go test ./...", true, supportsGoTest([]string{"test", "./..."}))
}

func TestSupportsGoTest_Package(t *testing.T) {
	assertEq(t, "go test ./pkg", true, supportsGoTest([]string{"test", "./pkg"}))
}

func TestSupportsGoTest_Verbose(t *testing.T) {
	assertEq(t, "go test -v ./...", true, supportsGoTest([]string{"test", "-v", "./..."}))
}

func TestSupportsGoTest_Run(t *testing.T) {
	assertEq(t, "go test -run Pat", true, supportsGoTest([]string{"test", "-run", "TestFoo", "./..."}))
	assertEq(t, "go test -run=Pat", true, supportsGoTest([]string{"test", "-run=TestFoo", "./..."}))
}

func TestSupportsGoTest_Count(t *testing.T) {
	assertEq(t, "go test -count=1", true, supportsGoTest([]string{"test", "-count=1", "./..."}))
	assertEq(t, "go test -count 1", true, supportsGoTest([]string{"test", "-count", "1", "./..."}))
}

func TestSupportsGoTest_Bench_Rejected(t *testing.T) {
	assertEq(t, "go test -bench rejected", false, supportsGoTest([]string{"test", "-bench", "."}))
}

func TestSupportsGoTest_CoverProfile_Rejected(t *testing.T) {
	assertEq(t, "go test -coverprofile rejected", false, supportsGoTest([]string{"test", "-coverprofile", "c.out"}))
}

func TestSupportsGoTest_WrongSubcmd(t *testing.T) {
	assertEq(t, "go build rejected", false, supportsGoTest([]string{"build"}))
	assertEq(t, "empty rejected", false, supportsGoTest(nil))
}

// --- unit tests: handleGoTest -------------------------------------------------

func TestHandleGoTest_AllPassing(t *testing.T) {
	raw := "ok  \ttko/internal/commands/git\t0.123s\n" +
		"ok  \ttko/internal/commands/ls\t0.456s\n"
	result, err := handleGoTest(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Stdout, "go test: 2 passed  0 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
	assertEq(t, "lossless", false, result.Lossless)
	// No failure output.
	if strings.Contains(result.Stdout, "FAIL") {
		t.Errorf("no FAIL lines expected in passing output:\n%s", result.Stdout)
	}
}

func TestHandleGoTest_OneFail(t *testing.T) {
	raw := "--- FAIL: TestFoo (0.00s)\n" +
		"    foo_test.go:12: expected true, got false\n" +
		"FAIL\n" +
		"FAIL\ttko/internal/commands/git\t0.234s\n" +
		"ok  \ttko/internal/commands/ls\t0.123s\n"
	result, err := handleGoTest(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Stdout, "go test: 1 passed  1 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "--- FAIL: TestFoo") {
		t.Errorf("expected failure detail in:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "expected true, got false") {
		t.Errorf("expected error message in:\n%s", result.Stdout)
	}
}

func TestHandleGoTest_VerbosePassingStripped(t *testing.T) {
	raw := "=== RUN   TestFoo\n" +
		"--- PASS: TestFoo (0.00s)\n" +
		"=== RUN   TestBar\n" +
		"--- PASS: TestBar (0.00s)\n" +
		"ok  \ttko/pkg\t0.123s\n"
	result, err := handleGoTest(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Stdout, "go test: 1 passed  0 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
	// Verbose lines stripped.
	if strings.Contains(result.Stdout, "=== RUN") {
		t.Errorf("=== RUN lines should be stripped:\n%s", result.Stdout)
	}
	if strings.Contains(result.Stdout, "--- PASS:") {
		t.Errorf("--- PASS: lines should be stripped:\n%s", result.Stdout)
	}
}

func TestHandleGoTest_BuildFail(t *testing.T) {
	// Build failures appear as FAIL pkg [build failed] with no timing.
	raw := "FAIL\ttko/internal/commands/git [build failed]\n"
	result, err := handleGoTest(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Stdout, "go test: 0 passed  1 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
}

// --- integration tests: real go binary ----------------------------------------

func TestIntegration_GoTest_AllPassing(t *testing.T) {
	dir := newGoTestModule(t, passingTestSrc)

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("go test: %v\noutput: %s", err, raw)
	}

	result, err := handleGoTest(string(raw), "")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw go test output:\n%s", raw)
	t.Logf("compressed:\n%s", result.Stdout)

	if !strings.HasPrefix(result.Stdout, "go test: 1 passed  0 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
	assertEq(t, "lossless", false, result.Lossless)
}

func TestIntegration_GoTest_OneFail(t *testing.T) {
	dir := newGoTestModule(t, failingTestSrc)

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	// go test exits non-zero on failure; ignore the error.
	raw, _ := cmd.Output()

	result, err := handleGoTest(string(raw), "")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("raw go test output:\n%s", raw)
	t.Logf("compressed:\n%s", result.Stdout)

	if !strings.HasPrefix(result.Stdout, "go test: 0 passed  1 failed") {
		t.Errorf("unexpected summary: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "FAIL") {
		t.Errorf("expected FAIL in output:\n%s", result.Stdout)
	}
}

// newGoTestModule creates a temporary Go module with the given test source.
func newGoTestModule(t *testing.T, testSrc string) string {
	t.Helper()
	dir := t.TempDir()

	goMod := "module testpkg\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const passingTestSrc = `package testpkg

import "testing"

func TestAlwaysPass(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("math is broken")
	}
}
`

const failingTestSrc = `package testpkg

import "testing"

func TestAlwaysFail(t *testing.T) {
	t.Errorf("expected pass, got fail: %d", 42)
}
`

func assertEq[T comparable](t *testing.T, label string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", label, want, got)
	}
}
