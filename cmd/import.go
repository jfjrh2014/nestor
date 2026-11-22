package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/fsutil"
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
	prevTemplates := len(cfg.Dotfiles.Templates)
	added, mergeErr := importer.MergeResult(cfg, res)
	if mergeErr != nil {
		return fmt.Errorf("import: %w", mergeErr)
	}
	if added == 0 {
		fmt.Fprintln(w, "nestor: nothing new to add (all items already in config)")
		return nil
	}

	// Validate the merged config before writing it. Importers build dests
	// straight from the foreign tool's file list (chezmoi decodes its tree
	// verbatim), so a chezmoi dir managing nestor's own config imports a
	// template whose dest is ~/.config/nestor/nestor.yml - the next config
	// load refuses the file and every nestor command bricks (probed:
	// runImport returned nil and the dest landed in nestor.yml). Same
	// fossilizer shape as the 'nestor add' prompts: refuse at the write
	// boundary instead of recording a mapping that corrupts every later run.
	// On refusal the config is left byte-for-byte as it was.
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("import would produce an invalid config - nothing written: %w", err)
	}

	// Materialize the new templates before the config records them. Import
	// used to write src entries and copy nothing, so every imported dotfile
	// was src-missing from the first 'nestor up' on, and 'diff' told the user
	// to run the very 'up' that cannot fix it (probed: exit 0, empty source
	// dir). Same contract as 'add dotfile' and sync capture. On failure the
	// config is left byte-for-byte as it was.
	if err := materializeImported(cfg, cfg.Dotfiles.Templates[prevTemplates:], res.Origins, w); err != nil {
		return fmt.Errorf("import: %w", err)
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

// materializeImported copies each newly merged template's origin file into
// the dotfiles source dir. Every origin is checked before anything is copied,
// so an unreadable origin refuses the import with nothing written. A src name
// already present on disk is an orphan of unknown content (the dests are new
// to the config), so the template moves to the first free <base>-N.tmpl
// instead of adopting it — the rule 'add dotfile' uses.
func materializeImported(cfg *config.Config, added []config.Template, origins map[string]string, w io.Writer) error {
	if len(added) == 0 {
		return nil
	}
	for _, t := range added {
		origin, ok := origins[t.Dest]
		if !ok {
			return fmt.Errorf("no source file recorded for %s - nothing written", t.Dest)
		}
		info, err := os.Stat(origin)
		if err != nil {
			return fmt.Errorf("reading %s for %s - nothing written: %w", origin, t.Dest, err)
		}
		if info.IsDir() {
			return fmt.Errorf("%s for %s is a directory - nothing written", origin, t.Dest)
		}
	}

	sourceDir := cfg.Dotfiles.Source
	if sourceDir == "" {
		sourceDir = config.DefaultDotfilesSource()
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return fmt.Errorf("create source dir: %w", err)
	}

	used := map[string]bool{}
	for _, t := range cfg.Dotfiles.Templates {
		used[filepath.Clean(t.Src)] = true
	}
	for _, p := range cfg.Profiles {
		for _, t := range p.Dotfiles {
			used[filepath.Clean(t.Src)] = true
		}
	}

	first := len(cfg.Dotfiles.Templates) - len(added)
	for i := range added {
		t := &cfg.Dotfiles.Templates[first+i]
		tmplPath := filepath.Join(sourceDir, t.Src)
		if _, err := os.Lstat(tmplPath); err == nil {
			for n := 2; ; n++ {
				cand := importer.NumberedSrc(t.Src, n)
				candPath := filepath.Join(sourceDir, cand)
				if used[filepath.Clean(cand)] {
					continue
				}
				if _, err := os.Lstat(candPath); err == nil {
					continue
				}
				fmt.Fprintf(w, "nestor: %s already exists in %s - using %s\n", t.Src, sourceDir, cand)
				t.Src, tmplPath = cand, candPath
				break
			}
		}
		used[filepath.Clean(t.Src)] = true
		if err := os.MkdirAll(filepath.Dir(tmplPath), 0o755); err != nil {
			return fmt.Errorf("create template dir: %w", err)
		}
		if err := fsutil.CopyFileSync(origins[t.Dest], tmplPath); err != nil {
			return fmt.Errorf("copy template for %s: %w", t.Dest, err)
		}
	}
	return nil
}
