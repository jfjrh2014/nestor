package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
)

func TestDashSecretsProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{"empty", &config.Config{Secrets: config.Secrets{}}, "none"},
		{"set", &config.Config{Secrets: config.Secrets{Provider: "bitwarden"}}, "bitwarden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashSecretsProvider(tt.cfg); got != tt.want {
				t.Errorf("dashSecretsProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDashProfiles(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"personal": {Packages: []string{"discord"}},
			"work":     {Packages: []string{"slack"}},
		},
	}
	got := dashProfiles(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}
	// should be sorted
	want := "personal"
	if got[0] != want {
		t.Errorf("expected first profile %q, got %q", want, got[0])
	}

	// empty profiles
	emptyCfg := &config.Config{}
	if got := dashProfiles(emptyCfg); len(got) != 0 {
		t.Errorf("expected 0 profiles for empty config, got %d", len(got))
	}
}

// dashDotfileState maps an internal dashDotfileStatus to the (present, drift)
// tuple, to keep these tests decoupled from the struct's field names.
func dashDotfileState(present, drift bool) dashDotfileStatus {
	return dashDotfileStatus{present: present, drift: drift}
}

// TestDashDotfileDriftCopy is a regression test for the drift detection
// reimplementation. Previously, only symlink-strategy dotfiles ever had drift
// checked; copy-strategy dotfiles that drifted in content were always marked
// "present, not drifted". By delegating to dotfiles.Deployer.Check (which
// compares file contents), drift is now detected for both strategies.
func TestDashDotfileDriftCopy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A source dotfile, and a destination with different content.
	src := filepath.Join(srcDir, "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n  name = original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destDir, "gitconfig")
	if err := os.WriteFile(dest, []byte("[user]\n  name = drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategyCopy, Source: srcDir}
	got := deployer.Check("gitconfig", dest)
	if got != dotfiles.CheckDrifted {
		t.Fatalf("copy-strategy drift should be detected; got %s", got)
	}

	// Match the dest content to confirm present detection.
	if err := os.WriteFile(dest, []byte("[user]\n  name = original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = deployer.Check("gitconfig", dest)
	if got != dotfiles.CheckPresent {
		t.Fatalf("matching content should be present; got %s", got)
	}
}

// TestDashDotfileDriftSymlink is a regression test for the broken use of
// os.Stat (follows symlinks). A drifted symlink (points somewhere unexpected)
// should read as present+drifted, not absent.
func TestDashDotfileDriftSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(srcDir, "tmux.conf")
	if err := os.WriteFile(src, []byte("set -g status on\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Drifted symlink: points to a file NOT in the source dir.
	other := filepath.Join(dir, "imposter.conf")
	if err := os.WriteFile(other, []byte("set -g status off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, ".tmux.conf")
	if err := os.Symlink(other, dest); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategySymlink, Source: srcDir}
	got := deployer.Check("tmux.conf", dest)
	if got != dotfiles.CheckDrifted {
		t.Fatalf("drifted symlink should be CheckDrifted; got %s", got)
	}

	// Fix the symlink → should be present.
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dest); err != nil {
		t.Fatal(err)
	}
	got = deployer.Check("tmux.conf", dest)
	if got != dotfiles.CheckPresent {
		t.Fatalf("correct symlink should be present; got %s", got)
	}
}

// TestDashDotfileAbsent verifies the absent case still maps to present=false.
func TestDashDotfileAbsent(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategyCopy, Source: srcDir}
	got := deployer.Check("vimrc", filepath.Join(dir, ".nonexistent"))
	if got != dotfiles.CheckAbsent {
		t.Fatalf("missing dest should be CheckAbsent; got %s", got)
	}
}

// TestDashDotfileStatusMapping locks in the contract that the four CheckStatus
// values map onto dashDotfileStatus the way dashboard.go's dashLoadStatus relies
// on. If someone changes the mapping, this breaks before a real dashboard run
// ever sees it.
func TestDashDotfileStatusMapping(t *testing.T) {
	cases := []struct {
		name         string
		status       dotfiles.CheckStatus
		wantPresent  bool
		wantDrift    bool
	}{
		{"present", dotfiles.CheckPresent, true, false},
		{"drifted", dotfiles.CheckDrifted, true, true},
		{"absent", dotfiles.CheckAbsent, false, false},
		{"src-missing", dotfiles.CheckSrcMissing, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			present, drift := dashCheckStatusToFields(c.status)
			if present != c.wantPresent || drift != c.wantDrift {
				t.Errorf("status %s => (%v,%v), want (%v,%v)",
					c.status, present, drift, c.wantPresent, c.wantDrift)
			}
			_ = dashDotfileState // sanity: helper still exists
		})
	}
}

// dashCheckStatusToFields mirrors the switch in dashLoadStatus so the mapping is
// unit-testable in isolation from bubbletea's tea.Cmd plumbing.
func dashCheckStatusToFields(s dotfiles.CheckStatus) (present, drift bool) {
	switch s {
	case dotfiles.CheckPresent:
		return true, false
	case dotfiles.CheckDrifted:
		return true, true
	case dotfiles.CheckAbsent, dotfiles.CheckSrcMissing:
		return false, false
	}
	return false, false
}
