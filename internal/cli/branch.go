package cli

import (
	"fmt"

	"gitlite/internal/refs"
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var deleteBranchName string

var branchCmd = &cobra.Command{
	Use:   "branch [branch-name]",
	Short: "List, create, or delete branches",
	Long:  `Create a new branch pointing to the current commit, list all existing branches, or delete a branch with -d.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		refMgr := repo.Refs()

		// Case 1: Delete branch
		if deleteBranchName != "" {
			if err := refMgr.DeleteBranch(deleteBranchName); err != nil {
				return err
			}
			cmd.Printf("Deleted branch %s\n", deleteBranchName)
			return nil
		}

		// Case 2: Create new branch
		if len(args) == 1 {
			branchName := args[0]
			headCommit, _, err := refMgr.ResolveHead()
			if err != nil {
				if err == refs.ErrRefNotFound {
					return fmt.Errorf("fatal: not a valid object name: 'HEAD'")
				}
				return err
			}

			if err := refMgr.CreateBranch(branchName, headCommit); err != nil {
				return err
			}
			return nil
		}

		// Case 3: List branches
		branches, err := refMgr.ListBranches()
		if err != nil {
			return err
		}

		if len(branches) == 0 {
			// On an unborn branch with 0 commits yet
			_, currentBranch, _ := refMgr.ResolveHead()
			if currentBranch != "" {
				cmd.Printf("* \033[32m%s\033[0m (initial commit)\n", currentBranch)
			}
			return nil
		}

		for _, b := range branches {
			if b.IsCurrent {
				cmd.Printf("* \033[32m%s\033[0m\n", b.Name)
			} else {
				cmd.Printf("  %s\n", b.Name)
			}
		}

		return nil
	},
}

func init() {
	branchCmd.Flags().StringVarP(&deleteBranchName, "delete", "d", "", "Delete a branch")
	branchCmd.PostRun = func(cmd *cobra.Command, args []string) {
		deleteBranchName = ""
	}
	rootCmd.AddCommand(branchCmd)
}
