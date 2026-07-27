package cmd

import (
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

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
