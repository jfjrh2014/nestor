package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/jfjrh2014/nestor/internal/restore"
	"github.com/spf13/cobra"
)

var (
	restoreForce   bool
	restoreDryRun  bool
	restoreOutput  string
)

var restoreCmd = &cobra.Command{
	Use:   "restore --from <url>",
	Short: "Pull a nestor.yml from a URL — perfect for team onboarding",
	Long: `restore fetches a nestor.yml config from an HTTP(S) URL, validates it,
shows a preview, and optionally writes it to disk.

Examples:
  nestor restore --from https://raw.githubusercontent.com/team/repo/main/nestor.yml
  nestor restore --from https://example.com/nestor.yml --dry-run
  nestor restore --from https://example.com/nestor.yml -o ~/.config/nestor/nestor.yml --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromURL, _ := cmd.Flags().GetString("from")
		return runRestoreOut(fromURL, os.Stdout)
	},
}

func runRestoreOut(fromURL string, w io.Writer) error {
	if fromURL == "" {
		return fmt.Errorf("--from <url> is required")
	}

	// 1. Fetch
	fmt.Fprintf(w, "nestor: fetching %s\n", fromURL)
	data, err := restore.Fetch(fromURL)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "nestor: fetched %d bytes\n", len(data))

	// 2. Validate
	cfg, err := restore.Validate(data)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	fmt.Fprintln(w, "nestor: config is valid ✓")

	// 3. Preview
	fmt.Fprintln(w, "\n--- preview ---")
	fmt.Fprint(w, restore.Preview(cfg))
	fmt.Fprintln(w, "--- end preview ---")

	// 4. Dry run stops here
	if restoreDryRun {
		fmt.Fprintln(w, "nestor: dry run, not writing to disk")
		return nil
	}

	// 5. Write
	dest := restoreOutput
	if dest == "" {
		dest = "nestor.yml"
	}

	if err := restore.Write(data, dest, restoreForce); err != nil {
		return err
	}

	fmt.Fprintf(w, "nestor: config written to %s\n", dest)
	fmt.Fprintln(w, "nestor: run 'nestor up' to apply it")
	return nil
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().String("from", "", "URL to fetch nestor.yml from (required)")
	restoreCmd.Flags().BoolVarP(&restoreDryRun, "dry-run", "n", false, "fetch and preview without writing")
	restoreCmd.Flags().StringVarP(&restoreOutput, "output", "o", "", "destination path (default: ./nestor.yml)")
	restoreCmd.Flags().BoolVarP(&restoreForce, "force", "f", false, "overwrite existing file")
}
