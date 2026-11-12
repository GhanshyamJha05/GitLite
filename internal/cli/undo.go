package cli

import (
	"errors"

	"gitlite/internal/journal"
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Revert the last repository operation safely",
	Long:  `Reverts the repository state (HEAD, active branch reference, index, and working tree files) to immediately before the last operation (commit, checkout, or merge).`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		snap, err := repo.Undo()
		if err != nil {
			if errors.Is(err, journal.ErrNoUndoAvailable) {
				cmd.Println("No operations available to undo.")
				return nil
			}
			return err
		}

		cmd.Printf("Undid operation: %s\n", snap.Description)
		headHash, branch, err := repo.Refs().ResolveHead()
		if err == nil && headHash != "" {
			short := string(headHash)
			if len(short) > 8 {
				short = short[:8]
			}
			if branch != "" {
				cmd.Printf("HEAD is now at branch %s (commit %s)\n", branch, short)
			} else {
				cmd.Printf("HEAD is now detached at %s\n", short)
			}
		} else if branch != "" {
			cmd.Printf("HEAD is now on unborn branch %s (initial commit)\n", branch)
		} else {
			cmd.Println("HEAD is on initial commit (unborn branch)")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
