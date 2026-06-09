package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	switch kind {
	case "package", "pkg":
		return addPackage(name)
	case "dotfile", "dot":
		return addDotfile(name)
	case "secret":
		return addSecret(name)
	default:
		return fmt.Errorf("unknown type %q — use package, dotfile, or secret", kind)
	}
}

func addPackage(name string) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("add package: %w", err)
	}

	// Check if already declared
	for _, p := range cfg.Packages.Common {
		if p == name {
			fmt.Printf("nestor: %s already in common packages\n", name)
			return nil
		}
	}

	cfg.Packages.Common = append(cfg.Packages.Common, name)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Printf("nestor: added package %q to common list\n", name)
	return nil
}

func addDotfile(name string) error {
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

	template := config.Template{
		Src:  srcName,
		Dest: name, // keep original (with ~ if provided)
	}

	cfg.Dotfiles.Templates = append(cfg.Dotfiles.Templates, template)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Printf("nestor: added dotfile %s → src: %s\n", name, srcName)
	return nil
}

func addSecret(name string) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("add secret: %w", err)
	}

	// Default to env provider if unset
	if cfg.Secrets.Provider == "" {
		cfg.Secrets.Provider = "env"
	}

	mapping := config.Mapping{
		Key:    name,
		Inject: map[string]string{},
	}

	cfg.Secrets.Mappings = append(cfg.Secrets.Mappings, mapping)
	if err := writeConfig(path, cfg); err != nil {
		return err
	}

	fmt.Printf("nestor: added secret %q (provider: %s)\n", name, cfg.Secrets.Provider)
	fmt.Println("       configure injection targets in nestor.yml")
	return nil
}

// writeConfig writes the config back to disk preserving YAML formatting.
func writeConfig(path string, cfg *config.Config) error {
	data, err := config.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
