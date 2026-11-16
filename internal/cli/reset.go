package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/repository"
)

var resetCmd = &cobra.Command{
	Use:   "reset [commit]",
	Short: "Reset current HEAD to the specified state",
	Long: `Reset current HEAD to the specified commit state.
Supports --soft, --mixed (default), and --hard modes.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		targetRev := "HEAD"
		if len(args) > 0 {
			targetRev = args[0]
		}

		targetCommit, err := repo.ResolveRevision(targetRev)
		if err != nil {
			return fmt.Errorf("resolving target %q: %w", targetRev, err)
		}

		soft, _ := cmd.Flags().GetBool("soft")
		hard, _ := cmd.Flags().GetBool("hard")
		mixed, _ := cmd.Flags().GetBool("mixed")

		mode := repository.ResetMixed
		if soft {
			mode = repository.ResetSoft
		} else if hard {
			mode = repository.ResetHard
		} else if mixed {
			mode = repository.ResetMixed
		}

		if err := repo.Reset(targetCommit, mode); err != nil {
			return err
		}

		modeStr := "--mixed"
		if mode == repository.ResetSoft {
			modeStr = "--soft"
		} else if mode == repository.ResetHard {
			modeStr = "--hard"
		}

		fmt.Printf("HEAD is now at %s (reset %s)\n", targetCommit[:min(8, len(targetCommit))], modeStr)
		return nil
	},
}

func init() {
	resetCmd.Flags().Bool("soft", false, "reset HEAD only, keep index and working tree")
	resetCmd.Flags().Bool("mixed", false, "reset HEAD and index, keep working tree (default)")
	resetCmd.Flags().Bool("hard", false, "reset HEAD, index, and working tree (discards all changes)")
	rootCmd.AddCommand(resetCmd)
}
