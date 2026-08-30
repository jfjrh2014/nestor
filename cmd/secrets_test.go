package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestSecretsCheckNoSecrets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, "version: 1\n")

	out := &bytes.Buffer{}
	code := runSecretsCheckForTest(t, cfgPath, out)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "no secrets declared") {
		t.Fatalf("expected 'no secrets declared', got: %s", out.String())
	}
}

func TestSecretsCheckEnvProvider(t *testing.T) {
	t.Setenv("NESTOR_TEST_KEY", "supersecret")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_TEST_KEY
      inject:
        ~/.nestor-test: "token={{.NESTOR_TEST_KEY}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsCheckForTest(t, cfgPath, out)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	body := out.String()
	if !strings.Contains(body, "NESTOR_TEST_KEY reachable") {
		t.Fatalf("expected key reachable, got: %s", body)
	}
	// Regression: the per-key loop must surface a count of resolved keys.
	// Pre-fix, resolved/failedCount were incremented but never reported.
	if !strings.Contains(body, "1 secret(s) resolved") {
		t.Fatalf("expected resolved-count summary, got: %s", body)
	}
	if !strings.Contains(body, "all secrets reachable") {
		t.Fatalf("expected all reachable, got: %s", body)
	}
}

func TestSecretsCheckMissingKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_NONEXISTENT_KEY
      inject:
        ~/.nestor-test: "token={{.NESTOR_NONEXISTENT_KEY}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsCheckForTest(t, cfgPath, out)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	body := out.String()
	if !strings.Contains(body, "not set") {
		t.Fatalf("expected 'not set' error, got: %s", body)
	}
	// Regression: a missing key must surface in the resolve summary, not just the diagnosis.
	if !strings.Contains(body, "0 resolved, 1 failed") {
		t.Fatalf("expected '0 resolved, 1 failed' summary, got: %s", body)
	}
	if !strings.Contains(body, "issue(s) found") {
		t.Fatalf("expected issue count, got: %s", body)
	}
}

// TestSecretsCheckMixedResolve verifies the resolve summary line reports
// correct counts when some keys resolve and others don't.
func TestSecretsCheckMixedResolve(t *testing.T) {
	t.Setenv("NESTOR_GOOD_KEY", "present")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_GOOD_KEY
      inject:
        ~/.nestor-a: "a={{.NESTOR_GOOD_KEY}}"
    - key: NESTOR_BAD_KEY
      inject:
        ~/.nestor-b: "b={{.NESTOR_BAD_KEY}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsCheckForTest(t, cfgPath, out)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	body := out.String()
	// One resolved, one failed — both must appear in the new summary line.
	if !strings.Contains(body, "1 resolved, 1 failed") {
		t.Fatalf("expected '1 resolved, 1 failed', got: %s", body)
	}
	if !strings.Contains(body, "NESTOR_GOOD_KEY reachable") {
		t.Fatalf("expected good key marked reachable, got: %s", body)
	}
	if !strings.Contains(body, "NESTOR_BAD_KEY") {
		t.Fatalf("expected bad key mentioned in output, got: %s", body)
	}
}

func TestSecretsInjectCreatesFile(t *testing.T) {
	t.Setenv("NESTOR_TEST_TOKEN", "tok123")
	dir := t.TempDir()
	destFile := filepath.Join(dir, "target.conf")
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_TEST_TOKEN
      inject:
        `+destFile+`: "api_token={{.NESTOR_TEST_TOKEN}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsInjectForTest(t, cfgPath, out)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("dest file not created: %v", err)
	}
	if !strings.Contains(string(data), "api_token=tok123") {
		t.Fatalf("unexpected file content: %s", string(data))
	}
	if !strings.Contains(out.String(), "1 secrets injected") {
		t.Fatalf("expected success message, got: %s", out.String())
	}
}

// TestSecretsCheckEmptyProviderWithMappings is the regression test for the
// "no secrets declared" guard bug. A config with secret mappings but an
// omitted (empty) provider is valid: config.validate() allows it,
// secrets.NewProvider("") returns the env default, and the provider literal
// must never be used as a proxy for "has secrets to act on". Before the fix
// the OR guard (provider=="" || len(mappings)==0) in runSecretsCheck /
// runSecretsInject / runDoctor short-circuited on provider=="" and silently
// reported "no secrets declared", hiding real mappings from the user.
func TestSecretsCheckEmptyProviderWithMappings(t *testing.T) {
	t.Setenv("NESTOR_EMPTY_PROVID_KEY", "present")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  mappings:
    - key: NESTOR_EMPTY_PROVID_KEY
      inject:
        `+filepath.Join(dir, "target")+`: "k={{.NESTOR_EMPTY_PROVID_KEY}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsCheckForTest(t, cfgPath, out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (output: %s)", code, out.String())
	}

	body := out.String()
	if strings.Contains(body, "no secrets declared") {
		t.Fatalf("regression: empty-provider-with-mappings reported 'no secrets declared'\n%s", body)
	}
	if !strings.Contains(body, "NESTOR_EMPTY_PROVID_KEY reachable") {
		t.Fatalf("expected the mapping key to be resolved and reported reachable\n%s", body)
	}
	if !strings.Contains(body, "provider: env") {
		t.Fatalf("expected provider to be reported as the resolved env default\n%s", body)
	}
}

// TestSecretsInjectEmptyProviderWithMappings mirrors the regression on the
// inject path: an empty provider must still produce a real injection.
func TestSecretsInjectEmptyProviderWithMappings(t *testing.T) {
	t.Setenv("NESTOR_EMPTY_PROVID_INJ", "injected-val")
	dir := t.TempDir()
	destFile := filepath.Join(dir, "out.conf")
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  mappings:
    - key: NESTOR_EMPTY_PROVID_INJ
      inject:
        `+destFile+`: "v={{.NESTOR_EMPTY_PROVID_INJ}}"
`)

	out := &bytes.Buffer{}
	code := runSecretsInjectForTest(t, cfgPath, out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (output: %s)", code, out.String())
	}

	if strings.Contains(out.String(), "no secrets declared") {
		t.Fatalf("regression: inject reported 'no secrets declared' for a config with mappings\n%s", out.String())
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("dest file not created: %v", err)
	}
	if !strings.Contains(string(data), "v=injected-val") {
		t.Fatalf("expected injected value in file, got: %s", string(data))
	}
}

// runSecretsCheckForTest runs the check logic with output written directly to
// the provided buffer. Returns 0 on success, 1 on error.
func runSecretsCheckForTest(t *testing.T, cfgPath string, out *bytes.Buffer) int {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	err := runSecretsCheck(context.Background(), out)
	if err != nil {
		return 1
	}
	return 0
}

// runSecretsInjectForTest runs the inject logic similarly.
func runSecretsInjectForTest(t *testing.T, cfgPath string, out *bytes.Buffer) int {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	err := runSecretsInject(context.Background(), out)
	if err != nil {
		return 1
	}
	return 0
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// runSecretsCheckProfileForTest points configPath at cfgPath and runs
// runSecretsCheckProfileOut with the given profile. Pattern mirrors
// runDiffForTest (session #59) and runDoctorProfileOut (session #62).
func runSecretsCheckProfileForTest(t *testing.T, cfgPath, profile string) (string, error) {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	out := &bytes.Buffer{}
	err := runSecretsCheckProfileOut(context.Background(), profile, out)
	return out.String(), err
}

func runSecretsInjectProfileForTest(t *testing.T, cfgPath, profile string) (string, error) {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	out := &bytes.Buffer{}
	err := runSecretsInjectProfileOut(context.Background(), profile, out)
	return out.String(), err
}

// TestSecretsCheckProfileExtraKeys is the session #63 regression:
// 'nestor up --profile X' injects the profile's secret mappings (up.go
// Step 5), but 'nestor secrets check' only ever read the base config, so
// profile-only and profile-extra keys were invisible to the dry run.
func TestSecretsCheckProfileExtraKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_BASE_KEY
      inject:
        ~/.nestor-test: "base={{.NESTOR_BASE_KEY}}"
profiles:
  work:
    secrets:
      - key: NESTOR_PROFILE_KEY
        inject:
          ~/.nestor-work-test: "work={{.NESTOR_PROFILE_KEY}}"
`)
	t.Setenv("NESTOR_BASE_KEY", "b")
	t.Setenv("NESTOR_PROFILE_KEY", "p")

	// Base run: only the base key is checked; the profile key is invisible.
	body, err := runSecretsCheckProfileForTest(t, cfgPath, "")
	if err != nil {
		t.Fatalf("base check: %v", err)
	}
	if !strings.Contains(body, "1 secret(s) resolved") {
		t.Fatalf("base run should resolve exactly 1, got: %s", body)
	}
	if strings.Contains(body, "NESTOR_PROFILE_KEY") {
		t.Fatalf("base run should not mention the profile key, got: %s", body)
	}

	// Profile run: both keys are checked.
	body, err = runSecretsCheckProfileForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("profile check: %v", err)
	}
	if !strings.Contains(body, "profile work: 1 extra secrets") {
		t.Fatalf("expected profile line, got: %s", body)
	}
	if !strings.Contains(body, "2 secret(s) resolved") {
		t.Fatalf("profile run should resolve 2, got: %s", body)
	}
	if !strings.Contains(body, "NESTOR_PROFILE_KEY reachable") {
		t.Fatalf("profile key should be checked, got: %s", body)
	}
}

// TestSecretsCheckProfileOnlyKeyingOnCount: base config with no secrets at
// all plus a profile that declares them — "no secrets declared" would be a
// lie for a work machine. Mirrors the #34/#35/#36 empty-provider lesson:
// map counts decide, nothing else.
func TestSecretsCheckProfileOnlySecrets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
profiles:
  work:
    secrets:
      - key: NESTOR_PROFILE_ONLY
        inject:
          ~/.nestor-test: "v={{.NESTOR_PROFILE_ONLY}}"
`)
	t.Setenv("NESTOR_PROFILE_ONLY", "yes")

	body, err := runSecretsCheckProfileForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("profile check: %v", err)
	}
	if strings.Contains(body, "no secrets declared") {
		t.Fatalf("profile secrets must not be hidden, got: %s", body)
	}
	if !strings.Contains(body, "1 secret(s) resolved") {
		t.Fatalf("expected 1 resolved, got: %s", body)
	}
}

// TestSecretsInjectProfileWritesProfileTarget proves the profile layer is
// actually injected to disk, not just counted.
func TestSecretsInjectProfileWritesProfileTarget(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, `version: 1
secrets:
  provider: env
  mappings:
    - key: NESTOR_INJ_BASE
      inject:
        `+dir+`/base.txt: "base={{.NESTOR_INJ_BASE}}"
profiles:
  work:
    secrets:
      - key: NESTOR_INJ_WORK
        inject:
          `+dir+`/work.txt: "work={{.NESTOR_INJ_WORK}}"
`)
	t.Setenv("NESTOR_INJ_BASE", "B")
	t.Setenv("NESTOR_INJ_WORK", "W")

	body, err := runSecretsInjectProfileForTest(t, cfgPath, "work")
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if !strings.Contains(body, "resolving 2 secrets") {
		t.Fatalf("expected 2 secrets resolved, got: %s", body)
	}
	if !strings.Contains(body, "2 secrets injected") {
		t.Fatalf("expected 2 injected, got: %s", body)
	}
	base, baseErr := os.ReadFile(filepath.Join(dir, "base.txt"))
	work, workErr := os.ReadFile(filepath.Join(dir, "work.txt"))
	if baseErr != nil || workErr != nil {
		t.Fatalf("both targets must exist: base=%v work=%v", baseErr, workErr)
	}
	if string(base) != "base=B\n" || string(work) != "work=W\n" {
		t.Fatalf("wrong contents: base=%q work=%q", base, work)
	}
}

// TestSecretsCheckUnknownProfile: like diff (#59) and doctor (#62), an
// unknown profile is a hard error, not a silent base-only run.
func TestSecretsCheckUnknownProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	writeFile(t, cfgPath, "version: 1\n")

	_, err := runSecretsCheckProfileForTest(t, cfgPath, "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown profile: nope") {
		t.Fatalf("expected unknown profile error, got: %v", err)
	}

	_, err = runSecretsInjectProfileForTest(t, cfgPath, "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown profile: nope") {
		t.Fatalf("expected unknown profile error from inject, got: %v", err)
	}
}

// TestEffectiveSecretMappings locks the resolution table in one place.
func TestEffectiveSecretMappings(t *testing.T) {
	base := []config.Mapping{{Key: "b"}}
	prof := []config.Mapping{{Key: "p"}}

	cases := []struct {
		name      string
		cfg       *config.Config
		profile   string
		wantLen   int
		wantExtra int
		wantErr   bool
	}{
		{"no profile", &config.Config{Secrets: config.Secrets{Mappings: base}}, "", 1, 0, false},
		{"profile layers", &config.Config{
			Secrets:  config.Secrets{Mappings: base},
			Profiles: map[string]config.Profile{"work": {SecretMappings: prof}},
		}, "work", 2, 1, false},
		{"profile empty stays base", &config.Config{
			Secrets:  config.Secrets{Mappings: base},
			Profiles: map[string]config.Profile{"work": {}},
		}, "work", 1, 0, false},
		{"unknown profile", &config.Config{}, "ghost", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, extra, err := effectiveSecretMappings(tc.cfg, tc.profile)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantLen || len(extra) != tc.wantExtra {
				t.Fatalf("got %d mappings (%d extra), want %d (%d extra)", len(got), len(extra), tc.wantLen, tc.wantExtra)
			}
		})
	}
}
