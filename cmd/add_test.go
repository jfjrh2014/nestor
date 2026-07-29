package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

// writeTestConfig writes a minimal valid nestor.yml to path and returns it.
func writeTestConfig(t *testing.T, path string) {
	t.Helper()
	content := `version: 1
packages:
  common:
    - git
dotfiles:
  source: /tmp/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
shells:
  default: bash
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddPackage(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	if err := addPackage("ripgrep", &out); err != nil {
		t.Fatalf("addPackage: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load after add: %v", err)
	}
	found := false
	for _, p := range cfg.Packages.Common {
		if p == "ripgrep" {
			found = true
		}
	}
	if !found {
		t.Error("ripgrep not in common packages after add")
	}
}

func TestAddPackageDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	if err := addPackage("git", &out); err != nil {
		t.Fatalf("addPackage: %v", err)
	}
	if !strings.Contains(out.String(), "already in common") {
		t.Errorf("expected duplicate message, got: %s", out.String())
	}

	// Config should be unchanged — only one git entry
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range cfg.Packages.Common {
		if p == "git" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 git entry, got %d", count)
	}
}

func TestAddDotfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	if err := addDotfile("~/.bashrc", &out); err != nil {
		t.Fatalf("addDotfile: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load after add: %v", err)
	}
	if len(cfg.Dotfiles.Templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(cfg.Dotfiles.Templates))
	}
	if cfg.Dotfiles.Templates[0].Dest != "~/.bashrc" {
		t.Errorf("expected dest ~/.bashrc, got %q", cfg.Dotfiles.Templates[0].Dest)
	}
}

func TestAddDotfileDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	if err := addDotfile("~/.bashrc", &out); err != nil {
		t.Fatalf("first addDotfile: %v", err)
	}
	out.Reset()
	if err := addDotfile("~/.bashrc", &out); err != nil {
		t.Fatalf("second addDotfile: %v", err)
	}
	if !strings.Contains(out.String(), "already in config") {
		t.Errorf("expected duplicate message, got: %s", out.String())
	}

	// Config must still load cleanly and have exactly one entry
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load after dup: %v", err)
	}
	if len(cfg.Dotfiles.Templates) != 1 {
		t.Errorf("expected 1 template after dup add, got %d", len(cfg.Dotfiles.Templates))
	}
}

func TestAddSecretWithTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	// Simulate user providing a dest path and accepting the default pattern
	input := "~/.config/gh/hosts.yml\n\n"
	var out bytes.Buffer
	if err := addSecret("github_token", strings.NewReader(input), &out); err != nil {
		t.Fatalf("addSecret: %v", err)
	}

	// The written config MUST pass validate() — this was the core bug.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load after add secret: %v", err)
	}
	if len(cfg.Secrets.Mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(cfg.Secrets.Mappings))
	}
	m := cfg.Secrets.Mappings[0]
	if m.Key != "github_token" {
		t.Errorf("expected key github_token, got %q", m.Key)
	}
	if len(m.Inject) == 0 {
		t.Fatal("inject map is empty — would fail validate() on reload")
	}
}

func TestAddSecretSkipTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	// User skips the inject target prompt
	input := "\n"
	var out bytes.Buffer
	if err := addSecret("api_key", strings.NewReader(input), &out); err != nil {
		t.Fatalf("addSecret: %v", err)
	}
	if !strings.Contains(out.String(), "no injection target set") {
		t.Errorf("expected skip warning, got: %s", out.String())
	}

	// The config should still load — but validate() will reject empty inject.
	// This is intentional: the user explicitly skipped, and the message tells
	// them to edit nestor.yml before running 'nestor up'. The old code wrote
	// this exact shape silently with no warning.
	_, err := config.Load(cfgPath)
	if err == nil {
		t.Error("expected config.Load to fail on empty inject (validate rejects it), but it passed")
	}
}

func TestAddSecretDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	// First add with a target
	input := "~/.env\nAPI_KEY={{.api_key}}\n"
	var out bytes.Buffer
	if err := addSecret("api_key", strings.NewReader(input), &out); err != nil {
		t.Fatalf("first addSecret: %v", err)
	}

	// Second add of same key
	out.Reset()
	input = "\n"
	if err := addSecret("api_key", strings.NewReader(input), &out); err != nil {
		t.Fatalf("second addSecret: %v", err)
	}
	if !strings.Contains(out.String(), "already in config") {
		t.Errorf("expected duplicate message, got: %s", out.String())
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if len(cfg.Secrets.Mappings) != 1 {
		t.Errorf("expected 1 mapping after dup add, got %d", len(cfg.Secrets.Mappings))
	}
}

func TestPromptInjectTarget(t *testing.T) {
	// Both dest and pattern provided
	t.Run("both provided", func(t *testing.T) {
		input := "~/.bashrc\nFOO={{.foo}}\n"
		var out bytes.Buffer
		dest, pattern := promptInjectTarget("foo", strings.NewReader(input), &out)
		if dest != "~/.bashrc" {
			t.Errorf("dest = %q, want ~/.bashrc", dest)
		}
		if pattern != "FOO={{.foo}}" {
			t.Errorf("pattern = %q, want FOO={{.foo}}", pattern)
		}
	})

	// Dest provided, pattern empty → default pattern
	t.Run("default pattern", func(t *testing.T) {
		input := "~/.env\n\n"
		var out bytes.Buffer
		dest, pattern := promptInjectTarget("api_key", strings.NewReader(input), &out)
		if dest != "~/.env" {
			t.Errorf("dest = %q, want ~/.env", dest)
		}
		want := "api_key: {{.api_key}}"
		if pattern != want {
			t.Errorf("pattern = %q, want %q", pattern, want)
		}
	})

	// Dest empty → skip entirely
	t.Run("skip", func(t *testing.T) {
		input := "\n"
		var out bytes.Buffer
		dest, pattern := promptInjectTarget("foo", strings.NewReader(input), &out)
		if dest != "" || pattern != "" {
			t.Errorf("expected empty dest+pattern, got %q/%q", dest, pattern)
		}
	})
}

// TestAddPackageEmptyName guards against panics / silent corruption when a
// caller passes an empty package name. The handler must return an error,
// not crash or write a garbage entry to config.
func TestAddPackageEmptyName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	err := addPackage("", &out)
	if err == nil {
		t.Fatal("expected error for empty package name, got nil")
	}
	// Config must be unchanged after a rejected add.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	for _, p := range cfg.Packages.Common {
		if p == "" {
			t.Error("empty string leaked into common packages")
		}
	}
}

// TestAddDotfileEmptyName exercises the original crash: addDotfile("") used to
// index name[0] without a length check, panicking with "index out of range".
func TestAddDotfileEmptyName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	err := addDotfile("", &out)
	if err == nil {
		t.Fatal("expected error for empty dotfile name, got nil")
	}
	// The key assertion: no panic. We got here, so the guard works.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if len(cfg.Dotfiles.Templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(cfg.Dotfiles.Templates))
	}
}

// TestAddSecretEmptyName ensures an empty secret key is rejected before any
// config is loaded or written.
func TestAddSecretEmptyName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeTestConfig(t, cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	err := addSecret("", strings.NewReader("\n"), &out)
	if err == nil {
		t.Fatal("expected error for empty secret name, got nil")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if len(cfg.Secrets.Mappings) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(cfg.Secrets.Mappings))
	}
}
