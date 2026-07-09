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
	rootCmd.AddCommand(addCmd)
}

func runAdd(kind, name string) error {
	return runAddIO(kind, name, os.Stdin, os.Stdout)
}

func runAddIO(kind, name string, in io.Reader, out io.Writer) error {
	switch kind {
	case "package", "pkg":
		return addPackage(name, out)
	case "dotfile", "dot":
		return addDotfile(name, out)
	case "secret":
		return addSecret(name, in, out)
	default:
		return fmt.Errorf("unknown type %q — use package, dotfile, or secret", kind)
	}
}

func addPackage(name string, out io.Writer) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("add package: %w", err)
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

func addDotfile(name string, out io.Writer) error {
	path := configPath()
	cfg, err := config.Load(path)
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

func addSecret(name string, in io.Reader, out io.Writer) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("add secret: %w", err)
	}

	// Default to env provider if unset
	if cfg.Secrets.Provider == "" {
		cfg.Secrets.Provider = "env"
	}

	// Check for duplicate key — a second entry with the same key is almost
	// always a mistake and would cause ResolveAll to overwrite silently.
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
