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
