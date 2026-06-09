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
