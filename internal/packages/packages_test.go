package packages

import (
	"testing"
)

func TestParseSpec_PlainName(t *testing.T) {
	s := ParseSpec("ripgrep", "apt")
	if s.Manager != "apt" || s.Sub != "" || s.Name != "ripgrep" {
		t.Errorf("got %+v, want Manager=apt Sub= Name=ripgrep", s)
	}
}

func TestParseSpec_ManagerPrefix(t *testing.T) {
	s := ParseSpec("snap: code", "apt")
	if s.Manager != "snap" || s.Sub != "" || s.Name != "code" {
		t.Errorf("got %+v, want Manager=snap Sub= Name=code", s)
	}
}

func TestParseSpec_ManagerAndSub(t *testing.T) {
	s := ParseSpec("homebrew/cask: visual-studio-code", "apt")
	if s.Manager != "homebrew" || s.Sub != "cask" || s.Name != "visual-studio-code" {
		t.Errorf("got %+v, want Manager=homebrew Sub=cask Name=visual-studio-code", s)
	}
}

func TestParseSpec_AltBrewPrefix(t *testing.T) {
	// brew/cask-verse til — same syntax, different label
	s := ParseSpec("brew/cask: firefox", "apt")
	if s.Manager != "brew" || s.Sub != "cask" || s.Name != "firefox" {
		t.Errorf("got %+v", s)
	}
}

func TestParseSpec_TrimsWhitespace(t *testing.T) {
	s := ParseSpec("  ripgrep  ", "apt")
	if s.Name != "ripgrep" {
		t.Errorf("name not trimmed: %q", s.Name)
	}
}

func TestResolve_MergesCommonAndPlatform(t *testing.T) {
	r := Resolver{
		Common: []string{"git", "tmux"},
		Lists: map[string][]string{
			"linux": {"ripgrep", "fd"},
		},
	}
	got := r.Resolve("linux")
	want := []string{"git", "tmux", "ripgrep", "fd"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestResolve_Deduplicates(t *testing.T) {
	r := Resolver{
		Common: []string{"git", "git", "tmux"},
		Lists:  map[string][]string{"linux": {"git", "fd"}},
	}
	got := r.Resolve("linux")
	if len(got) != 3 {
		t.Errorf("got %d items, want 3 (deduped): %v", len(got), got)
	}
}

func TestResolve_UnknownPlatform(t *testing.T) {
	r := Resolver{Common: []string{"git"}, Lists: map[string][]string{"linux": {"fd"}}}
	got := r.Resolve("plan9")
	if len(got) != 1 || got[0] != "git" {
		t.Errorf("got %v, want just [git]", got)
	}
}

func TestResolve_EmptyCommon(t *testing.T) {
	r := Resolver{Lists: map[string][]string{"linux": {"fd"}}}
	got := r.Resolve("linux")
	if len(got) != 1 || got[0] != "fd" {
		t.Errorf("got %v", got)
	}
}

func TestNewManager_Supported(t *testing.T) {
	for _, name := range []string{"brew", "apt", "dnf", "pacman", "snap"} {
		if _, err := NewManager(name); err != nil {
			t.Errorf("NewManager(%q): %v", name, err)
		}
	}
}

func TestNewManager_Unsupported(t *testing.T) {
	if _, err := NewManager("chocopkgs"); err == nil {
		t.Error("expected error for unknown manager")
	}
}
