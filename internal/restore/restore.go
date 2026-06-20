package restore

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jfjrh2014/nestor/internal/config"
	"gopkg.in/yaml.v3"
)

// Fetch downloads a nestor.yml from the given URL and returns the raw bytes.
func Fetch(rawURL string) ([]byte, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d %s", rawURL, resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("fetched config is empty")
	}

	return body, nil
}

// Validate parses the raw bytes to confirm it's a valid nestor config.
// It does not write anything to disk.
func Validate(data []byte) (*config.Config, error) {
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing fetched config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Write saves the fetched config to the given destination path.
// It refuses to overwrite an existing file unless overwrite is true.
func Write(data []byte, dest string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", dest)
		}
	}

	dir := filepath.Dir(dest)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	return nil
}

// Preview returns a human-readable summary of what the config contains,
// so users can review before committing.
func Preview(cfg *config.Config) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("version: %d\n", cfg.Version))

	b.WriteString(fmt.Sprintf("packages (%d):\n", countPackages(cfg.Packages)))
	for _, p := range cfg.Packages.Common {
		b.WriteString(fmt.Sprintf("  - %s\n", p))
	}
	for _, p := range cfg.Packages.MacOS {
		b.WriteString(fmt.Sprintf("  - %s (macOS)\n", p))
	}
	for _, p := range cfg.Packages.Linux {
		b.WriteString(fmt.Sprintf("  - %s (Linux)\n", p))
	}
	for _, p := range cfg.Packages.WSL {
		b.WriteString(fmt.Sprintf("  - %s (WSL)\n", p))
	}

	b.WriteString(fmt.Sprintf("dotfiles (%d templates, strategy: %s)\n", len(cfg.Dotfiles.Templates), cfg.Dotfiles.Strategy))
	for _, t := range cfg.Dotfiles.Templates {
		b.WriteString(fmt.Sprintf("  %s → %s\n", t.Src, t.Dest))
	}

	b.WriteString(fmt.Sprintf("secrets provider: %s (%d mappings)\n", cfg.Secrets.Provider, len(cfg.Secrets.Mappings)))

	if cfg.Shells.Default != "" {
		b.WriteString(fmt.Sprintf("shell: %s (%d plugins)\n", cfg.Shells.Default, len(cfg.Shells.Plugins)))
	}

	if len(cfg.Profiles) > 0 {
		b.WriteString(fmt.Sprintf("profiles: %d\n", len(cfg.Profiles)))
		for name := range cfg.Profiles {
			b.WriteString(fmt.Sprintf("  - %s\n", name))
		}
	}

	return b.String()
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

// countPackages totals all platform package lists.
func countPackages(p config.Packages) int {
	return len(p.Common) + len(p.MacOS) + len(p.Linux) + len(p.WSL)
}
