package pathutil

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	tests := []struct{ in, want string }{
		// passthrough
		{"", ""},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
		{".", "."},
		// bare tilde
		{"~", home},
		{"~" + string(filepath.Separator), filepath.Clean(home)},
		// own-home forms expand
		{"~/x", filepath.Join(home, "x")},
		{"~/a/b/c.txt", filepath.Join(home, "a", "b", "c.txt")},
		{"~" + string(filepath.Separator) + "x", filepath.Join(home, "x")},
		// other-user and other-name tildes are NOT expanded
		{"~root/.bashrc", "~root/.bashrc"},
		{"~shared/dir/file", "~shared/dir/file"},
		{"~x", "~x"},
		// empty home (lookup failed) leaves everything unchanged
	}
	for _, tt := range tests {
		if got := ExpandHome(tt.in, home); got != tt.want {
			t.Errorf("ExpandHome(%q, %q) = %q, want %q", tt.in, home, got, tt.want)
		}
	}

	// Empty home never re-roots a tilde path.
	for _, in := range []string{"~", "~/x", "~root/x"} {
		if got := ExpandHome(in, ""); got != in {
			t.Errorf("ExpandHome(%q, \"\") = %q, want unchanged", in, got)
		}
	}
}

func TestIsOtherUserTilde(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"~", false},
		{"~/x", false},
		{"~/", false},
		{"/abs", false},
		{"relative", false},
		{"~root/.bashrc", true},
		{"~shared/dir/file", true},
		{"~x", true},
	}
	for _, tt := range tests {
		if got := IsOtherUserTilde(tt.in); got != tt.want {
			t.Errorf("IsOtherUserTilde(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
