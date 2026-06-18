package cmd

import (
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestDashSecretsProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{"empty", &config.Config{Secrets: config.Secrets{}}, "none"},
		{"set", &config.Config{Secrets: config.Secrets{Provider: "bitwarden"}}, "bitwarden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashSecretsProvider(tt.cfg); got != tt.want {
				t.Errorf("dashSecretsProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDashProfiles(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"personal": {Packages: []string{"discord"}},
			"work":     {Packages: []string{"slack"}},
		},
	}
	got := dashProfiles(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}
	// should be sorted
	want := "personal"
	if got[0] != want {
		t.Errorf("expected first profile %q, got %q", want, got[0])
	}

	// empty profiles
	emptyCfg := &config.Config{}
	if got := dashProfiles(emptyCfg); len(got) != 0 {
		t.Errorf("expected 0 profiles for empty config, got %d", len(got))
	}
}

func TestDashExpandTilde(t *testing.T) {
	tests := []struct {
		path string
		home string
		want string
	}{
		{"~/.config", "/home/user", "/home/user/.config"},
		{"~/", "/root", "/root"},
		{"/abs/path", "/home/user", "/abs/path"},
		{"relative", "/home/user", "relative"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := dashExpandTilde(tt.path, tt.home); got != tt.want {
				t.Errorf("dashExpandTilde(%q, %q) = %v, want %v", tt.path, tt.home, got, tt.want)
			}
		})
	}
}
