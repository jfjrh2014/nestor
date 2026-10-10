package pathutil

import (
	"path/filepath"
	"strings"
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

// TestResolveHome: tilde dests expand when home is known; when the lookup
// fails (empty home) any tilde form is refused rather than passed through —
// an unexpanded "~/.bashrc" lands under a literal "~" directory relative to
// the working directory. "~user/..." is refused in both cases.
func TestResolveHome(t *testing.T) {
	homeErr := func(p, home string) (string, error) { return ResolveHome(p, home) }

	if got, err := homeErr("~/.bashrc", "/home/m"); err != nil || got != "/home/m/.bashrc" {
		t.Errorf("ResolveHome(~/.bashrc, home) = %q, %v", got, err)
	}
	if got, err := homeErr("~", "/home/m"); err != nil || got != "/home/m" {
		t.Errorf("ResolveHome(~, home) = %q, %v", got, err)
	}
	if got, err := homeErr("/abs/path", "/home/m"); err != nil || got != "/abs/path" {
		t.Errorf("ResolveHome(absolute) = %q, %v", got, err)
	}
	if got, err := homeErr("relative/path", ""); err != nil || got != "relative/path" {
		t.Errorf("ResolveHome(relative, no home) = %q, %v", got, err)
	}
	if got, err := homeErr("", ""); got != "" || err != nil {
		t.Errorf("ResolveHome(empty) = %q, %v", got, err)
	}

	for _, home := range []string{"/home/m", ""} {
		if _, err := homeErr("~other/.bashrc", home); err == nil {
			t.Errorf("ResolveHome(~other/.bashrc, home=%q) = nil error, want refusal", home)
		}
	}
	_, err := ResolveHome("~/.bashrc", "")
	if err == nil {
		t.Fatal("ResolveHome(tilde, no home) = nil error, want refusal")
	}
	if !strings.Contains(err.Error(), "home directory is unavailable") {
		t.Errorf("error text = %q, want home-unavailable reason", err)
	}
}
