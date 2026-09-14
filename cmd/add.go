package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [package|dotfile|secret] <name>",
	Short: "Add a package, dotfile, or secret to nestor.yml",
	Long: `Interactively add items to your nestor.yml config.

Examples:
  nestor add package ripgrep
  nestor add dotfile ~/.bashrc
  nestor add secret github_token`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAdd(args[0], args[1])
	},
}

func init() {
	addCmd.Flags().StringVarP(&addProfileFlag, "profile", "p", "", "add to a named profile instead of the common/base sections")
	rootCmd.AddCommand(addCmd)
}

// addProfileFlag names the profile that 'nestor add --profile' targets.
var addProfileFlag string

func runAdd(kind, name string) error {
	return runAddIO(kind, name, addProfileFlag, os.Stdin, os.Stdout)
}

func runAddIO(kind, name, profileName string, in io.Reader, out io.Writer) error {
	switch kind {
	case "package", "pkg":
		return addPackage(name, profileName, out)
	case "dotfile", "dot":
		return addDotfile(name, profileName, out)
	case "secret":
		return addSecret(name, profileName, in, out)
	default:
		return fmt.Errorf("unknown type %q — use package, dotfile, or secret", kind)
	}
}

// resolveAddTarget loads the config for an add operation and returns it with
// the validated target profile name ("" = base sections). An unknown profile
// is a hard error before anything is written: profile targeting steers items
// into profiles.<name>, which only exists if the user declared it.
func resolveAddTarget(profileName string) (*config.Config, string, error) {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return nil, "", fmt.Errorf("add: %w", err)
	}
	if profileName != "" && !cfg.ValidProfile(profileName) {
		return nil, "", fmt.Errorf("add: unknown profile %q — define profiles.%s in %s first", profileName, profileName, path)
	}
	return cfg, profileName, nil
}

func addPackage(name, profileName string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("add package: name is required")
	}
	path := configPath()
	cfg, profileName, err := resolveAddTarget(profileName)
	if err != nil {
		return fmt.Errorf("add package: %w", err)
	}

	if profileName != "" {
		prof := cfg.Profiles[profileName]
		for _, p := range prof.Packages {
			if p == name {
				fmt.Fprintf(out, "nestor: %s already in profile %s packages\n", name, profileName)
				return nil
			}
		}
		prof.Packages = append(prof.Packages, name)
		cfg.Profiles[profileName] = prof
		if err := writeConfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "nestor: added package %q to profile %s\n", name, profileName)
		return nil
	}

	// Check if already declared
	for _, p := range cfg.Packages.Common {
		if p == name {
			fmt.Fprintf(out, "nestor: %s already in common packages\n", name)
			return nil
		}
	}

	cfg.Packages.Common = append(cfg.Packages.Common, name)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "nestor: added package %q to common list\n", name)
	return nil
}

func addDotfile(name, profileName string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("add dotfile: name is required")
	}
	path := configPath()
	cfg, profileName, err := resolveAddTarget(profileName)
	if err != nil {
		return fmt.Errorf("add dotfile: %w", err)
	}

	// Resolve the path
	absPath := name
	home, _ := os.UserHomeDir()
	if name[0] == '~' {
		absPath = filepath.Join(home, name[1:])
	} else if !filepath.IsAbs(name) {
		absPath, _ = filepath.Abs(name)
	}

	// Figure out filename for src
	base := filepath.Base(absPath)
	srcName := base + ".tmpl"

	// Check for duplicate destination — validate() rejects dup dests, so
	// writing one here would brick every subsequent config load.
	if profileName != "" {
		prof := cfg.Profiles[profileName]
		for _, t := range prof.Dotfiles {
			if t.Dest == name {
				fmt.Fprintf(out, "nestor: dotfile %s already in profile %s (src: %s)\n", name, profileName, t.Src)
				return nil
			}
		}
		prof.Dotfiles = append(prof.Dotfiles, config.Template{
			Src:  srcName,
			Dest: name, // keep original (with ~ if provided)
		})
		cfg.Profiles[profileName] = prof
		if err := writeConfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "nestor: added dotfile %s to profile %s → src: %s\n", name, profileName, srcName)
		return nil
	}
	for _, t := range cfg.Dotfiles.Templates {
		if t.Dest == name {
			fmt.Fprintf(out, "nestor: dotfile %s already in config (src: %s)\n", name, t.Src)
			return nil
		}
	}

	template := config.Template{
		Src:  srcName,
		Dest: name, // keep original (with ~ if provided)
	}

	cfg.Dotfiles.Templates = append(cfg.Dotfiles.Templates, template)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "nestor: added dotfile %s → src: %s\n", name, srcName)
	return nil
}

func addSecret(name, profileName string, in io.Reader, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("add secret: name is required")
	}
	path := configPath()
	cfg, profileName, err := resolveAddTarget(profileName)
	if err != nil {
		return fmt.Errorf("add secret: %w", err)
	}

	// Default to env provider if unset
	if cfg.Secrets.Provider == "" {
		cfg.Secrets.Provider = "env"
	}

	// Check for duplicate key — a second entry with the same key is almost
	// always a mistake and would cause ResolveAll to overwrite silently.
	if profileName != "" {
		prof := cfg.Profiles[profileName]
		for _, m := range prof.SecretMappings {
			if m.Key == name {
				fmt.Fprintf(out, "nestor: secret %q already in profile %s\n", name, profileName)
				return nil
			}
		}
		dest, pattern := promptInjectTarget(name, in, out)
		inject := map[string]string{}
		if dest != "" && pattern != "" {
			inject[dest] = pattern
		}
		prof.SecretMappings = append(prof.SecretMappings, config.Mapping{Key: name, Inject: inject})
		cfg.Profiles[profileName] = prof
		if err := writeConfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "nestor: added secret %q to profile %s\n", name, profileName)
		return nil
	}
	for _, m := range cfg.Secrets.Mappings {
		if m.Key == name {
			fmt.Fprintf(out, "nestor: secret %q already in config\n", name)
			return nil
		}
	}

	// Prompt for an injection target so the written config passes validate().
	// An empty inject map bricks every other nestor command because Load()
	// runs validate() on every invocation.
	dest, pattern := promptInjectTarget(name, in, out)

	inject := map[string]string{}
	if dest != "" && pattern != "" {
		inject[dest] = pattern
	}

	mapping := config.Mapping{
		Key:    name,
		Inject: inject,
	}

	cfg.Secrets.Mappings = append(cfg.Secrets.Mappings, mapping)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "nestor: added secret %q (provider: %s)\n", name, cfg.Secrets.Provider)
	if len(inject) == 0 {
		fmt.Fprintln(out, "       no injection target set — edit nestor.yml to add one before running 'nestor up'")
	}
	return nil
}

// promptInjectTarget asks the user for a dest path and template pattern for
// the new secret. Both are optional — an empty line skips the target. The
// pattern defaults to "key: {{.<name>}}" (matching the PLAN.md example form)
// when only a dest is given.
func promptInjectTarget(name string, in io.Reader, out io.Writer) (dest, pattern string) {
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "inject into which file? (path, or blank to skip): ")
	if !scanner.Scan() {
		return "", ""
	}
	dest = strings.TrimSpace(scanner.Text())
	if dest == "" {
		return "", ""
	}
	defaultPattern := fmt.Sprintf("%s: {{.%s}}", name, name)
	fmt.Fprintf(out, "template pattern for %s (default %q): ", dest, defaultPattern)
	if !scanner.Scan() {
		return dest, defaultPattern
	}
	pattern = strings.TrimSpace(scanner.Text())
	if pattern == "" {
		pattern = defaultPattern
	}
	return dest, pattern
}

// writeConfig writes the config back to disk preserving YAML formatting.
func writeConfig(path string, cfg *config.Config) error {
	data, err := config.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
