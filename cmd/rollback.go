package cmd

import (
	"fmt"
	"os"

	"github.com/jfjrh2014/nestor/internal/snapshot"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [snapshot-id]",
	Short: "Restore files from the latest (or specified) snapshot",
	Long: `Rolls back dotfile destinations to their state before the
last 'nestor up'. Pass a snapshot ID to restore a specific snapshot.
Run without arguments to restore the most recent one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := ui.New(os.Stdout)

		id := ""
		if len(args) > 0 {
			id = args[0]
		}

		if id == "" {
			p.Info("restoring latest snapshot")
		} else {
			p.Info(fmt.Sprintf("restoring snapshot %s", id))
		}

		snap, err := snapshot.Restore(id)
		if err != nil {
			return fmt.Errorf("rollback: %w", err)
		}

		p.OK(fmt.Sprintf("restored %d files from %s", len(snap.Files), snap.CreatedAt.Format("2006-01-02 15:04:05")))
		for _, f := range snap.Files {
			p.Detail("restored", f.Original)
		}

		return nil
	},
}

var snapshotsKeepFlag int

var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "List available snapshots, or prune old ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := ui.New(os.Stdout)

		ids, err := snapshot.List()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			p.Info("no snapshots found")
			return nil
		}

		p.Header("snapshots")
		for _, id := range ids {
			p.OK(id)
		}
		p.Info(fmt.Sprintf("%d snapshot(s) total", len(ids)))
		return nil
	},
}

var snapshotsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old snapshots, keeping only the N most recent",
	Long: `Removes old snapshot directories, keeping only the N most
recent snapshots (default: 10). Safe to run any time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := ui.New(os.Stdout)

		removed, err := snapshot.Prune(snapshotsKeepFlag)
		if err != nil {
			return fmt.Errorf("prune: %w", err)
		}
		switch len(removed) {
		case 0:
			p.Info(fmt.Sprintf("nothing to prune (keep=%d)", snapshotsKeepFlag))
		default:
			p.OK(fmt.Sprintf("pruned %d snapshot(s), keeping %d", len(removed), snapshotsKeepFlag))
			for _, id := range removed {
				p.Detail("removed", id)
			}
		}
		return nil
	},
}

func init() {
	snapshotsCmd.PersistentFlags().IntVar(&snapshotsKeepFlag, "keep", 10, "number of recent snapshots to keep when pruning")
	snapshotsCmd.AddCommand(snapshotsPruneCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(snapshotsCmd)
}
