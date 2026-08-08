package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/spf13/cobra"
)

func init() {
	editCmd := &cobra.Command{
		Use:   "edit <template-src>",
		Short: "Edit a dotfile template and preview the rendered output",
		Long: `Open a dotfile template in $EDITOR, then show a preview of the rendered output.

If the file doesn't exist yet, a new empty template is created in the configured
dotfiles source directory.

Examples:
  nestor edit gitconfig.tmpl
  nestor edit tmux.conf.tmpl`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(args[0], cmd.OutOrStdout())
		},
	}
	rootCmd.AddCommand(editCmd)
}

func runEdit(name string, w io.Writer) error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}

	srcDir := cfg.Dotfiles.Source
	if srcDir == "" {
		home, _ := os.UserHomeDir()
		srcDir = filepath.Join(home, ".config", "nestor", "dotfiles")
	}

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("create source dir: %w", err)
	}

	srcPath := filepath.Join(srcDir, name)

	// Create empty file if it doesn't exist so the editor has something to open.
	isNew := false
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		if err := os.WriteFile(srcPath, []byte{}, 0o644); err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		isNew = true
	}

	if err := openEditor(srcPath); err != nil {
		return err
	}

	if isNew {
		fmt.Fprintf(w, "nestor: created %s\n", srcPath)
		fmt.Fprintln(w, "       add it to dotfiles.templates in nestor.yml to deploy it")
	}

	// Preview rendered output (only for .tmpl files).
	if filepath.Ext(srcPath) == ".tmpl" {
		fmt.Fprintln(w, "\n--- preview (rendered) ---")
		data, err := dotfiles.Render(srcPath)
		if err != nil {
			fmt.Fprintf(w, "nestor: render error: %v\n", err)
			fmt.Fprintln(w, "(template saved, but has syntax errors — fix and re-run)")
			return nil
		}
		fmt.Fprintln(w, string(data))
		fmt.Fprintln(w, "--- end preview ---")
	}

	return nil
}

func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor (%s): %w", editor, err)
	}
	return nil
}
