package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlite/internal/object"
	"gitlite/internal/repository"
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

		// Case 1: Two commits provided
		if len(args) == 2 {
			hashA := object.Hash(args[0])
			hashB := object.Hash(args[1])
			if err := hashA.Validate(); err != nil {
				return fmt.Errorf("invalid commit A: %w", err)
			}
			if err := hashB.Validate(); err != nil {
				return fmt.Errorf("invalid commit B: %w", err)
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
