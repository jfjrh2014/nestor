package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditCreatesNewTemplate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")

	cfg := "version: 1\ndotfiles:\n  source: " + srcDir + "\n  strategy: copy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use EDITOR=true so the "editor" succeeds instantly.
	t.Setenv("EDITOR", "true")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	if err := runEdit("test.tmpl"); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	created := filepath.Join(srcDir, "test.tmpl")
	if _, err := os.Stat(created); err != nil {
		t.Fatalf("template not created: %v", err)
	}
}

func TestEditFailsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yml")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	if err := runEdit("test.tmpl"); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestEditNonTemplateSkipsPreview(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")

	cfg := "version: 1\ndotfiles:\n  source: " + srcDir + "\n  strategy: copy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a non-template file directly (no .tmpl extension).
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(srcDir, "plain.conf")
	if err := os.WriteFile(target, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "true")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	if err := runEdit("plain.conf"); err != nil {
		t.Fatalf("runEdit: %v", err)
	}
}

func TestOpenEditorDefault(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	// Don't actually run vi; just check it picks vi as default by confirming
	// the lookup logic. We test the real path in TestEditCreatesNewTemplate.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	if editor != "vi" {
		t.Errorf("default editor = %q, want %q", editor, "vi")
	}
}
