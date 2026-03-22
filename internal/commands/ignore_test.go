package commands

import "testing"

func TestIsIgnored(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		// content display
		{"cat", true},
		{"head", true},
		{"tail", true},
		// stream processing
		{"grep", true},
		{"rg", true},
		{"awk", true},
		{"sed", true},
		{"xargs", true},
		// file/dir operations
		{"rm", true},
		{"mkdir", true},
		{"chmod", true},
		{"mv", true},
		{"cp", true},
		// script runners
		{"bash", true},
		{"python", true},
		{"python3", true},
		{"node", true},
		// network
		{"curl", true},
		{"wget", true},
		// process management
		{"kill", true},
		{"pkill", true},
		// full path — basename must match
		{"/usr/bin/grep", true},
		{"/bin/rm", true},
		// not ignored
		{"git", false},
		{"du", false},
		{"find", false},
		{"fd", false},
		{"ls", false},
		{"wc", false},
		{"go", false},
		{"make", false},
	}
	for _, c := range cases {
		got := IsIgnored(c.cmd)
		if got != c.want {
			t.Errorf("IsIgnored(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}
