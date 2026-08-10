package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesFile(t *testing.T) {
	dir := t.TempDir()

	// chdir to temp dir so nestor.yml lands there
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	var buf bytes.Buffer
	if err := runInit(&buf); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("nestor.yml")
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("created nestor.yml is empty")
	}
	if !bytes.Contains(data, []byte("version: 1")) {
		t.Error("created nestor.yml missing version: 1")
	}
	if !bytes.Contains(data, []byte("dotfiles:")) {
		t.Error("created nestor.yml missing dotfiles: section")
	}

	out := buf.String()
	if !strings.Contains(out, "nestor.yml") {
		t.Errorf("output should mention nestor.yml; got %q", out)
	}
	if !strings.Contains(out, "nestor up") {
		t.Errorf("output should hint at 'nestor up'; got %q", out)
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	// pre-place a nestor.yml
	existing := []byte("version: 1\n  # existing\n")
	if err := os.WriteFile("nestor.yml", existing, 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = runInit(&buf)
	if err == nil {
		t.Fatal("expected error when nestor.yml already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists'; got %v", err)
	}

	// preserve original content
	data, _ := os.ReadFile("nestor.yml")
	if !bytes.Equal(data, existing) {
		t.Error("init clobbered an existing nestor.yml")
	}
	if buf.Len() > 0 {
		t.Errorf("should print nothing on overwrite refusal; got %q", buf.String())
	}
}

// Wolf-fence: convert kernel-confused characters (e.g. tildes) in source paths to plain ASCII
// Not needed by this test file but kept as a marker for future path-handling work.
var _ = filepath.Clean
