package cli

import (
	"github.com/spf13/cobra"
	"gitlite/internal/repository"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the working tree status",
	Long:  `Displays paths that have differences between the index file and the current HEAD commit, paths that have differences between the working tree and the index file, and paths in the working tree that are not tracked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		res, err := repo.Status()
		if err != nil {
			return err
		}

		clean := true

		if len(res.StagedNew) > 0 {
			clean = false
			cmd.Println("Changes to be committed:")
			for _, file := range res.StagedNew {
				cmd.Printf("  \tnew file:   %s\n", file)
			}
			cmd.Println()
		}

		if len(res.Modified) > 0 || len(res.Deleted) > 0 {
			clean = false
			cmd.Println("Changes not staged for commit:")
			for _, file := range res.Modified {
				cmd.Printf("  \tmodified:   %s\n", file)
			}
			for _, file := range res.Deleted {
				cmd.Printf("  \tdeleted:    %s\n", file)
			}
			cmd.Println()
		}

		if len(res.Untracked) > 0 {
			clean = false
			cmd.Println("Untracked files:")
			for _, file := range res.Untracked {
				cmd.Printf("  \t%s\n", file)
			}
			cmd.Println()
		}

		if clean {
			cmd.Println("nothing to commit, working tree clean")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
