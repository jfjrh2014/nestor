package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockCmdOut records every command it was asked to run and returns canned
// output. out is returned when err is nil; resp lets individual commands be
// scripted by name for cases where multiple calls happen.
type mockCmdOut struct {
	calls []outCall
	out   []byte
	err   error
}

type outCall struct {
	name string
	args []string
}

func (m *mockCmdOut) Output(name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, outCall{name, append([]string{}, args...)})
	if m.err != nil {
		return nil, m.err
	}
	return m.out, nil
}

// swapCmdOut replaces the package-level cmdOut for the duration of a test
// and returns a restore func. All provider tests must use this so they never
// shell out to real secret managers.
func swapCmdOut(c cmdOutput) func() {
	old := cmdOut
	cmdOut = c
	return func() { cmdOut = old }
}

// assertOutCall fails the test unless the mock recorded exactly one call to
// name with the expected args.
func assertOutCall(t *testing.T, m *mockCmdOut, name string, args ...string) {
	t.Helper()
	if len(m.calls) != 1 {
		t.Fatalf("expected 1 command call, got %d: %v", len(m.calls), m.calls)
	}
	if m.calls[0].name != name {
		t.Fatalf("command = %q, want %q", m.calls[0].name, name)
	}
	if len(m.calls[0].args) != len(args) {
		t.Fatalf("args = %v, want %v", m.calls[0].args, args)
	}
	for i := range args {
		if m.calls[0].args[i] != args[i] {
			t.Fatalf("args = %v, want %v", m.calls[0].args, args)
		}
	}
}

func TestEnvProvider(t *testing.T) {
	t.Setenv("NESTOR_TOKEN", "abc123")
	p := EnvProvider{}
	v, err := p.Resolve("NESTOR_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if v != "abc123" {
		t.Fatalf("got %q, want abc123", v)
	}
}

func TestEnvProviderMissing(t *testing.T) {
	p := EnvProvider{}
	_, err := p.Resolve("NESTOR_DEFINITELY_NOT_SET_XYZ")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

// --- CLI-backed providers (runner-injected, no real binaries needed) ---

func TestOnePasswordProviderResolve(t *testing.T) {
	m := &mockCmdOut{out: []byte("op-secret\n")}
	defer swapCmdOut(m)()

	p := OnePasswordProvider{}
	v, err := p.Resolve("op://vault/item/field")
	if err != nil {
		t.Fatal(err)
	}
	if v != "op-secret" {
		t.Fatalf("value = %q, want %q", v, "op-secret")
	}
	assertOutCall(t, m, "op", "read", "op://vault/item/field")
	if p.Name() != "1password" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestOnePasswordProviderFailure(t *testing.T) {
	m := &mockCmdOut{err: errors.New("exit status 1")}
	defer swapCmdOut(m)()

	_, err := OnePasswordProvider{}.Resolve("op://x/y/z")
	if err == nil {
		t.Fatal("expected error when op fails")
	}
	if !strings.Contains(err.Error(), "op read") {
		t.Fatalf("error should mention command and key: %v", err)
	}
}

func TestBitwardenProviderResolve(t *testing.T) {
	m := &mockCmdOut{out: []byte("  bw-secret  \n")}
	defer swapCmdOut(m)()

	p := BitwardenProvider{}
	v, err := p.Resolve("github_token")
	if err != nil {
		t.Fatal(err)
	}
	if v != "bw-secret" {
		t.Fatalf("value = %q, want trimmed %q", v, "bw-secret")
	}
	assertOutCall(t, m, "bw", "get", "password", "github_token")
	if p.Name() != "bitwarden" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestBitwardenProviderFailure(t *testing.T) {
	m := &mockCmdOut{err: errors.New("not found")}
	defer swapCmdOut(m)()

	_, err := BitwardenProvider{}.Resolve("nope")
	if err == nil {
		t.Fatal("expected error when bw fails")
	}
	if !strings.Contains(err.Error(), "bw get password") {
		t.Fatalf("error should mention command and key: %v", err)
	}
}

func TestVaultProviderResolvePlainPath(t *testing.T) {
	// No # in the key: field defaults to "value".
	m := &mockCmdOut{out: []byte("v-secret\n")}
	defer swapCmdOut(m)()

	p := VaultProvider{}
	v, err := p.Resolve("secret/data/app")
	if err != nil {
		t.Fatal(err)
	}
	if v != "v-secret" {
		t.Fatalf("value = %q", v)
	}
	assertOutCall(t, m, "vault", "read", "-field=value", "secret/data/app")
	if p.Name() != "vault" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestVaultProviderParseKey(t *testing.T) {
	tests := []struct {
		key      string
		wantPath string
		wantArg  string // full -field flag value
	}{
		{"secret/data/app", "secret/data/app", "-field=value"},
		{"secret/data/app#password", "secret/data/app", "-field=password"},
		{"kv/prod/db#conn_str", "kv/prod/db", "-field=conn_str"},
	}
	for _, tt := range tests {
		m := &mockCmdOut{out: []byte("x\n")}
		func() {
			defer swapCmdOut(m)()
			if _, err := (VaultProvider{}).Resolve(tt.key); err != nil {
				t.Fatalf("Resolve(%q): %v", tt.key, err)
			}
		}()
		assertOutCall(t, m, "vault", "read", tt.wantArg, tt.wantPath)
	}
}

func TestVaultProviderFailure(t *testing.T) {
	m := &mockCmdOut{err: errors.New("permission denied")}
	defer swapCmdOut(m)()

	_, err := VaultProvider{}.Resolve("secret/data/app#password")
	if err == nil {
		t.Fatal("expected error when vault fails")
	}
	// The wrapped error should reference the original key (incl. #field), not
	// just the split path.
	if !strings.Contains(err.Error(), "secret/data/app#password") {
		t.Fatalf("error should mention original key: %v", err)
	}
}

func TestResolveAllMultipleCommands(t *testing.T) {
	// Two mappings through the same CLI provider: both commands run.
	m := &mockCmdOut{out: []byte("val\n")}
	defer swapCmdOut(m)()

	mappings := []Mapping{
		{Key: "github_token"},
		{Key: "api_key"},
	}
	vals, err := ResolveAll(BitwardenProvider{}, mappings)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 2 {
		t.Fatalf("expected 2 command calls, got %d", len(m.calls))
	}
	if vals["github_token"] != "val" || vals["api_key"] != "val" {
		t.Fatalf("vals = %v", vals)
	}
}

func TestInjectAllExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}

	// Dest with a ~ prefix: injectOne must expand it to the real home dir.
	// Use a throwaway path under home so the test never touches real configs.
	dest := "~/.nestor-test-inject-home"
	real := filepath.Join(home, ".nestor-test-inject-home")
	defer os.Remove(real)

	vals := map[string]string{"token": "tok"}
	mappings := []Mapping{{
		Key:    "token",
		Inject: map[string]string{dest: "token={{.Key}}"},
	}}

	results := InjectAll(vals, mappings)
	if len(results) != 1 || results[0].Status != StatusInjected {
		t.Fatalf("expected injected, got %+v", results)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatalf("dest not created at expanded home path: %v", err)
	}
	if !strings.Contains(string(data), "token=tok") {
		t.Fatalf("content = %q", data)
	}
}

func TestInjectAllNoResolvedValue(t *testing.T) {
	// Mapping present but its key absent from vals: must report an error
	// result, not panic or silently skip.
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	mappings := []Mapping{{
		Key:    "ghost_key",
		Inject: map[string]string{dest: "x={{.Key}}"},
	}}
	results := InjectAll(map[string]string{}, mappings)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusError {
		t.Fatalf("expected StatusError, got %s", results[0].Status)
	}
	if results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "no resolved value") {
		t.Fatalf("expected 'no resolved value' error, got %v", results[0].Err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("dest should not be created when value is missing")
	}
}

// TestInjectAllMkdirBlocked covers the mkdir-error branch: a regular file
// sitting where a directory would need to be created makes MkdirAll fail,
// which must surface as a StatusError result rather than a panic.
func TestInjectAllMkdirBlocked(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("i am a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(blocker, "nested", "config.yml") // blocker/file/... fails

	mappings := []Mapping{{
		Key:    "token",
		Inject: map[string]string{dest: "token={{.Key}}"},
	}}
	results := InjectAll(map[string]string{"token": "tok"}, mappings)
	if len(results) != 1 || results[0].Status != StatusError {
		t.Fatalf("expected StatusError, got %+v", results)
	}
	if results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "mkdir") {
		t.Fatalf("expected mkdir error, got %v", results[0].Err)
	}
}

// TestInjectAllOpenBlocked covers the open-error branch of the append path:
// when dest itself is a directory, ReadFile fails (EISDIR) and the code falls
// through to the append case, where OpenFile on a directory fails too.
func TestInjectAllOpenBlocked(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "iamdir")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	mappings := []Mapping{{
		Key:    "token",
		Inject: map[string]string{dest: "token={{.Key}}"},
	}}
	results := InjectAll(map[string]string{"token": "tok"}, mappings)
	if len(results) != 1 || results[0].Status != StatusError {
		t.Fatalf("expected StatusError, got %+v", results)
	}
	if results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "open") {
		t.Fatalf("expected open error, got %v", results[0].Err)
	}
}

func TestStatusString(t *testing.T) {
	if StatusInjected.String() != "injected" {
		t.Errorf("StatusInjected.String() = %q", StatusInjected.String())
	}
	if StatusError.String() != "error" {
		t.Errorf("StatusError.String() = %q", StatusError.String())
	}
	if Status(42).String() != "unknown" {
		t.Errorf("unknown status String() = %q", Status(42).String())
	}
}

// TestExpandHomeTildeAndPlain covers the home-expansion helper directly.
func TestExpandHomeTildeAndPlain(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	if got := expandHome(""); got != "" {
		t.Errorf("expandHome(\"\") = %q, want empty", got)
	}
	if got := expandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("expandHome(/abs/path) = %q", got)
	}
	if got := expandHome("~/x"); got != filepath.Join(home, "x") {
		t.Errorf("expandHome(~/x) = %q, want %q", got, filepath.Join(home, "x"))
	}
	if got := expandHome("~"); got != filepath.Join(home) {
		t.Errorf("expandHome(~) = %q, want %q", got, home)
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"env", true},
		{"", true},
		{"1password", true},
		{"bitwarden", true},
		{"vault", true},
		{"bogus", false},
	}
	for _, tt := range tests {
		p, err := NewProvider(tt.name)
		if tt.ok && err != nil {
			t.Errorf("NewProvider(%q): unexpected error %v", tt.name, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("NewProvider(%q): expected error, got %v", tt.name, p)
		}
	}
}

func TestResolveAll(t *testing.T) {
	t.Setenv("SECRET_A", "val_a")
	t.Setenv("SECRET_B", "val_b")

	mappings := []Mapping{
		{Key: "SECRET_A"},
		{Key: "SECRET_B"},
	}
	vals, err := ResolveAll(EnvProvider{}, mappings)
	if err != nil {
		t.Fatal(err)
	}
	if vals["SECRET_A"] != "val_a" {
		t.Fatalf("SECRET_A = %q", vals["SECRET_A"])
	}
	if vals["SECRET_B"] != "val_b" {
		t.Fatalf("SECRET_B = %q", vals["SECRET_B"])
	}
}

func TestResolveAllMissing(t *testing.T) {
	mappings := []Mapping{{Key: "NESTOR_NOPE"}}
	_, err := ResolveAll(EnvProvider{}, mappings)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestInjectAllAppendNew(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	vals := map[string]string{"github_token": "ghp_abc123"}
	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.Key}}"},
	}}

	results := InjectAll(vals, mappings)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusInjected {
		t.Fatalf("expected Injected, got %s (%v)", results[0].Status, results[0].Err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	want := "oauth_token: ghp_abc123\n"
	if string(data) != want {
		t.Fatalf("got %q, want %q", string(data), want)
	}
}

func TestInjectAllReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	// Write existing content with the placeholder pattern.
	os.WriteFile(dest, []byte("oauth_token: {{.Key}}\nother: yes\n"), 0o600)

	vals := map[string]string{"github_token": "ghp_replaced"}
	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.Key}}"},
	}}

	results := InjectAll(vals, mappings)
	if results[0].Status != StatusInjected {
		t.Fatalf("expected Injected, got %s (%v)", results[0].Status, results[0].Err)
	}

	data, _ := os.ReadFile(dest)
	content := string(data)
	if !strings.Contains(content, "oauth_token: ghp_replaced") {
		t.Fatalf("expected replacement in content, got %q", content)
	}
	if !strings.Contains(content, "other: yes") {
		t.Fatalf("expected 'other: yes' preserved, got %q", content)
	}
}

func TestInjectAllCreatesDir(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "deep", "nested", "config.yml")

	vals := map[string]string{"token": "xyz"}
	mappings := []Mapping{{
		Key:    "token",
		Inject: map[string]string{dest: "token={{.Key}}"},
	}}

	results := InjectAll(vals, mappings)
	if results[0].Status != StatusInjected {
		t.Fatalf("expected Injected, got %s (%v)", results[0].Status, results[0].Err)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest not created: %v", err)
	}
}

// countLines counts the number of '\n' characters in data. A trailing newline
// after content counts as one line. Empty input is zero lines.
func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

// TestInjectAllReinjectIdempotent re-runs injection with the same value and
// asserts the dest does not grow a duplicate line. This guards against the
// regression where, after the first injection, the dest holds the resolved
// value (not the literal pattern), so the pattern-match check misses and the
// code falls through to append.
func TestInjectAllReinjectIdempotent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.Key}}"},
	}}

	InjectAll(map[string]string{"github_token": "ghp_first"}, mappings)
	d1, _ := os.ReadFile(dest)
	if got := countLines(d1); got != 1 {
		t.Fatalf("after 1st inject: want 1 line, got %d (%q)", got, d1)
	}

	// Re-inject with the same value — must not append.
	InjectAll(map[string]string{"github_token": "ghp_first"}, mappings)
	d2, _ := os.ReadFile(dest)
	if got := countLines(d2); got != 1 {
		t.Fatalf("after 2nd inject: want 1 line, got %d (duplicate appended) (%q)", got, d2)
	}
}

// TestInjectAllRotatesInPlace injects a value, then re-injects with a rotated
// value. The rotated line must replace the old value, not append a new line.
func TestInjectAllRotatesInPlace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.Key}}"},
	}}

	InjectAll(map[string]string{"github_token": "ghp_old"}, mappings)
	InjectAll(map[string]string{"github_token": "ghp_new"}, mappings)

	data, _ := os.ReadFile(dest)
	content := string(data)
	if strings.Contains(content, "ghp_old") {
		t.Fatalf("stale value still present after rotation: %q", content)
	}
	if !strings.Contains(content, "oauth_token: ghp_new") {
		t.Fatalf("rotated value not present: %q", content)
	}
	if got := countLines(data); got != 1 {
		t.Fatalf("expected 1 line after rotation, got %d (%q)", got, content)
	}
}

// TestInjectAllNamedPlaceholderIdempotent exercises the {{.<key>}} (named) form
// of the placeholder, which is what the nestor.yml example uses. After a first
// injection the dest holds the resolved value; a second run must replace the
// line in place, not duplicate it.
func TestInjectAllNamedPlaceholderIdempotent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "hosts.yml")

	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.github_token}}"},
	}}

	InjectAll(map[string]string{"github_token": "ghp_one"}, mappings)
	InjectAll(map[string]string{"github_token": "ghp_two"}, mappings)

	data, _ := os.ReadFile(dest)
	content := string(data)
	if strings.Contains(content, "ghp_one") {
		t.Fatalf("stale value present after rotation: %q", content)
	}
	if !strings.Contains(content, "oauth_token: ghp_two") {
		t.Fatalf("expected rotated value, got %q", content)
	}
	if got := countLines(data); got != 1 {
		t.Fatalf("expected 1 line after re-inject, got %d (%q)", got, content)
	}
}

// TestInjectAllPreserves neighbouringLines ensures the anchor-based line
// replacement only swaps the anchored line and leaves the rest of the file
// untouched.
func TestInjectAllPreservesNeighbouringLines(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "config.yml")

	// Start from a file that already has unrelated content around the anchor.
	startup := "user: marcus\noauth_token: {{.Key}}\neditor: nvim\n"
	if err := os.WriteFile(dest, []byte(startup), 0o600); err != nil {
		t.Fatal(err)
	}

	mappings := []Mapping{{
		Key:    "github_token",
		Inject: map[string]string{dest: "oauth_token: {{.Key}}"},
	}}

	InjectAll(map[string]string{"github_token": "ghp_first"}, mappings)

	data, _ := os.ReadFile(dest)
	content := string(data)
	if !strings.HasPrefix(content, "user: marcus\n") {
		t.Fatalf("leading line lost: %q", content)
	}
	if !strings.HasSuffix(content, "\neditor: nvim\n") {
		t.Fatalf("trailing line lost: %q", content)
	}
	if !strings.Contains(content, "oauth_token: ghp_first") {
		t.Fatalf("injected value not present: %q", content)
	}
	if got := countLines(data); got != 3 {
		t.Fatalf("expected 3 lines, got %d (%q)", got, content)
	}
}

// TestPatternAnchor covers the pure helper that derives the static prefix used
// to locate an existing injection on re-runs.
func TestPatternAnchor(t *testing.T) {
	cases := []struct {
		pattern string
		want    string
	}{
		{"oauth_token: {{.Key}}", "oauth_token: "},
		{"token={{.github_token}}", "token="},
		{"no-placeholder", "no-placeholder"},
		{"", ""},
		{"{{.Key}}", ""}, // anchor is the empty prefix — no static text
	}
	for _, c := range cases {
		if got := patternAnchor(c.pattern); got != c.want {
			t.Errorf("patternAnchor(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

// TestReplaceAnchoredLine covers the line-replacement helper.
func TestReplaceAnchoredLine(t *testing.T) {
	if _, ok := replaceAnchoredLine("a\nb\nc", "", "x"); ok {
		t.Error("empty anchor should never match")
	}

	updated, ok := replaceAnchoredLine("user: bob\noauth_token: old\neditor: vim\n", "oauth_token: ", "oauth_token: new")
	if !ok {
		t.Fatal("expected a match")
	}
	want := "user: bob\noauth_token: new\neditor: vim\n"
	if updated != want {
		t.Fatalf("got %q, want %q", updated, want)
	}

	if _, ok := replaceAnchoredLine("nothing here", "missing: ", "x"); ok {
		t.Error("expected no match when anchor absent")
	}
}
