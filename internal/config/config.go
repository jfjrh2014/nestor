package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level nestor.yml
type Config struct {
	Version  int       `yaml:"version"`
	Packages Packages  `yaml:"packages"`
	Dotfiles Dotfiles  `yaml:"dotfiles"`
	Secrets  Secrets   `yaml:"secrets"`
	Shells   Shells    `yaml:"shells"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Packages lists packages by platform
type Packages struct {
	Common []string `yaml:"common"`
	MacOS  []string `yaml:"macos"`
	Linux  []string `yaml:"linux"`
	WSL    []string `yaml:"wsl"`
}

// Dotfiles config
type Dotfiles struct {
	Source    string     `yaml:"source"`
	Strategy  string     `yaml:"strategy"`
	Templates []Template `yaml:"templates"`
}

// Template maps a source template to a destination
type Template struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
}

// Secrets config
type Secrets struct {
	Provider string    `yaml:"provider"`
	Mappings []Mapping `yaml:"mappings"`
}

// Mapping maps a secret key to injection targets
type Mapping struct {
	Key    string            `yaml:"key"`
	Inject map[string]string `yaml:"inject"`
}

// Shells config
type Shells struct {
	Default string   `yaml:"default"`
	Plugins []string `yaml:"plugins"`
}

// Profile defines a named set of extra packages, dotfile variants, and secrets
// that layer on top of the base config when activated.
type Profile struct {
	Packages []string `yaml:"packages"`
}

// ValidProfile returns true if a profile with the given name is defined.
func (c *Config) ValidProfile(name string) bool {
	_, ok := c.Profiles[name]
	return ok
}

// ProfilePackages returns the extra packages for a profile, or empty slice if
// the profile doesn't exist.
func (c *Config) ProfilePackages(name string) []string {
	if name == "" {
		return nil
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil
	}
	return p.Packages
}

// Load reads and parses a nestor.yml file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// expand ~ in paths
	home, _ := os.UserHomeDir()
	cfg.Dotfiles.Source = expandHome(cfg.Dotfiles.Source, home)

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Version == 0 {
		return fmt.Errorf("config version is required")
	}
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (only version 1 is supported)", c.Version)
	}
	if c.Dotfiles.Strategy == "" {
		c.Dotfiles.Strategy = "copy"
	}
	if c.Dotfiles.Strategy != "copy" && c.Dotfiles.Strategy != "symlink" {
		return fmt.Errorf("dotfiles.strategy must be 'copy' or 'symlink', got %q", c.Dotfiles.Strategy)
	}
	return nil
}

// Marshal serializes the config back to YAML bytes.
func Marshal(cfg *Config) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}
	return data, nil
}

func expandHome(path, home string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		return filepath.Join(home, path[1:])
	}
	return path
}
