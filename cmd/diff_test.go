package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runDiffForTest points configPath at cfgPath and runs runDiffOut with the
// given profile, capturing output. Same pattern as runDoctorForTest.
func runDiffForTest(t *testing.T, cfgPath, profile string) (string, error) {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	out := &bytes.Buffer{}
	err := runDiffOut(context.Background(), profile, out)
	return out.String(), err
}

// TestDiffProfilePackagesNotDrift is the regression for session #59:
// packages installed by 'nestor up --profile X' were reported by
// 'nestor diff' as untracked drift, because diff never layered the
// profile into its notion of the configured package set.
func TestDiffProfilePackagesNotDrift(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	cfgContent := `version: 1
packages:
  common: []
  linux:
    - jq
profiles:
  work:
    packages: [vim]
dotfiles:
  source: ` + dir + `/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --profile, vim is not in the tracked set.
	out, err := runDiffForTest(t, cfgPath, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "extra (not tracked): vim") {
		t.Errorf("base diff should report vim as untracked; output:\n%s", out)
	}

	// With --profile work, vim IS tracked (and installed on the host), so:
	// no untracked line, no missing line — the drift disappears entirely.
	out, err = runDiffForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("diff --profile work: %v", err)
	}
	if strings.Contains(out, "extra (not tracked): vim") {
		t.Errorf("profile diff must not report vim as untracked; output:\n%s", out)
	}
	if strings.Contains(out, "missing: vim") {
		t.Errorf("vim is installed on the host; should not be reported missing; output:\n%s", out)
	}
	if !strings.Contains(out, "profile: work") {
		t.Errorf("profile diff should echo the active profile; output:\n%s", out)
	}
}

// TestDiffUnknownProfileErrors confirms '--profile nope' is rejected with a
// clear error rather than silently diffing the base config.
func TestDiffUnknownProfileErrors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	cfgContent := `version: 1
packages:
  common: []
dotfiles:
  templates: []
secrets:
  mappings: []
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runDiffForTest(t, cfgPath, "nope")
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "unknown profile: nope") {
		t.Fatalf("expected 'unknown profile: nope', got: %v", err)
	}
}

// TestDiffProfileDotfilesChecked confirms profile-layered dotfile templates
// participate in drift detection, mirroring 'nestor up --profile'.
func TestDiffProfileDotfilesChecked(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Profile template source exists, dest does not.
	if err := os.WriteFile(filepath.Join(srcDir, "workrc.tmpl"), []byte("export MODE=work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgContent := `version: 1
packages:
  common: []
dotfiles:
  source: ` + srcDir + `
  strategy: copy
  templates: []
secrets:
  mappings: []
profiles:
  work:
    dotfiles:
      - src: workrc.tmpl
        dest: ` + filepath.Join(dir, "workrc") + `
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --profile the template isn't checked at all.
	out, err := runDiffForTest(t, cfgPath, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if strings.Contains(out, "workrc") {
		t.Errorf("base diff must not check profile dotfiles; output:\n%s", out)
	}

	// With --profile the template is checked and reported missing.
	out, err = runDiffForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("diff --profile work: %v", err)
	}
	if !strings.Contains(out, "missing: "+filepath.Join(dir, "workrc")) {
		t.Errorf("profile diff should report workrc dest missing; output:\n%s", out)
	}
}

// TestDiffForeignLinkAdvisesCorrectly is the regression for the copy-strategy
// symlink-dest misreport: Check called a foreign link "drifted", so diff's
// summary told the user to run 'nestor up' — which refuses symlink dests
// under copy strategy. The verdict and the advice must both name the real
// problem: a link another tool owns, fixable by removing it or by switching
// dotfiles.strategy to symlink.
func TestDiffForeignLinkAdvisesCorrectly(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	srcDir := filepath.Join(dir, "dotfiles")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "gitconfig"), []byte("[user]\n\tname = t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Dest is a symlink whose target content MATCHES src: under the old code
	// this read as plain drift ("run up to fix").
	target := filepath.Join(dir, "upstream")
	if err := os.WriteFile(target, []byte("[user]\n\tname = t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, ".gitconfig")
	if err := os.Symlink(target, dest); err != nil {
		t.Fatal(err)
	}

	cfgContent := `version: 1
packages:
  common: []
dotfiles:
  source: ` + srcDir + `
  strategy: copy
  templates:
    - src: gitconfig
      dest: ` + dest + `
secrets:
  mappings: []
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDiffForTest(t, cfgPath, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "symlink owned by another tool") {
		t.Errorf("diff should name the foreign link, not plain drift; output:\n%s", out)
	}
	if !strings.Contains(out, "dotfiles.strategy to symlink") {
		t.Errorf("diff should offer the strategy-switch remedy; output:\n%s", out)
	}
	// The dangerous advice must be gone: no bare "drifted:" line for this dest.
	if strings.Contains(out, "drifted: "+dest) {
		t.Errorf("foreign link must not read as plain drift; output:\n%s", out)
	}
}
