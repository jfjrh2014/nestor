package pathutil

import (
	"os"
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

func TestSamePath(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"tilde vs absolute same file", "~/.bashrc", home + "/.bashrc", true},
		{"absolute vs tilde reversed", home + "/.bashrc", "~/.bashrc", true},
		{"identical tilde forms", "~/.bashrc", "~/.bashrc", true},
		{"different files", "~/.bashrc", "~/.zshrc", false},
		{"plain relative vs tilde", ".bashrc", "~/.bashrc", false},
		{"empty paths", "", "", true},
		{"empty vs nonempty", "", "~/.bashrc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SamePath(tt.a, tt.b, home); got != tt.want {
				t.Errorf("SamePath(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCanonicalDest(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tilde", "~/.bashrc", filepath.Join(home, ".bashrc")},
		{"dot spelling of tilde", "~/.config/./nestor/nestor.yml", filepath.Join(home, ".config", "nestor", "nestor.yml")},
		{"traversal inside tilde", "~/x/../y/.zshrc", filepath.Join(home, "y", ".zshrc")},
		{"relative dot", "./zshrc", filepath.Join(cwd, "zshrc")},
		{"plain relative", "zshrc", filepath.Join(cwd, "zshrc")},
		{"relative traversal", "a/../zshrc", filepath.Join(cwd, "zshrc")},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalDest(tt.in, home); got != tt.want {
				t.Errorf("CanonicalDest(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	// absolute passthrough with redundant segments
	abs := home + "/.config/nestor/../nestor/nestor.yml"
	if got := CanonicalDest(abs, home); got != filepath.Join(home, ".config", "nestor", "nestor.yml") {
		t.Errorf("CanonicalDest(abs with ..) = %q", got)
	}
}

func TestCanonicalDestIdentityCollapses(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	base := CanonicalDest("~/.bashrc", home)
	for _, spelling := range []string{
		"~/.bashrc",
		home + "/.bashrc",
		home + "/./.bashrc",
		home + "/../" + filepath.Base(home) + "/.bashrc",
	} {
		if got := CanonicalDest(spelling, home); got != base {
			t.Errorf("CanonicalDest(%q) = %q, want %q (same file as ~/.bashrc)", spelling, got, base)
		}
	}
}

func TestSamePathCanonicalSpellings(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	pairs := []struct {
		name string
		a, b string
		want bool
	}{
		{"dot spelling", "~/.bashrc", home + "/./.bashrc", true},
		{"traversal", "~/.bashrc", home + "/../tester/.bashrc", true},
		{"relative and tilde are different files", ".bashrc", "~/.bashrc", false},
		{"different files still differ", "~/.bashrc", "~/.zshrc", false},
	}
	for _, tt := range pairs {
		t.Run(tt.name, func(t *testing.T) {
			if got := SamePath(tt.a, tt.b, home); got != tt.want {
				t.Errorf("SamePath(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsSelfInjectCanonicalSpellings(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	target := filepath.Join(home, ".config", "nestor", "nestor.yml")
	spellings := []string{
		"~/.config/nestor/nestor.yml",
		target,
		home + "/.config/nestor/./nestor.yml",
		home + "/.config/nestor/../nestor/nestor.yml",
	}
	for _, s := range spellings {
		if !IsSelfInject(s, home) {
			t.Errorf("IsSelfInject(%q) = false, want true", s)
		}
	}
	if IsSelfInject("~/.config/other/nestor.yml", home) {
		t.Error("different file wrongly flagged as self-inject")
	}
}

func TestIsManagedDotfileDestCanonicalSpellings(t *testing.T) {
	home := string(filepath.Separator) + filepath.Join("home", "tester")
	dests := []string{"~/.bashrc"}
	forms := []string{
		"~/.bashrc",
		home + "/.bashrc",
		home + "/./.bashrc",
		home + "/../tester/.bashrc",
	}
	for _, f := range forms {
		if !IsManagedDotfileDest(f, dests, home) {
			t.Errorf("IsManagedDotfileDest(%q) = false, want true", f)
		}
	}
	if IsManagedDotfileDest("~/.zshrc", dests, home) {
		t.Error("different file wrongly flagged as managed dest")
	}
}
