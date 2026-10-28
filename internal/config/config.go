package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jfjrh2014/nestor/internal/pathutil"
	"gopkg.in/yaml.v3"
)

// Config represents the top-level nestor.yml
type Config struct {
	Version  int                `yaml:"version"`
	Packages Packages           `yaml:"packages"`
	Dotfiles Dotfiles           `yaml:"dotfiles"`
	Secrets  Secrets            `yaml:"secrets"`
	Shells   Shells             `yaml:"shells"`
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
	Packages       []string   `yaml:"packages"`
	Dotfiles       []Template `yaml:"dotfiles"`
	SecretMappings []Mapping  `yaml:"secrets"`
}

// DefaultDotfilesSource returns the source directory used when the config
// does not declare dotfiles.source: ~/.config/nestor/dotfiles. Every command
// that reads templates resolves the source through this helper — the fallback
// used to be hand-copied per command with two spellings that disagreed when
// HOME was unset (filepath.Join built a relative .config/... while
// fmt.Sprintf built an absolute /.config/...).
func DefaultDotfilesSource() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nestor", "dotfiles")
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

// ProfileDotfiles returns extra dotfile templates for a profile.
func (c *Config) ProfileDotfiles(name string) []Template {
	if name == "" {
		return nil
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil
	}
	return p.Dotfiles
}

// ProfileSecretMappings returns extra secret mappings for a profile.
func (c *Config) ProfileSecretMappings(name string) []Mapping {
	if name == "" {
		return nil
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil
	}
	return p.SecretMappings
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
	cfg.Dotfiles.Source = pathutil.ExpandHome(cfg.Dotfiles.Source, home)

	return &cfg, nil
}

// Validate checks the config for required fields and applies defaults.
// Exported so other packages (e.g. restore) can validate parsed configs
// without writing to disk.
func (c *Config) Validate() error {
	return c.validate()
}

// validProviders is the set of acceptable secrets.provider values.
var validProviders = map[string]bool{
	"":          true, // default env
	"env":       true,
	"1password": true,
	"bitwarden": true,
	"vault":     true,
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

	// Self-inject guard needs the real home to resolve "~" forms against.
	home, _ := os.UserHomeDir()
	if c.Dotfiles.Strategy != "copy" && c.Dotfiles.Strategy != "symlink" {
		return fmt.Errorf("dotfiles.strategy must be 'copy' or 'symlink', got %q", c.Dotfiles.Strategy)
	}

	// Validate secret provider is known (only when provider or mappings are declared).
	if !validProviders[c.Secrets.Provider] {
		return fmt.Errorf("secrets.provider %q is not valid (allowed: env, 1password, bitwarden, vault)", c.Secrets.Provider)
	}

	// Validate templates: src and dest must be non-empty, dest must not use
	// the "~user/..." tilde form (it would silently deploy into the current
	// user's home instead of the named user's).
	for i, t := range c.Dotfiles.Templates {
		if t.Src == "" {
			return fmt.Errorf("dotfiles.templates[%d]: src is empty", i)
		}
		if t.Dest == "" {
			return fmt.Errorf("dotfiles.templates[%d]: dest is empty (src=%q)", i, t.Src)
		}
		if pathutil.IsOtherUserTilde(t.Dest) {
			return fmt.Errorf("dotfiles.templates[%d]: dest %q uses the ~user/... form, which nestor does not expand (it would deploy into your own home)", i, t.Dest)
		}
		if err := checkTemplateDestAgainstInject(t.Dest, fmt.Sprintf("dotfiles.templates[%d]", i), c, home); err != nil {
			return err
		}
	}

	// Duplicate destinations cause two templates to overwrite each other.
	// Compared in the expanded shape: "~/.bashrc" and the absolute form are
	// the same file, and letting both spellings coexist means two templates
	// deploy to one dest last-wins (and the first deploy re-spells the other
	// entry's dest through the tilde boundary).
	dup := make(map[string]bool)
	for _, t := range c.Dotfiles.Templates {
		key := pathutil.ExpandHome(t.Dest, home)
		if dup[key] {
			return fmt.Errorf("dotfiles.templates: duplicate dest %q (same file as another entry once expanded)", t.Dest)
		}
		dup[key] = true
	}

	// Validate secret mappings: key non-empty, inject targets non-empty and
	// not using the "~user/..." tilde form.
	for i, m := range c.Secrets.Mappings {
		if m.Key == "" {
			return fmt.Errorf("secrets.mappings[%d]: key is empty", i)
		}
		if len(m.Inject) == 0 {
			return fmt.Errorf("secrets.mappings[%d]: inject is empty (key=%q)", i, m.Key)
		}
		for dest := range m.Inject {
			if pathutil.IsOtherUserTilde(dest) {
				return fmt.Errorf("secrets.mappings[%d]: inject target %q uses the ~user/... form, which nestor does not expand (it would inject into your own home)", i, dest)
			}
			if pathutil.IsSelfInject(dest, home) {
				return fmt.Errorf("secrets.mappings[%d]: inject target %q is nestor's own config file — injecting there corrupts nestor.yml on every 'nestor up'", i, dest)
			}
			if pathutil.IsManagedDotfileDest(dest, templateDests(c.Dotfiles.Templates), home) {
				return fmt.Errorf("secrets.mappings[%d]: inject target %q is a managed dotfile destination — deploying it overwrites the injected value", i, dest)
			}
		}
	}

	// Validate profiles: same field checks applied to nested dotfiles/secrets.
	for name, prof := range c.Profiles {
		pdup := make(map[string]bool)
		for i, t := range prof.Dotfiles {
			if t.Src == "" {
				return fmt.Errorf("profiles.%s.dotfiles[%d]: src is empty", name, i)
			}
			if t.Dest == "" {
				return fmt.Errorf("profiles.%s.dotfiles[%d]: dest is empty (src=%q)", name, i, t.Src)
			}
			if pathutil.IsOtherUserTilde(t.Dest) {
				return fmt.Errorf("profiles.%s.dotfiles[%d]: dest %q uses the ~user/... form, which nestor does not expand (it would deploy into your own home)", name, i, t.Dest)
			}
			pkey := pathutil.ExpandHome(t.Dest, home)
			if pdup[pkey] {
				return fmt.Errorf("profiles.%s.dotfiles: duplicate dest %q (same file as another entry once expanded)", name, t.Dest)
			}
			pdup[pkey] = true
			if err := checkTemplateDestAgainstInject(t.Dest, fmt.Sprintf("profiles.%s.dotfiles[%d]", name, i), c, home); err != nil {
				return err
			}
		}
		for i, m := range prof.SecretMappings {
			if m.Key == "" {
				return fmt.Errorf("profiles.%s.secrets[%d]: key is empty", name, i)
			}
			if len(m.Inject) == 0 {
				return fmt.Errorf("profiles.%s.secrets[%d]: inject is empty (key=%q)", name, i, m.Key)
			}
			for dest := range m.Inject {
				if pathutil.IsOtherUserTilde(dest) {
					return fmt.Errorf("profiles.%s.secrets[%d]: inject target %q uses the ~user/... form, which nestor does not expand (it would inject into your own home)", name, i, dest)
				}
				if pathutil.IsSelfInject(dest, home) {
					return fmt.Errorf("profiles.%s.secrets[%d]: inject target %q is nestor's own config file — injecting there corrupts nestor.yml on every 'nestor up'", name, i, dest)
				}
				if pathutil.IsManagedDotfileDest(dest, templateDests(prof.Dotfiles), home) {
					return fmt.Errorf("profiles.%s.secrets[%d]: inject target %q is a managed dotfile destination — deploying it overwrites the injected value", name, i, dest)
				}
				if pathutil.IsManagedDotfileDest(dest, templateDests(c.Dotfiles.Templates), home) {
					return fmt.Errorf("profiles.%s.secrets[%d]: inject target %q is a base dotfile destination — deploying it overwrites the injected value", name, i, dest)
				}
			}
		}
	}

	return nil
}

// TemplateDests returns every destination a dotfile template may deploy to:
// the base templates plus each profile's layer. Validation, 'nestor add' and
// the deploy path use it to refuse a dotfile whose dest is a declared secret
// inject target — a deploy overwrites the injected value silently, the exact
// corruption the inject-side checks in validate() refuse in the other
// direction.
func (c *Config) TemplateDests() []string {
	dests := make([]string, 0, len(c.Dotfiles.Templates))
	for _, t := range c.Dotfiles.Templates {
		dests = append(dests, t.Dest)
	}
	for _, prof := range c.Profiles {
		for _, t := range prof.Dotfiles {
			dests = append(dests, t.Dest)
		}
	}
	return dests
}

// InjectTargetDests returns every destination a secret may be injected into:
// the base mappings plus each profile's layer. Validation, 'nestor add' and
// the deploy path use it to refuse a dotfile template whose dest is a
// declared inject target — the same corruption, seen from the dotfile side.
func (c *Config) InjectTargetDests() []string {
	dests := make([]string, 0)
	for _, m := range c.Secrets.Mappings {
		for dest := range m.Inject {
			dests = append(dests, dest)
		}
	}
	for _, prof := range c.Profiles {
		for _, m := range prof.SecretMappings {
			for dest := range m.Inject {
				dests = append(dests, dest)
			}
		}
	}
	return dests
}

// checkTemplateDestAgainstInject refuses a dotfile template whose dest is a
// declared secret inject target or nestor's own config file. Deploying the
// template overwrites the injected value silently (templates render with no
// secret data) — the mirror image of the inject-onto-managed-dotfile check
// above, which refuses the same pair from the secret side. nestor.yml as a
// dotfile dest would have every deploy rewrite the config that drives it.
// The inject target list spans base AND profile layers: a profile template
// that collides with a base inject target corrupts on every plain 'nestor up'
// (dotfiles deploy, then base inject targets receive values).
func checkTemplateDestAgainstInject(dest, label string, c *Config, home string) error {
	if pathutil.IsSelfInject(dest, home) {
		return fmt.Errorf("%s: dest %q is nestor's own config file — every deploy would rewrite the config that drives it", label, dest)
	}
	if pathutil.IsManagedDotfileDest(dest, c.InjectTargetDests(), home) {
		return fmt.Errorf("%s: dest %q is a declared secret inject target — deploying the template overwrites the injected value", label, dest)
	}
	return nil
}

// templateDests extracts the destinations of a template list for
// pathutil.IsManagedDotfileDest comparisons.
func templateDests(templates []Template) []string {
	dests := make([]string, len(templates))
	for i, t := range templates {
		dests[i] = t.Dest
	}
	return dests
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
	return pathutil.ExpandHome(path, home)
}
