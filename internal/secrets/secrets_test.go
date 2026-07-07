package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
