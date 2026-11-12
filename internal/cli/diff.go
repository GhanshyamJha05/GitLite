package cli

import (
	"fmt"

	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var diffStaged bool

var diffCmd = &cobra.Command{
	Use:   "diff [commit-a] [commit-b]",
	Short: "Show changes between commits, commit and working tree, etc",
	Long:  `Show changes between the working tree and the index, changes between the index and HEAD (--staged), or differences between two commits.`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		// Case 1: Two revisions provided
		if len(args) == 2 {
			hashA, err := repo.ResolveRevision(args[0])
			if err != nil {
				return fmt.Errorf("invalid commit A (%s): %w", args[0], err)
			}
			hashB, err := repo.ResolveRevision(args[1])
			if err != nil {
				return fmt.Errorf("invalid commit B (%s): %w", args[1], err)
			}
			out, err := repo.DiffCommits(hashA, hashB)
			if err != nil {
				return err
			}
			cmd.Print(out)
			return nil
		}

		// Case 2: diff --staged (index vs HEAD)
		isStaged, _ := cmd.Flags().GetBool("staged")
		isCached, _ := cmd.Flags().GetBool("cached")
		if isStaged || isCached {
			out, err := repo.DiffIndexAgainstHead()
			if err != nil {
				return err
			}
			cmd.Print(out)
			return nil
		}

		// Case 3: diff (working tree vs index)
		out, err := repo.DiffWorkingTreeAgainstIndex()
		if err != nil {
			return err
		}
		cmd.Print(out)
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		diffStaged = false
	},
}

func init() {
	diffCmd.Flags().BoolVar(&diffStaged, "staged", false, "View staged changes (index vs HEAD)")
	diffCmd.Flags().BoolVar(&diffStaged, "cached", false, "Synonym for --staged")
	rootCmd.AddCommand(diffCmd)
}
