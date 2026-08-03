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

	if err := runListOut(context.Background(), out); err != nil {
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
