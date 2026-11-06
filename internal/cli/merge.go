package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"gitlite/internal/repository"
)

var mergeCmd = &cobra.Command{
	Use:   "merge <branch>",
	Short: "Join two development histories together",
	Long:  `Incorporates changes from the named branch or commit into the current branch using a three-way merge algorithm.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		res, err := repo.Merge(target)
		if err != nil {
			if errors.Is(err, repository.ErrMergeConflict) {
				cmd.Println("Automatic merge failed; fix conflicts and then commit the result.")
				for _, c := range res.Conflicts {
					cmd.Printf("CONFLICT (content): Merge conflict in %s\n", c)
				}
				return nil
			}
			if errors.Is(err, repository.ErrAlreadyUpToDate) {
				cmd.Println("Already up to date.")
				return nil
			}
			return err
		}

		if res.FastForward {
			cmd.Printf("Updating %s..%s\nFast-forward\n", target, string(res.MergeCommit[:8]))
			return nil
		}

		cmd.Printf("Merge made by the 'three-way' strategy.\n")
		for _, f := range res.MergedFiles {
			cmd.Printf(" merged: %s\n", f)
		}
		cmd.Printf("Commit: %s\n", res.MergeCommit)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}
