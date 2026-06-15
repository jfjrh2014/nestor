package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !strings.Contains(body, "issue(s) found") {
		t.Fatalf("expected issue count, got: %s", body)
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

// runSecretsCheckForTest runs the check logic with a given config path and
// captures output. Returns 0 on success, 1 on error.
func runSecretsCheckForTest(t *testing.T, cfgPath string, out *bytes.Buffer) int {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	// The runSecretsCheck creates its own ui.New(os.Stdout), so we need to
	// capture via a pipe.
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		out.Write(buf[:n])
		close(done)
	}()

	err := runSecretsCheck(context.Background())
	w.Close()
	<-done

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

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		out.Write(buf[:n])
		close(done)
	}()

	err := runSecretsInject(context.Background())
	w.Close()
	<-done

	if err != nil {
		return 1
	}
	return 0
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
