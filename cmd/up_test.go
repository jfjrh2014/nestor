package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/shell"
	"github.com/jfjrh2014/nestor/internal/ui"
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
				Version:  1,
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

// TestUpShellPluginEntryResolved pins session #74: the rc source block must
// reference an entry file that actually exists in the clone. alias-tips ships
// alias-tips.plugin.zsh (oh-my-zsh convention); the old code wrote
// source <clone>/alias-tips.zsh — a file that does not exist — breaking every
// shell startup after `nestor up`. Runs through the real InstallPlugins with a
// local bare repo so the clone is real, no network needed.
// seedBareRemote creates a local bare git repo whose main branch contains the
// given files — a plugin "remote" the clone path can hit without network.
// The bare repo is created with HEAD at main so clones see the tree (a
// dangling-HEAD bare repo yields an empty worktree, as session #71 learned).
func seedBareRemote(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), repo+".git")
	seed := filepath.Join(t.TempDir(), repo)
	os.MkdirAll(seed, 0o755)
	for name, body := range files {
		os.WriteFile(filepath.Join(seed, name), []byte(body), 0o644)
	}
	for _, a := range [][]string{
		{"init", "-q", "-b", "main", seed},
		{"-C", seed, "add", "."},
		{"-C", seed, "-c", "user.email=t@l", "-c", "user.name=t", "commit", "-qm", "init"},
		{"init", "-q", "--bare", "-b", "main", remote},
		{"-C", seed, "push", "-q", remote, "main"},
	} {
		if out, err := exec.Command("git", a...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
	return remote
}

func TestUpShellPluginEntryResolved(t *testing.T) {
	remote := seedBareRemote(t, "alias-tips", map[string]string{
		"alias-tips.plugin.zsh": "# plugin\n",
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	shell.SetCloneURLFn(func(owner, repo string) string { return remote })
	defer shell.SetCloneURLFn(nil)

	var buf bytes.Buffer
	p := ui.New(&buf)
	cfg := &config.Config{}
	cfg.Shells.Plugins = []string{"djui/alias-tips"}

	configureShell(p, cfg)

	out := buf.String()
	if strings.Contains(out, "no entry file") {
		t.Errorf("alias-tips must resolve via the .plugin.zsh convention, output:\n%s", out)
	}
	if !strings.Contains(out, "shell config updated") {
		t.Fatalf("expected rc write confirmation, output:\n%s", out)
	}

	rcPath := filepath.Join(home, ".bashrc")
	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("rc file not written: %v", err)
	}
	body := string(content)
	pluginsRoot := filepath.Join(home, ".config", "nestor", "plugins")
	want := "source " + filepath.Join(pluginsRoot, "alias-tips", "alias-tips.plugin.zsh")
	if !strings.Contains(body, want) {
		t.Errorf("rc missing resolved entry %q:\n%s", want, body)
	}
	if strings.Contains(body, "alias-tips/alias-tips.zsh") {
		t.Errorf("rc contains the broken non-existent entry:\n%s", body)
	}
}

// TestUpShellPluginEntryUnresolvedWarned pins the other half of session #74:
// a clone with no resolvable entry file must produce a visible warning and
// must NOT put a dead source line into the rc.
func TestUpShellPluginEntryUnresolvedWarned(t *testing.T) {
	remote := seedBareRemote(t, "no-entry", map[string]string{
		"README.md": "# just docs\n",
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	shell.SetCloneURLFn(func(owner, repo string) string { return remote })
	defer shell.SetCloneURLFn(nil)

	var buf bytes.Buffer
	p := ui.New(&buf)
	cfg := &config.Config{}
	cfg.Shells.Plugins = []string{"someone/no-entry"}

	configureShell(p, cfg)

	out := buf.String()
	if !strings.Contains(out, "no entry file found in clone") {
		t.Errorf("expected unresolved warning in output:\n%s", out)
	}

	body, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("rc file not written: %v", err)
	}
	if strings.Contains(string(body), "source ") {
		t.Errorf("dead source line must not land in rc:\n%s", body)
	}
}

// TestProfileDotfileCollisions pins the session #78 helper contract: only
// dests present in BOTH layers are reported, order preserved, no
// false positives from profile-only or base-only dests.
func TestProfileDotfileCollisions(t *testing.T) {
	base := []config.Template{
		{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
		{Src: "gitconfig.tmpl", Dest: "~/.gitconfig"},
	}
	prof := []config.Template{
		{Src: "bashrc.work.tmpl", Dest: "~/.bashrc"}, // collision
		{Src: "kitty.tmpl", Dest: "~/.config/kitty/kitty.conf"},
		{Src: "gitconfig.work.tmpl", Dest: "~/.gitconfig"}, // collision
	}

	got := profileDotfileCollisions(base, prof)
	want := []string{"~/.bashrc", "~/.gitconfig"}
	if len(got) != len(want) {
		t.Fatalf("collisions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("collisions[%d] = %s, want %s", i, got[i], want[i])
		}
	}

	if got := profileDotfileCollisions(base, []config.Template{{Src: "k.tmpl", Dest: "~/.config/kitty/kitty.conf"}}); len(got) != 0 {
		t.Errorf("no-overlap case returned %v, want none", got)
	}
}

// TestProfileSecretCollisions pins the key-collision helper on the secrets
// side: last-write-wins in ResolveAll means a profile key shadowing a base
// key must be reported before inject runs.
func TestProfileSecretCollisions(t *testing.T) {
	base := []config.Mapping{
		{Key: "ghp", Inject: map[string]string{"~/.gitconfig": "{{ .ghp }}"}},
		{Key: "aws", Inject: map[string]string{"~/.aws/credentials": "{{ .aws }}"}},
	}
	prof := []config.Mapping{
		{Key: "ghp", Inject: map[string]string{"~/.workrc": "{{ .ghp }}"}}, // collision
		{Key: "workonly", Inject: map[string]string{"~/.workrc": "{{ .workonly }}"}},
	}

	got := profileSecretCollisions(base, prof)
	if len(got) != 1 || got[0] != "ghp" {
		t.Errorf("collisions = %v, want [ghp]", got)
	}

	if got := profileSecretCollisions(base, []config.Mapping{{Key: "workonly", Inject: map[string]string{"~/.w": "x"}}}); len(got) != 0 {
		t.Errorf("no-overlap case returned %v, want none", got)
	}
}
