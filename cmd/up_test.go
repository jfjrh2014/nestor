package cmd

import (
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestHasSecrets(t *testing.T) {
	tests := []struct {
		name    string
		base    []config.Mapping
		profile []config.Mapping
		want    bool
	}{
		{
			name: "empty base and profile",
			want: false,
		},
		{
			name: "base only",
			base: []config.Mapping{{Key: "K"}},
			want: true,
		},
		{
			name:    "profile only",
			profile: []config.Mapping{{Key: "K"}},
			want:    true,
		},
		{
			name:    "both present",
			base:    []config.Mapping{{Key: "A"}},
			profile: []config.Mapping{{Key: "B"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasSecrets(tt.base, tt.profile); got != tt.want {
				t.Fatalf("hasSecrets(%v, %v) = %v, want %v", tt.base, tt.profile, got, tt.want)
			}
		})
	}
}

// TestHasSecretsEmptyProviderRegression is the regression test for the
// "no secrets declared" guard surviving in `nestor up` after session #34 fixed
// it in secrets inject/check and doctor. A config with mappings but no provider
// line must still report "has secrets" — the provider literal must never be
// used as a proxy for "has work to do", because NewProvider("") returns the env
// default.
func TestHasSecretsEmptyProviderRegression(t *testing.T) {
	// A config with mappings and an empty provider string. The helper does not
	// receive the provider at all — by design, since the provider literal is
	// not part of the "are there secrets?" decision.
	cfgMappings := []config.Mapping{{Key: "TOKEN"}}

	if !hasSecrets(cfgMappings, nil) {
		t.Fatal("regression: empty-provider config with mappings must still report hasSecrets=true")
	}
}


func TestSnapshotDestPaths(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		profile string
		want    []string
	}{
		{
			name: "nil if no base or profile templates",
			cfg: &config.Config{
				Version: 1,
				Profiles: map[string]config.Profile{},
			},
			profile: "",
			want:    nil,
		},
		{
			name: "only base templates",
			cfg: &config.Config{
				Version: 1,
				Dotfiles: config.Dotfiles{
					Templates: []config.Template{
						{Src: "a.tmpl", Dest: "~/.a"},
						{Src: "b.tmpl", Dest: "~/.b"},
					},
				},
				Profiles: map[string]config.Profile{},
			},
			profile: "",
			want:    []string{"~/.a", "~/.b"},
		},
		{
			name: "profile dotfiles included when profile selected",
			cfg: &config.Config{
				Version: 1,
				Dotfiles: config.Dotfiles{
					Templates: []config.Template{
						{Src: "a.tmpl", Dest: "~/.a"},
					},
				},
				Profiles: map[string]config.Profile{
					"work": {
						Dotfiles: []config.Template{
							{Src: "work-a.tmpl", Dest: "~/.work-a"},
						},
					},
				},
			},
			profile: "work",
			want:    []string{"~/.a", "~/.work-a"},
		},
		{
			name: "profile only (no base) included",
			cfg: &config.Config{
				Version: 1,
				Profiles: map[string]config.Profile{
					"work": {
						Dotfiles: []config.Template{
							{Src: "work-a.tmpl", Dest: "~/.work-a"},
						},
					},
				},
			},
			profile: "work",
			want:    []string{"~/.work-a"},
		},
		{
			name: "dup dest across base and profile deduped",
			cfg: &config.Config{
				Version: 1,
				Dotfiles: config.Dotfiles{
					Templates: []config.Template{
						{Src: "a.tmpl", Dest: "~/.a"},
					},
				},
				Profiles: map[string]config.Profile{
					"work": {
						Dotfiles: []config.Template{
							{Src: "a-overide.tmpl", Dest: "~/.a"},
						},
					},
				},
			},
			profile: "work",
			want:    []string{"~/.a"},
		},
		{
			name: "unknown profile name returns only base templates",
			cfg: &config.Config{
				Version: 1,
				Dotfiles: config.Dotfiles{
					Templates: []config.Template{
						{Src: "a.tmpl", Dest: "~/.a"},
					},
				},
				Profiles: map[string]config.Profile{
					"work": {
						Dotfiles: []config.Template{
							{Src: "work-a.tmpl", Dest: "~/.work-a"},
						},
					},
				},
			},
			profile: "nonexistent",
			want:    []string{"~/.a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotDestPaths(tt.cfg, tt.profile)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
