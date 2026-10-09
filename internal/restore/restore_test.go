package restore

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com/nestor.yml", true},
		{"http://example.com/nestor.yml", true},
		{"ftp://example.com/nestor.yml", false},
		{"not-a-url", false},
		{"", false},
		{"https://", false}, // no host
		{"file:///etc/passwd", false},
	}

	for _, tt := range tests {
		err := validateURL(tt.url)
		if tt.valid && err != nil {
			t.Errorf("validateURL(%q): expected valid, got error: %v", tt.url, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateURL(%q): expected error, got nil", tt.url)
		}
	}
}

func TestFetch(t *testing.T) {
	validConfig := `version: 1
packages:
  common:
    - git
dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
shells:
  default: bash
`

	emptyBody := ""

	tests := []struct {
		name       string
		body       string
		status     int
		wantErr    bool
		errContent string
	}{
		{"valid config", validConfig, 200, false, ""},
		{"empty body", emptyBody, 200, true, "empty"},
		{"404", "", 404, true, "404"},
		{"500", "", 500, true, "500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if tt.body != "" {
					fmt.Fprintln(w, tt.body)
				}
			}))
			defer srv.Close()

			_, err := Fetch(srv.URL + "/nestor.yml")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContent)
				}
				if !strings.Contains(err.Error(), tt.errContent) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContent, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			"valid",
			"version: 1\npackages:\n  common:\n    - git\ndotfiles:\n  strategy: copy\n  templates: []\nsecrets:\n  provider: env\n  mappings: []\n",
			false,
		},
		{
			"missing version",
			"packages:\n  common:\n    - git\n",
			true,
		},
		{
			"bad version",
			"version: 2\n",
			true,
		},
		{
			"bad strategy",
			"version: 1\ndotfiles:\n  strategy: magic\n",
			true,
		},
		{
			"malformed yaml",
			"version: 1\n  bad: : :\n",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate([]byte(tt.yaml))
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	t.Run("write new file", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "nestor.yml")

		if err := Write([]byte("version: 1\n"), dest, false); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		data, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "version: 1\n" {
			t.Errorf("unexpected content: %q", data)
		}
	})

	t.Run("refuse overwrite without force", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "nestor.yml")

		// Create existing file
		if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := Write([]byte("new"), dest, false)
		if err == nil {
			t.Fatal("expected overwrite error, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}

		// Verify original content untouched
		data, _ := os.ReadFile(dest)
		if string(data) != "existing" {
			t.Error("original file was modified")
		}
	})

	t.Run("overwrite with force", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "nestor.yml")

		if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := Write([]byte("new content"), dest, true); err != nil {
			t.Fatalf("Write with force failed: %v", err)
		}

		data, _ := os.ReadFile(dest)
		if string(data) != "new content" {
			t.Errorf("expected 'new content', got %q", data)
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "deep", "nested", "dir", "nestor.yml")

		if err := Write([]byte("version: 1\n"), dest, false); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if _, err := os.Stat(dest); err != nil {
			t.Errorf("file not created: %v", err)
		}
	})
}

func TestPreview(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
		Packages: config.Packages{
			Common: []string{"git", "neovim"},
			Linux:  []string{"htop"},
		},
		Dotfiles: config.Dotfiles{
			Strategy: "copy",
			Templates: []config.Template{
				{Src: "gitconfig.tmpl", Dest: "~/.gitconfig"},
			},
		},
		Secrets: config.Secrets{
			Provider: "bitwarden",
			Mappings: []config.Mapping{
				{Key: "github_token"},
			},
		},
		Shells: config.Shells{
			Default: "zsh",
			Plugins: []string{"starship"},
		},
		Profiles: map[string]config.Profile{
			"personal": {Packages: []string{"discord"}},
		},
	}

	out := Preview(cfg)

	checks := []string{
		"version: 1",
		"packages (3)", // 2 common + 1 linux
		"neovim",
		"htop (Linux)",
		"dotfiles (1 templates",
		"gitconfig.tmpl",
		"bitwarden (1 mappings)",
		"shell: zsh (1 plugins)",
		"profiles: 1",
		"personal",
	}

	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("Preview() missing %q in output:\n%s", check, out)
		}
	}
}

func TestCountPackages(t *testing.T) {
	p := config.Packages{
		Common: []string{"a", "b"},
		MacOS:  []string{"c"},
		Linux:  []string{"d", "e"},
		WSL:    []string{"f"},
	}

	if got := countPackages(p); got != 6 {
		t.Errorf("countPackages = %d, want 6", got)
	}
}

func TestValidateURL_Schemes(t *testing.T) {
	// Ensure file:// is rejected — important to prevent local file exfiltration
	err := validateURL("file:///etc/passwd")
	if err == nil {
		t.Error("file:// scheme should be rejected")
	}
	if !strings.Contains(err.Error(), "http") {
		t.Errorf("error should mention http, got: %v", err)
	}
}

// TestWriteDurableOverwrite pins the --force overwrite path: the rewritten
// file must land complete and parseable, and no temp litter may survive.
func TestWriteDurableOverwrite(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "nestor.yml")

	first := []byte("version: 1\n")
	if err := Write(first, dest, false); err != nil {
		t.Fatalf("first write: %v", err)
	}
	updated := []byte("version: 1\npackages:\n  common:\n    - git\n")
	if err := Write(updated, dest, true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(updated) {
		t.Errorf("dest = %q, want %q", got, updated)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".nestor-write-") {
			t.Errorf("temp litter left behind: %s", e.Name())
		}
	}
}

// TestWriteTildeDest pins the documented example `nestor restore --from <url>
// -o ~/.config/nestor/nestor.yml`: the tilde must resolve to the real home.
// Without expansion, os.WriteFile silently creates a literal directory named
// "~" in the current directory (probed: MkdirAll+WriteFile return nil and the
// cwd gains a "~" entry).
func TestWriteTildeDest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	if err := Write([]byte("version: 1\n"), "~/.config/nestor/nestor.yml", false); err != nil {
		t.Fatalf("Write with tilde dest: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".config", "nestor", "nestor.yml"))
	if err != nil {
		t.Fatalf("file not written to the real home: %v", err)
	}
	if string(data) != "version: 1\n" {
		t.Errorf("unexpected content: %q", data)
	}

	// The damage the bug caused: a literal "~" directory in the cwd.
	if _, err := os.Stat("~"); err == nil {
		t.Error(`a literal "~" directory was created in the current directory`)
	}
}

// TestWriteRefusesOtherUserTilde pins the "~user/..." rejection: nestor does
// not expand foreign tildes, and silently re-rooting "~root/.bashrc" at the
// current user's home is exactly the bug class pathutil guards against.
func TestWriteRefusesOtherUserTilde(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	err := Write([]byte("version: 1\n"), "~root/.config/nestor/nestor.yml", false)
	if err == nil {
		t.Fatal("expected error for ~user/... dest, got nil")
	}
	if !strings.Contains(err.Error(), "~user/... form") {
		t.Errorf("error should mention the ~user/... form, got: %v", err)
	}
	// Nothing may land anywhere: neither the real home tree nor a literal
	// "~root" directory in the cwd.
	if _, statErr := os.Stat(filepath.Join(dir, ".config")); statErr == nil {
		t.Error("config tree created in the real home despite refusal")
	}
	if _, statErr := os.Stat("~root"); statErr == nil {
		t.Error(`a literal "~root" directory was created despite refusal`)
	}
}

// TestFetchRefusesOversizedBody pins the LimitReader truncation fix: a body
// larger than maxFetchBytes fills the limit reader completely, so ReadAll
// returns err == nil with the tail silently gone. Fetch must refuse the
// filled buffer instead of handing a truncated config to Validate and Write.
func TestFetchRefusesOversizedBody(t *testing.T) {
	marker := []byte("TRAILING-MARKER")
	oversized := append(bytes.Repeat([]byte("a"), maxFetchBytes), marker...)
	exactly := append(bytes.Repeat([]byte("b"), maxFetchBytes-len(marker)), marker...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(oversized)
	}))
	defer srv.Close()

	_, err := Fetch(srv.URL + "/nestor.yml")
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention the size limit, got: %v", err)
	}

	// A body of exactly maxFetchBytes must still be accepted, whole —
	// the guard keys on the limit being exceeded, not on big bodies in
	// general.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(exactly)
	}))
	defer srv2.Close()

	got, err := Fetch(srv2.URL + "/nestor.yml")
	if err != nil {
		t.Fatalf("exactly-at-limit body should fetch cleanly: %v", err)
	}
	if !bytes.Equal(got, exactly) {
		t.Errorf("exactly-at-limit body must be returned whole (len=%d, want=%d)", len(got), len(exactly))
	}
}
