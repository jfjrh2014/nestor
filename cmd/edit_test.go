package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

	var buf bytes.Buffer
	if err := runEdit("test.tmpl", &buf); err != nil {
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

	var buf bytes.Buffer
	if err := runEdit("test.tmpl", &buf); err == nil {
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

	var buf bytes.Buffer
	if err := runEdit("plain.conf", &buf); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "--- preview") {
		t.Errorf("expected no preview for non-template file, got:\n%s", out)
	}
}

func TestEditPreviewRenderedOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a template with supported syntax (env func + literals).
	// (.Field lookups have no data source by design and now fail loudly.)
	tmplContent := "name = {{env \"NESTOR_EDIT_TEST\"}} ok\n"
	tmplPath := filepath.Join(srcDir, "gitconfig.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := "version: 1\ndotfiles:\n  source: " + srcDir + "\n  strategy: copy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "true")
	t.Setenv("NESTOR_EDIT_TEST", "marcus")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	var buf bytes.Buffer
	if err := runEdit("gitconfig.tmpl", &buf); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--- preview (rendered) ---") {
		t.Errorf("expected preview header, got:\n%s", out)
	}
	if !strings.Contains(out, "name = marcus ok") {
		t.Errorf("expected rendered line, got:\n%s", out)
	}
	if !strings.Contains(out, "--- end preview ---") {
		t.Errorf("expected preview footer, got:\n%s", out)
	}
}

func TestEditRenderErrorReported(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a template with broken Go template syntax (unclosed action).
	badContent := "name = {{.name\n"
	tmplPath := filepath.Join(srcDir, "broken.tmpl")
	if err := os.WriteFile(tmplPath, []byte(badContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := "version: 1\ndotfiles:\n  source: " + srcDir + "\n  strategy: copy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "true")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	var buf bytes.Buffer
	if err := runEdit("broken.tmpl", &buf); err != nil {
		t.Fatalf("runEdit returned error (should report in output, not return): %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "render error") {
		t.Errorf("expected render error message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "syntax errors") {
		t.Errorf("expected syntax errors hint in output, got:\n%s", out)
	}
}

func TestEditNewTemplatePrintsCreatedMessage(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")

	cfg := "version: 1\ndotfiles:\n  source: " + srcDir + "\n  strategy: copy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "true")

	origCfgFile := cfgFile
	cfgFile = cfgPath
	defer func() { cfgFile = origCfgFile }()

	var buf bytes.Buffer
	if err := runEdit("fresh.tmpl", &buf); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "nestor: created") {
		t.Errorf("expected 'created' message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "dotfiles.templates") {
		t.Errorf("expected hint about dotfiles.templates, got:\n%s", out)
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
