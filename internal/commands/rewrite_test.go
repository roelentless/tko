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

func TestRewrite_GitDiffCached(t *testing.T) {
	got, ok := commands.Rewrite("git diff --cached")
	if !ok || got != "tko -- git diff --cached" {
		t.Errorf("got (%q, %v), want (\"tko -- git diff --cached\", true)", got, ok)
	}
}

func TestRewrite_GitLogNotHandled(t *testing.T) {
	// git log has no handler — should not be rewritten
	_, ok := commands.Rewrite("git log --oneline -10")
	if ok {
		t.Error("git log should not be rewritten: no handler supports it")
	}
}

func TestRewrite_GitDiffWordDiffNotHandled(t *testing.T) {
	// --word-diff changes output format — handler rejects it, should not rewrite
	_, ok := commands.Rewrite("git diff --word-diff")
	if ok {
		t.Error("git diff --word-diff should not be rewritten: unsupported flag")
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
