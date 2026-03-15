package pager

import (
	"fmt"
	"os"
	"time"
)

// Save writes content to a temp file at /tmp/tp-<ms>-<cmd>.txt.
// Returns the file path. Used only for lossy compressions.
func Save(cmd string, content string) (string, error) {
	path := fmt.Sprintf("/tmp/tko-%d-%s.txt", time.Now().UnixMilli(), sanitize(cmd))
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// Hint returns the hint line appended to lossy output.
func Hint(path string) string {
	return fmt.Sprintf("# [RAW] Full output: %s", path)
}

func sanitize(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			b = append(b, c)
		default:
			b = append(b, '-')
		}
	}
	return string(b)
}
