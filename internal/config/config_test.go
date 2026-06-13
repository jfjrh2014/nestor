package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	body := `version: 1
packages:
  common: [git]
dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy
secrets:
  provider: env
shells:
  default: zsh
profiles: {}
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}
	if cfg.Dotfiles.Strategy != "copy" {
		t.Errorf("strategy = %q, want copy", cfg.Dotfiles.Strategy)
	}
	if len(cfg.Packages.Common) != 1 || cfg.Packages.Common[0] != "git" {
		t.Errorf("packages.common = %v, want [git]", cfg.Packages.Common)
	}
}

func TestLoadMissingVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("dotfiles: {strategy: copy}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestLoadBadStrategy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 1\ndotfiles: {strategy: teleport}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	original := `version: 1
packages:
  common:
    - git
    - ripgrep
dotfiles:
  source: /tmp/dots
  strategy: copy
secrets:
  provider: env
shells:
  default: zsh
profiles: {}
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Write back and reload
	outPath := filepath.Join(dir, "out.yml")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg2, err := Load(outPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if len(cfg2.Packages.Common) != 2 {
		t.Errorf("round-trip lost packages: got %v", cfg2.Packages.Common)
	}
	if cfg2.Secrets.Provider != "env" {
		t.Errorf("round-trip lost provider: got %q", cfg2.Secrets.Provider)
	}
}

func TestExpandHome(t *testing.T) {
	got := expandHome("~/foo/bar", "/home/marcus")
	want := "/home/marcus/foo/bar"
	if got != want {
		t.Errorf("expandHome = %q, want %q", got, want)
	}
	// empty
	if expandHome("", "/h") != "" {
		t.Error("expected empty to stay empty")
	}
	// no tilde
	if expandHome("/abs/path", "/h") != "/abs/path" {
		t.Error("absolute path should not be expanded")
	}
}

func TestValidProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"personal": {Packages: []string{"discord"}},
			"work":     {Packages: []string{"slack"}},
		},
	}

	if !cfg.ValidProfile("personal") {
		t.Error("expected 'personal' to be valid")
	}
	if !cfg.ValidProfile("work") {
		t.Error("expected 'work' to be valid")
	}
	if cfg.ValidProfile("ghost") {
		t.Error("expected 'ghost' to be invalid")
	}
}

func TestProfilePackages(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"personal": {Packages: []string{"discord", "spotify"}},
			"work":     {Packages: []string{"slack"}},
		},
	}

	got := cfg.ProfilePackages("personal")
	if len(got) != 2 {
		t.Fatalf("personal packages = %d, want 2", len(got))
	}
	if got[0] != "discord" || got[1] != "spotify" {
		t.Errorf("personal packages = %v, want [discord spotify]", got)
	}

	// empty name returns nil
	if cfg.ProfilePackages("") != nil {
		t.Error("empty profile name should return nil")
	}
	// nonexistent returns nil
	if cfg.ProfilePackages("ghost") != nil {
		t.Error("nonexistent profile should return nil")
	}
}
