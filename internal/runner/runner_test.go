package runner_test

import (
	"os/exec"
	"strings"
	"testing"

	"tko/internal/runner"
)

func TestRun_CapturesStdout(t *testing.T) {
	result, err := runner.Run("git", []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Stdout, "git version") {
		t.Errorf("unexpected stdout: %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
}

func TestRun_ForwardsExitCode(t *testing.T) {
	// 'false' always exits 1
	result, err := runner.Run("false", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit 1, got %d", result.ExitCode)
	}
}

func TestRun_OutputMatchesDirect(t *testing.T) {
	direct, err := exec.Command("git", "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run("git", []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != string(direct) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", result.Stdout, string(direct))
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	_, err := runner.Run("this-command-does-not-exist-tp-test", []string{})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}
