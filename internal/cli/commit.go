package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlite/internal/repository"
)

var commitMsg string

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository",
	Long:  `Create a new commit containing the current contents of the index and the given log message describing the changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if commitMsg == "" {
			return fmt.Errorf("must provide a commit message with -m")
		}

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		sig := repository.DefaultSignature()
		commitHash, err := repo.Commit(commitMsg, sig, sig)
		if err != nil {
			return err
		}

		_, branch, _ := repo.Refs().ResolveHead()
		if branch == "" {
			branch = "detached HEAD"
		}

		cmd.Printf("[%s %s] %s\n", branch, string(commitHash[:8]), commitMsg)
		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMsg, "message", "m", "", "Commit message (required)")
	rootCmd.AddCommand(commitCmd)
}
