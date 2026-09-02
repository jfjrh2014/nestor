package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestValidateConfigVersion(t *testing.T) {
	tests := []struct {
		name    string
		version int
		wantErr bool
	}{
		{"v1 ok", 1, false},
		{"zero fails", 0, true},
		{"v2 fails", 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Version: tt.version, Dotfiles: config.Dotfiles{Strategy: "copy"}}
			r := Validate(cfg, "")
			if tt.wantErr && r.ErrorCount() == 0 {
				t.Errorf("expected error for version %d, got none", tt.version)
			}
			if !tt.wantErr && r.ErrorCount() > 0 {
				t.Errorf("expected no errors for version %d, got %d", tt.version, r.ErrorCount())
			}
		})
	}
}

func TestValidateDotfilesStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		wantErr  bool
	}{
		{"copy", false},
		{"symlink", false},
		{"", false}, // defaults to copy
		{"weird", true},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			cfg := &config.Config{Version: 1, Dotfiles: config.Dotfiles{Strategy: tt.strategy}}
			r := Validate(cfg, "")
			if tt.wantErr && r.ErrorCount() == 0 {
				t.Errorf("expected error for strategy %q", tt.strategy)
			}
			if !tt.wantErr && r.ErrorCount() > 0 {
				t.Errorf("expected no errors for strategy %q, got %d", tt.strategy, r.ErrorCount())
			}
		})
	}
}

func TestValidateDotfilesDuplicateDest(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Strategy: "copy",
			Templates: []config.Template{
				{Src: "a.tmpl", Dest: "~/.bashrc"},
				{Src: "b.tmpl", Dest: "~/.bashrc"},
			},
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() == 0 {
		t.Fatal("expected error for duplicate dest, got none")
	}
	found := false
	for _, f := range r.Findings {
		if f.Category == "dotfiles" && contains(f.Message, "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate dest finding, got: %+v", r.Findings)
	}
}

func TestValidateDotfilesEmptyFields(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Strategy: "copy",
			Templates: []config.Template{
				{Src: "", Dest: "~/.bashrc"},
				{Src: "b.tmpl", Dest: ""},
			},
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() < 2 {
		t.Errorf("expected 2 errors for empty src/dest, got %d", r.ErrorCount())
	}
}

func TestValidateDotfilesSourceExists(t *testing.T) {
	dir := t.TempDir()
	// create a template file
	if err := os.WriteFile(filepath.Join(dir, "exists.tmpl"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Strategy: "copy",
			Templates: []config.Template{
				{Src: "exists.tmpl", Dest: "~/.bashrc"},
				{Src: "missing.tmpl", Dest: "~/.zshrc"},
			},
		},
	}
	r := Validate(cfg, dir)

	// missing template should be a warning, not an error
	foundWarn := false
	for _, f := range r.Findings {
		if f.Category == "dotfiles" && f.Severity == SeverityWarning && contains(f.Message, "missing.tmpl") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning for missing template src, got: %+v", r.Findings)
	}
}

func TestValidatePackagesUnknownManager(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Packages: config.Packages{
			Common: []string{"git", "weirdmgr: foo"},
		},
	}
	r := Validate(cfg, "")
	foundWarn := false
	for _, f := range r.Findings {
		if f.Category == "packages" && contains(f.Message, "weirdmgr") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning for unknown package manager, got: %+v", r.Findings)
	}
}

func TestValidateSecretsNoProvider(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Secrets: config.Secrets{
			Mappings: []config.Mapping{
				{Key: "foo"},
			},
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() == 0 {
		t.Errorf("expected error for mappings without provider")
	}
}

func TestValidateSecretsInvalidProvider(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Secrets: config.Secrets{
			Provider: "nonsense",
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() == 0 {
		t.Errorf("expected error for invalid provider")
	}
}

func TestValidateSecretsValidProvider(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Secrets: config.Secrets{
			Provider: "env",
			Mappings: []config.Mapping{
				{Key: "API_TOKEN", Inject: map[string]string{"~/.env": "TOKEN={{.Key}}"}},
			},
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() > 0 {
		t.Errorf("expected no errors for valid env provider, got: %+v", r.Findings)
	}
}

func TestValidateSecretsEmptyKey(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Secrets: config.Secrets{
			Provider: "env",
			Mappings: []config.Mapping{
				{Key: "", Inject: map[string]string{"~/.env": "X={{.Key}}"}},
			},
		},
	}
	r := Validate(cfg, "")
	if r.ErrorCount() == 0 {
		t.Errorf("expected error for empty secret key")
	}
}

func TestValidateProfiles(t *testing.T) {
	t.Run("empty profile warns", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"empty": {},
			},
		}
		r := Validate(cfg, "")
		found := false
		for _, f := range r.Findings {
			if f.Category == "profiles" && contains(f.Message, "empty") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected warning for empty profile, got: %+v", r.Findings)
		}
	})

	t.Run("valid profiles pass", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"work":     {Packages: []string{"slack"}},
				"personal": {Packages: []string{"discord"}},
			},
		}
		r := Validate(cfg, "")
		if r.ErrorCount() > 0 {
			t.Errorf("expected no errors, got: %+v", r.Findings)
		}
	})
}

func TestReportHasErrors(t *testing.T) {
	r := Report{Findings: []Finding{{SeverityWarning, "x", "y"}}}
	if r.HasErrors() {
		t.Error("warnings-only report should not have errors")
	}

	r.Findings = append(r.Findings, Finding{SeverityError, "x", "z"})
	if !r.HasErrors() {
		t.Error("report with error should report HasErrors")
	}
}

func TestReportCounts(t *testing.T) {
	r := Report{Findings: []Finding{
		{SeverityError, "a", "1"},
		{SeverityError, "b", "2"},
		{SeverityWarning, "c", "3"},
	}}
	if r.ErrorCount() != 2 {
		t.Errorf("ErrorCount = %d, want 2", r.ErrorCount())
	}
	if r.WarnCount() != 1 {
		t.Errorf("WarnCount = %d, want 1", r.WarnCount())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateProfilesDotfiles(t *testing.T) {
	t.Run("profile empty fields flagged", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"work": {Dotfiles: []config.Template{
					{Src: "", Dest: "~/.zshrc"},
					{Src: "f.tmpl", Dest: ""},
				}},
			},
		}
		r := Validate(cfg, "")
		found := 0
		for _, f := range r.Findings {
			if f.Category == "profile work" && strings.Contains(f.Message, "empty") {
				found++
			}
		}
		if found < 2 {
			t.Errorf("expected 2 empty-field findings, got %d: %+v", found, r.Findings)
		}
	})

	t.Run("profile duplicate dest among profile templates errors", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"work": {Dotfiles: []config.Template{
					{Src: "a.tmpl", Dest: "~/.gitconfig"},
					{Src: "b.tmpl", Dest: "~/.gitconfig"},
				}},
			},
		}
		r := Validate(cfg, "")
		found := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && f.Severity == SeverityError && strings.Contains(f.Message, "duplicate dest") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected duplicate dest error, got: %+v", r.Findings)
		}
	})

	t.Run("profile dest overriding base warns not errors", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Dotfiles: config.Dotfiles{
				Templates: []config.Template{{Src: "git.tmpl", Dest: "~/.gitconfig"}},
			},
			Profiles: map[string]config.Profile{
				"work": {Dotfiles: []config.Template{
					{Src: "git-work.tmpl", Dest: "~/.gitconfig"},
				}},
			},
		}
		r := Validate(cfg, "")
		sawWarn := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && strings.Contains(f.Message, "~/.gitconfig") {
				if f.Severity == SeverityError {
					t.Errorf("base+profile dest share must warn, not error: %+v", r.Findings)
				}
				if f.Severity == SeverityWarning && strings.Contains(f.Message, "overrides") {
					sawWarn = true
				}
			}
		}
		if !sawWarn {
			t.Errorf("expected override warning for ~/.gitconfig, got: %+v", r.Findings)
		}
	})

	t.Run("profile src existence checked when source given", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"work": {Dotfiles: []config.Template{
					{Src: "missing.tmpl", Dest: "~/.zshrc"},
				}},
			},
		}
		r := Validate(cfg, t.TempDir()) // empty dir: src will not exist
		found := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && f.Severity == SeverityWarning && strings.Contains(f.Message, "missing.tmpl") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected src-not-found warning, got: %+v", r.Findings)
		}
	})

	t.Run("renderable tmpl src passes silently", func(t *testing.T) {
		srcDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(srcDir, "ok.tmpl"), []byte("x={{env \"HOME\"}}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{
			Version: 1,
			Dotfiles: config.Dotfiles{
				Source:    srcDir,
				Templates: []config.Template{{Src: "ok.tmpl", Dest: "~/.x"}},
			},
		}
		r := Validate(cfg, srcDir)
		for _, f := range r.Findings {
			if strings.Contains(f.Message, "ok.tmpl") {
				t.Errorf("unexpected finding for renderable template: %+v", f)
			}
		}
	})

	t.Run("tmpl src with undefined key errors", func(t *testing.T) {
		srcDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(srcDir, "bad.tmpl"), []byte("v={{.UNDEFINED}}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{
			Version: 1,
			Dotfiles: config.Dotfiles{
				Source:    srcDir,
				Templates: []config.Template{{Src: "bad.tmpl", Dest: "~/.x"}},
			},
		}
		r := Validate(cfg, srcDir)
		found := false
		for _, f := range r.Findings {
			if f.Category == "dotfiles" && f.Severity == SeverityError && strings.Contains(f.Message, "fails to render") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected fails-to-render error, got: %+v", r.Findings)
		}
	})
}

func TestValidateProfilesSecrets(t *testing.T) {
	t.Run("profile mappings with no provider error", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Profiles: map[string]config.Profile{
				"work": {SecretMappings: []config.Mapping{{Key: "API_KEY", Inject: map[string]string{"HOME": "~/.env"}}}},
			},
		}
		r := Validate(cfg, "")
		found := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && f.Severity == SeverityError && strings.Contains(f.Message, "no provider set") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected no-provider error for profile mappings, got: %+v", r.Findings)
		}
	})

	t.Run("profile mapping empty key flagged", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Secrets: config.Secrets{Provider: "env"},
			Profiles: map[string]config.Profile{
				"work": {SecretMappings: []config.Mapping{{Key: "", Inject: map[string]string{"HOME": "~/.env"}}}},
			},
		}
		r := Validate(cfg, "")
		found := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && f.Severity == SeverityError && strings.Contains(f.Message, "empty key") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected empty-key error, got: %+v", r.Findings)
		}
	})

	t.Run("profile mapping no inject targets warns", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			Secrets: config.Secrets{Provider: "env"},
			Profiles: map[string]config.Profile{
				"work": {SecretMappings: []config.Mapping{{Key: "API_KEY", Inject: map[string]string{}}}},
			},
		}
		r := Validate(cfg, "")
		found := false
		for _, f := range r.Findings {
			if f.Category == "profile work" && f.Severity == SeverityWarning && strings.Contains(f.Message, "no inject targets") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected no-inject-targets warning, got: %+v", r.Findings)
		}
	})
}
