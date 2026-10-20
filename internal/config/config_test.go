package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	body := `version: 1
packages:
  common: [git]
dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy
secrets:
  provider: env
shells:
  default: zsh
profiles: {}
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}
	if cfg.Dotfiles.Strategy != "copy" {
		t.Errorf("strategy = %q, want copy", cfg.Dotfiles.Strategy)
	}
	if len(cfg.Packages.Common) != 1 || cfg.Packages.Common[0] != "git" {
		t.Errorf("packages.common = %v, want [git]", cfg.Packages.Common)
	}
}

func TestLoadMissingVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("dotfiles: {strategy: copy}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestLoadBadStrategy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 1\ndotfiles: {strategy: teleport}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	original := `version: 1
packages:
  common:
    - git
    - ripgrep
dotfiles:
  source: /tmp/dots
  strategy: copy
secrets:
  provider: env
shells:
  default: zsh
profiles: {}
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Write back and reload
	outPath := filepath.Join(dir, "out.yml")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg2, err := Load(outPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if len(cfg2.Packages.Common) != 2 {
		t.Errorf("round-trip lost packages: got %v", cfg2.Packages.Common)
	}
	if cfg2.Secrets.Provider != "env" {
		t.Errorf("round-trip lost provider: got %q", cfg2.Secrets.Provider)
	}
}

func TestExpandHome(t *testing.T) {
	got := expandHome("~/foo/bar", "/home/marcus")
	want := "/home/marcus/foo/bar"
	if got != want {
		t.Errorf("expandHome = %q, want %q", got, want)
	}
	// empty
	if expandHome("", "/h") != "" {
		t.Error("expected empty to stay empty")
	}
	// no tilde
	if expandHome("/abs/path", "/h") != "/abs/path" {
		t.Error("absolute path should not be expanded")
	}
}

func TestValidProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"personal": {Packages: []string{"discord"}},
			"work":     {Packages: []string{"slack"}},
		},
	}

	if !cfg.ValidProfile("personal") {
		t.Error("expected 'personal' to be valid")
	}
	if !cfg.ValidProfile("work") {
		t.Error("expected 'work' to be valid")
	}
	if cfg.ValidProfile("ghost") {
		t.Error("expected 'ghost' to be invalid")
	}
}

func TestProfilePackages(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"personal": {Packages: []string{"discord", "spotify"}},
			"work":     {Packages: []string{"slack"}},
		},
	}

	got := cfg.ProfilePackages("personal")
	if len(got) != 2 {
		t.Fatalf("personal packages = %d, want 2", len(got))
	}
	if got[0] != "discord" || got[1] != "spotify" {
		t.Errorf("personal packages = %v, want [discord spotify]", got)
	}

	// empty name returns nil
	if cfg.ProfilePackages("") != nil {
		t.Error("empty profile name should return nil")
	}
	// nonexistent returns nil
	if cfg.ProfilePackages("ghost") != nil {
		t.Error("nonexistent profile should return nil")
	}
}

func TestProfileDotfiles(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"work": {
				Dotfiles: []Template{
					{Src: "gitconfig-work.tmpl", Dest: "~/.gitconfig"},
					{Src: "ssh-config-work.tmpl", Dest: "~/.ssh/config"},
				},
			},
		},
	}

	got := cfg.ProfileDotfiles("work")
	if len(got) != 2 {
		t.Fatalf("work dotfiles = %d, want 2", len(got))
	}
	if got[0].Dest != "~/.gitconfig" {
		t.Errorf("first dotfile dest = %q, want ~/.gitconfig", got[0].Dest)
	}

	// empty name returns nil
	if cfg.ProfileDotfiles("") != nil {
		t.Error("empty profile name should return nil")
	}
	// nonexistent returns nil
	if cfg.ProfileDotfiles("ghost") != nil {
		t.Error("nonexistent profile should return nil")
	}
}

func TestProfileSecretMappings(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"work": {
				SecretMappings: []Mapping{
					{Key: "slack_token", Inject: map[string]string{"~/.config/slack/config": "token: {{.slack_token}}"}},
				},
			},
		},
	}

	got := cfg.ProfileSecretMappings("work")
	if len(got) != 1 {
		t.Fatalf("work secrets = %d, want 1", len(got))
	}
	if got[0].Key != "slack_token" {
		t.Errorf("secret key = %q, want slack_token", got[0].Key)
	}

	// empty name returns nil
	if cfg.ProfileSecretMappings("") != nil {
		t.Error("empty profile name should return nil")
	}
	// nonexistent returns nil
	if cfg.ProfileSecretMappings("ghost") != nil {
		t.Error("nonexistent profile should return nil")
	}
}

func TestLoadProfileWithDotfilesAndSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nestor.yml")
	body := `version: 1
packages:
  common: [git]
dotfiles:
  strategy: copy
  templates: []
secrets:
  provider: env
profiles:
  work:
    packages: [slack]
    dotfiles:
      - src: gitconfig-work.tmpl
        dest: ~/.gitconfig
    secrets:
      - key: slack_token
        inject:
          ~/.config/slack/config: "token: {{.slack_token}}"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	prof, ok := cfg.Profiles["work"]
	if !ok {
		t.Fatal("work profile not found")
	}
	if len(prof.Packages) != 1 || prof.Packages[0] != "slack" {
		t.Errorf("packages = %v, want [slack]", prof.Packages)
	}
	if len(prof.Dotfiles) != 1 {
		t.Fatalf("dotfiles = %d, want 1", len(prof.Dotfiles))
	}
	if prof.Dotfiles[0].Dest != "~/.gitconfig" {
		t.Errorf("dotfile dest = %q, want ~/.gitconfig", prof.Dotfiles[0].Dest)
	}
	if len(prof.SecretMappings) != 1 {
		t.Fatalf("secrets = %d, want 1", len(prof.SecretMappings))
	}
	if prof.SecretMappings[0].Key != "slack_token" {
		t.Errorf("secret key = %q, want slack_token", prof.SecretMappings[0].Key)
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	cfg := &Config{Version: 1, Secrets: Secrets{Provider: "keeper"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestValidateRejectsEmptyTemplateSrc(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy:  "copy",
			Templates: []Template{{Src: "", Dest: "~/.bashrc"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty src")
	}
}

func TestValidateRejectsEmptyTemplateDest(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy:  "copy",
			Templates: []Template{{Src: "bashrc.tmpl", Dest: ""}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty dest")
	}
}

func TestValidateRejectsDuplicateDestinations(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy: "copy",
			Templates: []Template{
				{Src: "a.tmpl", Dest: "~/.bashrc"},
				{Src: "b.tmpl", Dest: "~/.bashrc"},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate dest")
	}
}

func TestValidateRejectsSecretMappingEmptyKey(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{
				{Key: "", Inject: map[string]string{"~/.x": "v"}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty secret key")
	}
}

func TestValidateRejectsSecretMappingEmptyInject(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{
				{Key: "token", Inject: map[string]string{}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty inject map")
	}
}

func TestValidateRejectsProfileEmptyTemplateSrc(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Profiles: map[string]Profile{
			"work": {
				Dotfiles: []Template{{Src: "", Dest: "~/.x"}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for profile empty src")
	}
}

func TestValidateRejectsProfileDuplicateDestinations(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Profiles: map[string]Profile{
			"work": {
				Dotfiles: []Template{
					{Src: "a.tmpl", Dest: "~/.gitconfig"},
					{Src: "b.tmpl", Dest: "~/.gitconfig"},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for profile duplicate dest")
	}
}

func TestValidateRejectsProfileSecretMissingKey(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Profiles: map[string]Profile{
			"work": {
				SecretMappings: []Mapping{
					{Key: "", Inject: map[string]string{"~/.x": "v"}},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for profile secret empty key")
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	cfg := &Config{
		Version:  1,
		Packages: Packages{Common: []string{"git"}},
		Dotfiles: Dotfiles{
			Strategy: "copy",
			Templates: []Template{
				{Src: "bashrc.tmpl", Dest: "~/.bashrc"},
			},
		},
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{
				{Key: "token", Inject: map[string]string{"~/.x": "v"}},
			},
		},
		Profiles: map[string]Profile{
			"work": {
				Packages: []string{"slack"},
				Dotfiles: []Template{
					{Src: "work.tmpl", Dest: "~/.gitconfig"},
				},
				SecretMappings: []Mapping{
					{Key: "slack_token", Inject: map[string]string{"~/.slack": "t"}},
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// TestValidateRejectsOtherUserTilde: "~user/..." paths must be rejected loudly
// instead of silently deploying into the current user's home.
func TestValidateRejectsOtherUserTilde(t *testing.T) {
	valid := func() Config {
		return Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Source: "/cfg",
				Templates: []Template{
					{Src: "bashrc.tmpl", Dest: "~/.bashrc"},
				},
			},
		}
	}

	t.Run("base template dest", func(t *testing.T) {
		cfg := valid()
		cfg.Dotfiles.Templates[0].Dest = "~root/.bashrc"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "~user/...") {
			t.Fatalf("want ~user/... error, got %v", err)
		}
	})

	t.Run("base inject target", func(t *testing.T) {
		cfg := valid()
		cfg.Secrets = Secrets{Mappings: []Mapping{
			{Key: "tok", Inject: map[string]string{"~shared/config": "tok={{.tok}}"}},
		}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "~user/...") {
			t.Fatalf("want ~user/... error, got %v", err)
		}
	})

	t.Run("profile template dest", func(t *testing.T) {
		cfg := valid()
		cfg.Profiles = map[string]Profile{"work": {Dotfiles: []Template{
			{Src: "work.tmpl", Dest: "~deploy/.gitconfig"},
		}}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "~user/...") {
			t.Fatalf("want ~user/... error, got %v", err)
		}
	})

	t.Run("profile inject target", func(t *testing.T) {
		cfg := valid()
		cfg.Profiles = map[string]Profile{"work": {SecretMappings: []Mapping{
			{Key: "tok", Inject: map[string]string{"~svc/app.conf": "tok={{.tok}}"}},
		}}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "~user/...") {
			t.Fatalf("want ~user/... error, got %v", err)
		}
	})

	t.Run("own-home forms still accepted", func(t *testing.T) {
		cfg := valid()
		cfg.Dotfiles.Templates[0].Dest = "~deploy/.bashrc" // tilde but not ~user/
		// "~deploy/" starts with ~deploy which IS ~user form; use plain forms:
		cfg.Dotfiles.Templates[0].Dest = "~/.bashrc"
		cfg.Secrets = Secrets{Mappings: []Mapping{
			{Key: "tok", Inject: map[string]string{"~/.x": "v"}},
		}}
		cfg.Profiles = map[string]Profile{"work": {Dotfiles: []Template{
			{Src: "work.tmpl", Dest: "~/.gitconfig"},
		}}}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid config rejected: %v", err)
		}
	})
}

// TestDefaultDotfilesSource pins the session #87 consolidation: the default
// dotfiles source dir used to be hand-copied per command with two spellings
// that disagreed when HOME was unset — filepath.Join built a relative
// ".config/nestor/dotfiles" while fmt.Sprintf built an absolute
// "/.config/nestor/dotfiles". The helper is the single spelling.
func TestDefaultDotfilesSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	got := DefaultDotfilesSource()
	want := filepath.Join(dir, ".config", "nestor", "dotfiles")
	if got != want {
		t.Fatalf("DefaultDotfilesSource() = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("default source %q is not absolute", got)
	}
}

// TestValidateRejectsSelfInjectTarget: injecting a secret INTO nestor.yml
// rewrites the config with a rendered key: value line and bricks the next
// Load. The mapping must be refused at validation, not discovered on the
// first 'nestor up'.
func TestValidateRejectsSelfInjectTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy:  "copy",
			Templates: []Template{},
		},
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/nestor/nestor.yml": "tok: {{.tok}}"}}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for self-inject target, got nil")
	}
	if !strings.Contains(err.Error(), "nestor's own config file") {
		t.Errorf("error should name the config file, got: %v", err)
	}
}

// TestValidateAcceptsSelfInjectLookalike pins the untouched path: a lookalike
// path (.yaml extension, sibling name) is not the config file and stays legal.
func TestValidateAcceptsSelfInjectLookalike(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &Config{
		Version: 1,
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/nestor/nestor.yaml": "tok: {{.tok}}"}}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("lookalike path should pass, got: %v", err)
	}
}

// TestValidateRejectsManagedDotfileInjectTarget: templates render with no
// secret data, so a secret injected into a managed dotfile is overwritten by
// the next deploy. The mapping is refused. Since session #113 both sides of
// the collision are checked and the template loop fires first (templates are
// validated before mappings), so the error names the collision from the
// dotfile side — the same refusal, whichever door the config came in through.
func TestValidateRejectsManagedDotfileInjectTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy:  "copy",
			Templates: []Template{{Src: "bashrc.tmpl", Dest: "~/.bashrc"}},
		},
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.bashrc": "tok: {{.tok}}"}}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for managed-dotfile inject target, got nil")
	}
	if !strings.Contains(err.Error(), "declared secret inject target") {
		t.Errorf("error should name the inject-target collision, got: %v", err)
	}
}

// TestValidateAllowsUnmanagedInjectTarget pins the untouched path: the same
// dest with no managed template pointing at it is still legal.
func TestValidateAllowsUnmanagedInjectTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &Config{
		Version: 1,
		Dotfiles: Dotfiles{
			Strategy:  "copy",
			Templates: []Template{{Src: "zsh.tmpl", Dest: "~/.zshrc"}},
		},
		Secrets: Secrets{
			Provider: "env",
			Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.bashrc": "tok: {{.tok}}"}}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unmanaged dest should pass, got: %v", err)
	}
}

// TestValidateRejectsProfileManagedDotfileInjectTarget: a profile secret
// mapping whose dest collides with the profile's own dotfile is refused too.
func TestValidateRejectsProfileManagedDotfileInjectTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &Config{
		Version: 1,
		Secrets: Secrets{Provider: "env"},
		Profiles: map[string]Profile{
			"work": {
				Dotfiles:       []Template{{Src: "bashrc.tmpl", Dest: "~/.bashrc"}},
				SecretMappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.bashrc": "tok: {{.tok}}"}}},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for profile self-colliding inject target, got nil")
	}
	if !strings.Contains(err.Error(), "declared secret inject target") {
		t.Errorf("error should name the inject-target collision, got: %v", err)
	}
}

// TestValidateRejectsDotfileDestSelfConfig: a dotfile template whose dest is
// nestor's own config file would have every deploy rewrite the config that
// drives it (session #113, mirror of the inject-side self-inject refusal).
func TestValidateRejectsDotfileDestSelfConfig(t *testing.T) {
	t.Run("base template", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Strategy:  "copy",
				Templates: []Template{{Src: "n.tmpl", Dest: "~/.config/nestor/nestor.yml"}},
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for dotfile dest = nestor.yml, got nil")
		}
		if !strings.Contains(err.Error(), "nestor's own config file") {
			t.Errorf("error should name the config file, got: %v", err)
		}
	})
	t.Run("profile template", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Profiles: map[string]Profile{
				"work": {Dotfiles: []Template{{Src: "n.tmpl", Dest: "~/.config/nestor/nestor.yml"}}},
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for profile dotfile dest = nestor.yml, got nil")
		}
		if !strings.Contains(err.Error(), "nestor's own config file") {
			t.Errorf("error should name the config file, got: %v", err)
		}
	})
}

// TestValidateRejectsDotfileOnInjectTarget: a dotfile template whose dest is
// a declared inject target corrupts on every run without a successful inject
// (provider down, restore, snapshot) — the deploy silently overwrites the
// injected value. Refused across layers, both directions.
func TestValidateRejectsDotfileOnInjectTarget(t *testing.T) {
	t.Run("base template vs base inject", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Strategy:  "copy",
				Templates: []Template{{Src: "app.tmpl", Dest: "~/.config/app/conf"}},
			},
			Secrets: Secrets{
				Provider: "env",
				Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/app/conf": "tok: {{.tok}}"}}},
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for template dest on inject target, got nil")
		}
		if !strings.Contains(err.Error(), "declared secret inject target") {
			t.Errorf("error should name the inject-target collision, got: %v", err)
		}
	})
	t.Run("profile template vs base inject", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Strategy: "copy",
			},
			Secrets: Secrets{
				Provider: "env",
				Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/app/conf": "tok: {{.tok}}"}}},
			},
			Profiles: map[string]Profile{
				"work": {Dotfiles: []Template{{Src: "app.tmpl", Dest: "~/.config/app/conf"}}},
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for profile template on base inject target, got nil")
		}
		if !strings.Contains(err.Error(), "declared secret inject target") {
			t.Errorf("error should name the inject-target collision, got: %v", err)
		}
	})
	t.Run("base template vs profile inject", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Strategy:  "copy",
				Templates: []Template{{Src: "app.tmpl", Dest: "~/.config/app/conf"}},
			},
			Profiles: map[string]Profile{
				"work": {SecretMappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/app/conf": "tok: {{.tok}}"}}}},
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for base template on profile inject target, got nil")
		}
		if !strings.Contains(err.Error(), "declared secret inject target") {
			t.Errorf("error should name the inject-target collision, got: %v", err)
		}
	})
	t.Run("disjoint dests still accepted", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		cfg := &Config{
			Version: 1,
			Dotfiles: Dotfiles{
				Strategy:  "copy",
				Templates: []Template{{Src: "bashrc.tmpl", Dest: "~/.bashrc"}},
			},
			Secrets: Secrets{
				Provider: "env",
				Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.config/app/conf": "tok: {{.tok}}"}}},
			},
			Profiles: map[string]Profile{
				"work": {Dotfiles: []Template{{Src: "git.tmpl", Dest: "~/.gitconfig"}}},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("disjoint dests should pass, got: %v", err)
		}
	})
}

// TestTemplateDestHelpers pins the dest-collection helpers the validation and
// 'nestor add' guards share: both layers collected, inject targets from base
// AND profile mappings.
func TestTemplateDestHelpers(t *testing.T) {
	cfg := &Config{
		Dotfiles: Dotfiles{
			Templates: []Template{{Src: "a.tmpl", Dest: "~/.bashrc"}},
		},
		Secrets: Secrets{
			Mappings: []Mapping{{Key: "tok", Inject: map[string]string{"~/.x": "v"}}},
		},
		Profiles: map[string]Profile{
			"work": {
				Dotfiles:       []Template{{Src: "b.tmpl", Dest: "~/.gitconfig"}},
				SecretMappings: []Mapping{{Key: "k2", Inject: map[string]string{"~/.y": "v"}}},
			},
		},
	}
	if got := cfg.TemplateDests(); len(got) != 2 || got[0] != "~/.bashrc" || got[1] != "~/.gitconfig" {
		t.Errorf("TemplateDests() = %v, want both layers", got)
	}
	inj := cfg.InjectTargetDests()
	if len(inj) != 2 {
		t.Fatalf("InjectTargetDests() = %v, want 2 targets", inj)
	}
	for _, want := range []string{"~/.x", "~/.y"} {
		found := false
		for _, d := range inj {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Errorf("InjectTargetDests() missing %q: %v", want, inj)
		}
	}
}
