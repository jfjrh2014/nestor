package cmd

import (
	"fmt"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/importer"
	"github.com/spf13/cobra"
)

var importDryRun bool
var importSource string

var importCmd = &cobra.Command{
	Use:   "import [chezmoi|yadm|brewfile]",
	Short: "Import from an existing dotfile manager",
	Long: `Import packages and dotfiles from chezmoi, yadm, or a Brewfile.

If no source is given, nestor auto-detects which tool you use.

Examples:
  nestor import                 # auto-detect
  nestor import chezmoi        # from chezmoi source dir
  nestor import yadm           # from yadm list
  nestor import brewfile       # from Brewfile in CWD
  nestor import brewfile ~/Dotfiles/Brewfile
  nestor import --dry-run      # preview what would be imported`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = importSource
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if name != "" {
			importSource = name
		}
		return runImport(name)
	},
}

func init() {
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "preview without writing")
	rootCmd.AddCommand(importCmd)
}

func runImport(source string) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	var imp importer.Importer
	switch source {
	case "", "auto":
		imp, err = importer.Auto()
	case "chezmoi":
		imp, err = importer.NewChezmoi("")
	case "yadm":
		imp, err = importer.NewYadm()
	case "brewfile", "brew":
		imp, err = importer.NewBrewfile("")
	default:
		return fmt.Errorf("unknown import source %q (use chezmoi, yadm, or brewfile)", source)
	}
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	res, err := imp.Import()
	if err != nil {
		return fmt.Errorf("import from %s: %w", imp.Name(), err)
	}

	// Show what we found
	fmt.Printf("nestor: source = %s (%s)\n", imp.Name(), res.Source)
	if len(res.Packages) > 0 {
		fmt.Printf("  packages found: %d\n", len(res.Packages))
		for _, p := range res.Packages {
			fmt.Printf("    + %s\n", p)
		}
	}
	if len(res.Dotfiles) > 0 {
		fmt.Printf("  dotfiles found: %d\n", len(res.Dotfiles))
		for _, d := range res.Dotfiles {
			fmt.Printf("    + %s\n", d.Dest)
		}
	}
	if res.Skipped > 0 {
		fmt.Printf("  skipped: %d (unsupported entries)\n", res.Skipped)
	}

	if importDryRun {
		fmt.Println("  (dry-run, nothing written)")
		return nil
	}

	// Merge into config and write
	added := importer.MergeResult(cfg, res)
	if added == 0 {
		fmt.Println("nestor: nothing new to add (all items already in config)")
		return nil
	}

	if err := writeConfig(path, cfg); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	fmt.Printf("nestor: imported %d new items into %s\n", added, path)
	return nil
}
