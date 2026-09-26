package packages

import (
	"errors"
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
	// "homebrew" canonicalizes to "brew" (the legacy alias the brewfile
	// importer wrote before import and install spoke the same dialect).
	s := ParseSpec("homebrew/cask: visual-studio-code", "apt")
	if s.Manager != "brew" || s.Sub != "cask" || s.Name != "visual-studio-code" {
		t.Errorf("got %+v, want Manager=brew Sub=cask Name=visual-studio-code", s)
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

func TestParseSpec_HomebrewAliasCanonicalizes(t *testing.T) {
	// Legacy dialect written by 'nestor import brewfile' before the
	// unification: must dispatch to the brew backend, not error with
	// "unsupported package manager: homebrew".
	s := ParseSpec("homebrew: git", "apt")
	if s.Manager != "brew" || s.Name != "git" {
		t.Errorf("homebrew alias: got %+v, want manager=brew name=git", s)
	}
	s = ParseSpec("homebrew/cask: firefox", "apt")
	if s.Manager != "brew" || s.Sub != "cask" || s.Name != "firefox" {
		t.Errorf("homebrew/cask alias: got %+v, want manager=brew sub=cask name=firefox", s)
	}
}

func TestKnownManager(t *testing.T) {
	for _, name := range []string{"brew", "apt", "dnf", "pacman", "snap"} {
		if !KnownManager(name) {
			t.Errorf("KnownManager(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"homebrew", "", "chocopkgs", "brew/cask"} {
		if KnownManager(name) {
			t.Errorf("KnownManager(%q) = true, want false", name)
		}
	}
}

func TestBrew_TapNotInstallable(t *testing.T) {
	// A tap is a formula repository: IsInstalled short-circuits true
	// without shelling out (a tap can't be "list"-ed like a package), and
	// Install refuses rather than building a nonexistent "brew install
	// --tap" command.
	mr := &mockRunner{}
	defer swapRunner(mr)()

	b := brewMgr{}
	ok, err := b.IsInstalled(Spec{Name: "hashicorp/tap", Sub: "tap"})
	if err != nil {
		t.Fatalf("IsInstalled tap: %v", err)
	}
	if !ok {
		t.Error("IsInstalled tap = false, want true (taps are always present)")
	}
	if len(mr.calls) != 0 {
		t.Errorf("tap check shelled out: %v", mr.calls)
	}

	err = b.Install(Spec{Name: "hashicorp/tap", Sub: "tap"})
	if err == nil {
		t.Error("Install tap: expected refusal, got nil")
	}
	if len(mr.calls) != 0 {
		t.Errorf("tap install shelled out: %v", mr.calls)
	}
}

func TestInstallAll_HomebrewAliasDispatches(t *testing.T) {
	// End-to-end through the dispatcher: a legacy "homebrew: git" spec
	// from a pre-unification config must reach the brew backend's exact
	// command, not die in NewManager with "unsupported package manager".
	// isInstalledFailer fails IsInstalled (brew list) but succeeds on
	// Install, so InstallAll runs the full path and the recorded call is
	// the install command itself.
	isf := &isInstalledFailer{}
	defer swapRunner(isf)()

	results := InstallAll([]Spec{ParseSpec("homebrew: git", "")}, "")
	if len(results) != 1 || results[0].Status != StatusInstalled {
		t.Fatalf("results: %+v, want one installed", results)
	}
	// Both calls recorded: the IsInstalled probe (list fails) and the
	// install itself. The install must be the exact brew command.
	if len(isf.calls) != 2 {
		t.Fatalf("got %d calls (%v), want 2", len(isf.calls), isf.calls)
	}
	assertArgs(t, isf.calls[0].args, []string{"list", "--formula", "git"})
	want := cmdCall{"brew", []string{"install", "git"}}
	assertArgs(t, isf.calls[1].args, want.args)
}

// isInstalledFailer simulates a package that is not installed (list fails)
// but installs successfully — InstallAll then runs the full install path.
type isInstalledFailer struct{ calls []cmdCall }

func (f *isInstalledFailer) Run(name string, args ...string) error {
	f.calls = append(f.calls, cmdCall{name, append([]string{}, args...)})
	if len(args) > 0 && args[0] == "list" {
		return errors.New("exit status 1")
	}
	return nil
}
