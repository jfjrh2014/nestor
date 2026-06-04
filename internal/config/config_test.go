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
