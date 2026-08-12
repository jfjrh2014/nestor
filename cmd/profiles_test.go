package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runProfilesForTest isolates config path to the test and captures output.
func runProfilesForTest(t *testing.T, cfgPath string, out *bytes.Buffer) error {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	return runProfiles(out)
}

func TestProfilesEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runProfilesForTest(t, cfgPath, out); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out.String(), "no profiles defined") {
		t.Fatalf("expected 'no profiles defined', got:\n%s", out.String())
	}
}

func TestProfilesList(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
profiles:
  work:
    packages:
      - slack
      - zoom
    dotfiles:
      - src: gitconfig-work.tmpl
        dest: ~/.gitconfig
    secrets:
      - key: WORK_TOKEN
        inject:
          `+destFile+`: "token={{.WORK_TOKEN}}"
  personal:
    packages:
      - spotify
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runProfilesForTest(t, cfgPath, out); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	got := out.String()
	// profiles should be listed, sorted alphabetically
	if !strings.Contains(got, "personal") || !strings.Contains(got, "work") {
		t.Fatalf("expected both profiles in output, got:\n%s", got)
	}
	// personal should appear before work (alphabetical)
	personalIdx := strings.Index(got, "personal")
	workIdx := strings.Index(got, "work")
	if personalIdx < 0 || workIdx < 0 || personalIdx > workIdx {
		t.Fatalf("expected alphabetical order (personal before work):\n%s", got)
	}
	// summary line
	if !strings.Contains(got, "2 profile(s) total") {
		t.Fatalf("expected '2 profile(s) total', got:\n%s", got)
	}
	// package detail
	if !strings.Contains(got, "slack") || !strings.Contains(got, "spotify") {
		t.Fatalf("expected package names in detail output:\n%s", got)
	}
}

func TestProfilesMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yml")

	out := &bytes.Buffer{}
	err := runProfilesForTest(t, cfgPath, out)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

func TestProfilesCountsDisplayed(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
profiles:
  full:
    packages:
      - a
      - b
      - c
    dotfiles:
      - src: x.tmpl
        dest: ~/.x
    secrets:
      - key: K1
        inject:
          `+destFile+`: "k1={{.K1}}"
      - key: K2
        inject:
          `+destFile+`: "k2={{.K2}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runProfilesForTest(t, cfgPath, out); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	got := out.String()
	// Count summary should show all three categories
	if !strings.Contains(got, "3 packages") {
		t.Fatalf("expected '3 packages' in summary line:\n%s", got)
	}
	if !strings.Contains(got, "1 dotfiles") {
		t.Fatalf("expected '1 dotfiles' in summary line:\n%s", got)
	}
	if !strings.Contains(got, "2 secrets") {
		t.Fatalf("expected '2 secrets' in summary line:\n%s", got)
	}
}
