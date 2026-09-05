package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runListForTest runs the list logic with config and output isolated to the
// test. Returns 0 on success, 1 on error.
func runListForTest(t *testing.T, cfgPath string, out *bytes.Buffer) int {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	if err := runListOut(context.Background(), "", out); err != nil {
		return 1
	}
	return 0
}

// TestListEmptyProviderWithMappings is the regression test for the empty-provider
// guard bug in `nestor list`. A config with secret mappings but no explicit
// `provider:` line is valid (defaults to env), but list used to guard on
// `cfg.Secrets.Provider == ""` in addition to mapping count, hiding the user's
// configured mappings behind a false "no secrets declared".
func TestListEmptyProviderWithMappings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
secrets:
  mappings:
    - key: API_TOKEN
      inject:
        `+filepath.Join(dir, "app.conf")+`: "token={{.API_TOKEN}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if code := runListForTest(t, cfgPath, out); code != 0 {
		t.Fatalf("expected exit 0, got %d (output: %s)", code, out.String())
	}

	if strings.Contains(out.String(), "no secrets declared") {
		t.Fatalf("regression: list reported 'no secrets declared' for a config with mappings\n%s", out.String())
	}
	if !strings.Contains(out.String(), "API_TOKEN") {
		t.Fatalf("expected 'API_TOKEN' in output, got:\n%s", out.String())
	}
}

// TestListNoSecretsDeclaredStillWorks confirms the non-regression: a config
// with zero mappings still prints "no secrets declared".
func TestListNoSecretsDeclaredStillWorks(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
secrets:
  provider: env
  mappings: []
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if code := runListForTest(t, cfgPath, out); code != 0 {
		t.Fatalf("expected exit 0, got %d (output: %s)", code, out.String())
	}

	if !strings.Contains(out.String(), "no secrets declared") {
		t.Fatalf("expected 'no secrets declared' in output, got:\n%s", out.String())
	}
}

// --- session #69: profile-aware list ---

// runListProfileForTest runs the list logic with an isolated config and a
// named profile ("" = base only). Returns the output text and any error.
func runListProfileForTest(t *testing.T, cfgPath, profile string) (string, error) {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	err := runListOut(context.Background(), profile, &out)
	return out.String(), err
}

// TestListProfileLayersPackagesDotfilesSecrets verifies 'nestor list --profile'
// reports exactly what 'up --profile' deploys: base + profile packages, the
// profile's dotfiles (with live status), and the profile's secret mappings.
// Sixth member of the profile-blindness family (diff #59, doctor #62,
// secrets #63, ci #64, dashboard #65): list showed base-only status while the
// machine was profile-managed.
func TestListProfileLayersPackagesDotfilesSecrets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "workrc.tmpl"), []byte("export WORK=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-deploy the profile dotfile so copy-strategy Check reads present.
	if err := os.WriteFile(filepath.Join(dir, ".workrc"), []byte("export WORK=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "nestor.yml")
	cfgContent := `version: 1
packages:
  common: [jq]
dotfiles:
  source: ` + srcDir + `
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings:
    - key: API_TOKEN
      inject:
        app.conf: "token={{.API_TOKEN}}"
profiles:
  work:
    packages: [vim]
    dotfiles:
      - src: workrc.tmpl
        dest: ~/.workrc
    secrets:
      - key: WORK_TOKEN
        inject:
          work.conf: "token={{.WORK_TOKEN}}"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Base-only: profile items must not leak into the default view.
	base, err := runListProfileForTest(t, cfgPath, "")
	if err != nil {
		t.Fatalf("base list: %v", err)
	}
	for _, leaked := range []string{"vim", ".workrc", "WORK_TOKEN"} {
		if strings.Contains(base, leaked) {
			t.Errorf("base-only list leaked profile item %q; output:\n%s", leaked, base)
		}
	}
	if !strings.Contains(base, "jq") || !strings.Contains(base, "API_TOKEN") {
		t.Errorf("base list missing base items; output:\n%s", base)
	}
	if !strings.Contains(base, "no templates declared") {
		t.Errorf("base list should report no templates; output:\n%s", base)
	}

	// With profile work: base + profile items, deployed profile dotfile present,
	// summary counts both layers.
	got, err := runListProfileForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("profile list: %v", err)
	}
	for _, want := range []string{"profile: work", "vim", ".workrc", "present", "WORK_TOKEN", "3 managed"} {
		if !strings.Contains(got, want) {
			t.Errorf("profile list missing %q; output:\n%s", want, got)
		}
	}
}

// TestListUnknownProfileErrors pins the family-wide rule: an unknown profile
// is a hard error before any status work.
func TestListUnknownProfileErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runListProfileForTest(t, cfgPath, "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown profile: nope") {
		t.Fatalf("expected unknown-profile error, got: %v", err)
	}
}
