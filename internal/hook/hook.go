package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookCmd returns the command string to register in settings.json,
// using the path of the currently running tko binary.
func hookCmd() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve binary path: %w", err)
	}
	return exe + " hook claude", nil
}

// stateDir returns ~/.local/share/tko.
func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tko"), nil
}

// claudeSettingsPath returns ~/.claude/settings.json.
func claudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// Install registers the tko binary as a Claude Code PreToolUse hook.
func Install() error {
	cmd, err := hookCmd()
	if err != nil {
		return err
	}

	settingsPath, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	if err := patchSettings(settingsPath, cmd); err != nil {
		return fmt.Errorf("patch settings.json: %w", err)
	}

	fmt.Printf("hook:     %s\n", cmd)
	fmt.Printf("settings: %s\n", settingsPath)
	fmt.Println("\nRestart Claude Code to activate.")
	return nil
}

// Uninstall removes the tko hook from settings.json.
func Uninstall() error {
	settingsPath, err := claudeSettingsPath()
	if err != nil {
		return err
	}

	cmd, err := hookCmd()
	if err != nil {
		return err
	}

	if err := unpatchSettings(settingsPath, cmd); err != nil {
		return fmt.Errorf("unpatch settings.json: %w", err)
	}

	fmt.Println("Hook removed. Restart Claude Code to deactivate.")
	return nil
}

// Status prints whether the hook is currently installed.
func Status() {
	cmd, err := hookCmd()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	settingsPath, err := claudeSettingsPath()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	if hookPresentInSettings(settingsPath, cmd) {
		fmt.Printf("installed: %s\n", cmd)
	} else {
		fmt.Println("not installed. Run: tko hook install")
	}
}

// --- settings.json patching ---------------------------------------------------

func patchSettings(settingsPath, hookCmd string) error {
	root := readOrInitSettings(settingsPath)

	hooks := ensureMap(root, "hooks")
	preToolUse := ensureSlice(hooks, "PreToolUse")

	bashEntry, bashIdx := findBashMatcher(preToolUse)

	entryHooks := filterTkoHooks(bashEntry["hooks"])
	entryHooks = append(entryHooks, map[string]interface{}{
		"type":    "command",
		"command": hookCmd,
	})
	bashEntry["hooks"] = entryHooks
	bashEntry["matcher"] = "Bash"

	if bashIdx >= 0 {
		preToolUse[bashIdx] = bashEntry
	} else {
		preToolUse = append(preToolUse, bashEntry)
	}
	hooks["PreToolUse"] = preToolUse

	return writeSettings(settingsPath, root)
}

func unpatchSettings(settingsPath, hookCmd string) error {
	root := readOrInitSettings(settingsPath)

	hooks, ok := root["hooks"].(map[string]interface{})
	if !ok {
		return nil
	}
	preToolUse, _ := hooks["PreToolUse"].([]interface{})
	for _, entry := range preToolUse {
		m, ok := entry.(map[string]interface{})
		if !ok || m["matcher"] != "Bash" {
			continue
		}
		m["hooks"] = filterTkoHooks(m["hooks"])
	}
	return writeSettings(settingsPath, root)
}

func hookPresentInSettings(settingsPath, hookCmd string) bool {
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), hookCmd)
}

// --- JSON helpers -------------------------------------------------------------

func readOrInitSettings(path string) map[string]interface{} {
	content, err := os.ReadFile(path)
	if err == nil && len(content) > 0 {
		var root map[string]interface{}
		if json.Unmarshal(content, &root) == nil {
			return root
		}
	}
	return map[string]interface{}{}
}

func ensureMap(parent map[string]interface{}, key string) map[string]interface{} {
	m, _ := parent[key].(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
		parent[key] = m
	}
	return m
}

func ensureSlice(parent map[string]interface{}, key string) []interface{} {
	s, _ := parent[key].([]interface{})
	if s == nil {
		s = []interface{}{}
		parent[key] = s
	}
	return s
}

func findBashMatcher(entries []interface{}) (map[string]interface{}, int) {
	for i, e := range entries {
		if m, ok := e.(map[string]interface{}); ok && m["matcher"] == "Bash" {
			return m, i
		}
	}
	return map[string]interface{}{}, -1
}

// filterTkoHooks removes any existing tko hook entries (both legacy .sh and binary).
func filterTkoHooks(raw interface{}) []interface{} {
	existing, _ := raw.([]interface{})
	out := make([]interface{}, 0, len(existing)+1)
	for _, h := range existing {
		if hm, ok := h.(map[string]interface{}); ok {
			if cmd, _ := hm["command"].(string); isTkoHook(cmd) {
				continue
			}
		}
		out = append(out, h)
	}
	return out
}

func isTkoHook(cmd string) bool {
	return strings.HasSuffix(cmd, "/tko hook claude")
}

func writeSettings(path string, root map[string]interface{}) error {
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.WriteFile(path+".bak", func() []byte {
			c, _ := os.ReadFile(path)
			return c
		}(), 0o644)
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
