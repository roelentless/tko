package commands_test

import (
	"testing"

	"tko/internal/commands"
	_ "tko/internal/commands/git" // register git handler
)

func TestRewrite_GitStatus(t *testing.T) {
	got, ok := commands.Rewrite("git status")
	if !ok || got != "tko -- git status" {
		t.Errorf("got (%q, %v), want (\"tko -- git status\", true)", got, ok)
	}
}

func TestRewrite_GitLog(t *testing.T) {
	got, ok := commands.Rewrite("git log --oneline")
	if !ok || got != "tko -- git log --oneline" {
		t.Errorf("got (%q, %v), want (\"tko -- git log --oneline\", true)", got, ok)
	}
}

func TestRewrite_GitDiffNotHandled(t *testing.T) {
	// git diff is a targeted command — should pass through raw
	_, ok := commands.Rewrite("git diff --cached")
	if ok {
		t.Error("git diff should not be rewritten: targeted command")
	}
}

func TestRewrite_GitStatusWithFlags(t *testing.T) {
	// git status --short changes output format — should not be rewritten
	_, ok := commands.Rewrite("git status --short")
	if ok {
		t.Error("git status --short should not be rewritten: unsupported flag")
	}
}

func TestRewrite_AlreadyWrapped(t *testing.T) {
	_, ok := commands.Rewrite("tko -- git status")
	if ok {
		t.Error("should not double-wrap a tko command")
	}
}

func TestRewrite_CompoundAnd(t *testing.T) {
	_, ok := commands.Rewrite("git status && git diff")
	if ok {
		t.Error("should not rewrite compound && command")
	}
}

func TestRewrite_Pipe(t *testing.T) {
	_, ok := commands.Rewrite("git log | head -20")
	if ok {
		t.Error("should not rewrite piped command")
	}
}

func TestRewrite_Semicolon(t *testing.T) {
	_, ok := commands.Rewrite("git add .; git status")
	if ok {
		t.Error("should not rewrite semicolon-separated commands")
	}
}

func TestRewrite_UnknownCommand(t *testing.T) {
	_, ok := commands.Rewrite("curl https://example.com")
	if ok {
		t.Error("should not rewrite command with no registered handler")
	}
}

func TestRewrite_Empty(t *testing.T) {
	_, ok := commands.Rewrite("")
	if ok {
		t.Error("should not rewrite empty command")
	}
}

func TestRewrite_FullPath(t *testing.T) {
	// /usr/bin/git should still match the "git" handler
	got, ok := commands.Rewrite("/usr/bin/git status")
	if !ok || got != "tko -- /usr/bin/git status" {
		t.Errorf("got (%q, %v), want (\"tko -- /usr/bin/git status\", true)", got, ok)
	}
}
