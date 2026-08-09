package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/importer"
	"github.com/spf13/cobra"
)

var importDryRun bool

var importCmd = &cobra.Command{
	Use:   "import [chezmoi|yadm|brewfile] [path]",
	Short: "Import from an existing dotfile manager",
	Long: `Import packages and dotfiles from chezmoi, yadm, or a Brewfile.

If no source is given, nestor auto-detects which tool you use.
An optional path overrides the default location (chezmoi source dir,
Brewfile path). yadm has no configurable source.

Examples:
  nestor import                 # auto-detect
  nestor import chezmoi        # from default chezmoi source dir
  nestor import yadm           # from yadm list
  nestor import brewfile       # from Brewfile in CWD
  nestor import brewfile ~/Dotfiles/Brewfile
  nestor import chezmoi ~/.config/chezmoi-alt
  nestor import --dry-run      # preview what would be imported`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		srcPath := ""
		if len(args) > 0 {
			name = args[0]
		}
		if len(args) > 1 {
			srcPath = args[1]
		}
		return runImport(name, srcPath, os.Stdout)
	},
}

func init() {
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "preview without writing")
	rootCmd.AddCommand(importCmd)
}

func runImport(name, srcPath string, w io.Writer) error {
	nestorPath := configPath()
	cfg, err := config.Load(nestorPath)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	imp, err := resolveImporter(name, srcPath)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	res, err := imp.Import()
	if err != nil {
		return fmt.Errorf("import from %s: %w", imp.Name(), err)
	}

	// Show what we found
	fmt.Fprintf(w, "nestor: source = %s (%s)\n", imp.Name(), res.Source)
	if len(res.Packages) > 0 {
		fmt.Fprintf(w, "  packages found: %d\n", len(res.Packages))
		for _, p := range res.Packages {
			fmt.Fprintf(w, "    + %s\n", p)
		}
	}
	if len(res.Dotfiles) > 0 {
		fmt.Fprintf(w, "  dotfiles found: %d\n", len(res.Dotfiles))
		for _, d := range res.Dotfiles {
			fmt.Fprintf(w, "    + %s\n", d.Dest)
		}
	}
	if res.Skipped > 0 {
		fmt.Fprintf(w, "  skipped: %d (unsupported entries)\n", res.Skipped)
	}

	if importDryRun {
		fmt.Fprintln(w, "  (dry-run, nothing written)")
		return nil
	}

	// Merge into config and write
	added := importer.MergeResult(cfg, res)
	if added == 0 {
		fmt.Fprintln(w, "nestor: nothing new to add (all items already in config)")
		return nil
	}

	if err := writeConfig(nestorPath, cfg); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	fmt.Fprintf(w, "nestor: imported %d new items into %s\n", added, nestorPath)
	return nil
}

// resolveImporter picks the importer for a named source, applying an optional
// path override. Extracted from runImport so the plumbing (name + path →
// importer) can be unit-tested without invoking a real import or touching disk
// beyond what the importer constructors already do.
func resolveImporter(name, srcPath string) (importer.Importer, error) {
	switch name {
	case "", "auto":
		// auto-detect ignores an explicit path — there is no single source to
		// point it at without knowing which tool was detected. The user would
		// pass an explicit source name to use a path.
		if srcPath != "" {
			return nil, fmt.Errorf("--path %q requires an explicit source (chezmoi or brewfile), not auto", srcPath)
		}
		return importer.Auto()
	case "chezmoi":
		return importer.NewChezmoi(srcPath)
	case "yadm":
		if srcPath != "" {
			return nil, fmt.Errorf("yadm does not accept a path override (it reads `yadm list -a`)")
		}
		return importer.NewYadm()
	case "brewfile", "brew":
		return importer.NewBrewfile(srcPath)
	default:
		return nil, fmt.Errorf("unknown import source %q (use chezmoi, yadm, or brewfile)", name)
	}
}
