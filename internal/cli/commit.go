package cli

import (
	"fmt"

	"gitlite/internal/object"
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var (
	commitMsg string
	allFlag   bool
	amendFlag bool
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository",
	Long:  `Create a new commit containing the current contents of the index and the given log message describing the changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			commitMsg = ""
			allFlag = false
			amendFlag = false
			_ = cmd.Flags().Set("message", "")
			_ = cmd.Flags().Set("all", "false")
			_ = cmd.Flags().Set("amend", "false")
		}()

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		// -a: automatically stage modified and deleted files
		if allFlag {
			status, err := repo.Status()
			if err != nil {
				return fmt.Errorf("checking status for -a: %w", err)
			}
			for _, m := range status.Modified {
				if err := repo.Add(m); err != nil {
					return fmt.Errorf("staging modified file %s: %w", m, err)
				}
			}
			for _, d := range status.Deleted {
				idx, err := repo.LoadIndex()
				if err == nil {
					idx.Remove(d)
					_ = repo.SaveIndex(idx)
				}
			}
		}

		// Handle commit message
		if commitMsg == "" {
			if amendFlag {
				// Re-use current commit message if not provided
				headH, _, err := repo.Refs().ResolveHead()
				if err != nil || headH == "" {
					return fmt.Errorf("cannot amend without existing commit")
				}
				currCommit, err := repo.Store.ReadCommit(headH)
				if err != nil {
					return fmt.Errorf("reading commit: %w", err)
				}
				commitMsg = currCommit.Message
			} else {
				return fmt.Errorf("must provide a commit message with -m")
			}
		}

		sig := repo.DefaultSignature()
		var commitHash object.Hash
		if amendFlag {
			commitHash, err = repo.CommitAmend(commitMsg, sig, sig)
		} else {
			commitHash, err = repo.Commit(commitMsg, sig, sig)
		}
		if err != nil {
			return err
		}

		_, branch, _ := repo.Refs().ResolveHead()
		if branch == "" {
			branch = "detached HEAD"
		}

		prefix := ""
		if amendFlag {
			prefix = "(amended) "
		}
		cmd.Printf("[%s %s%s] %s\n", branch, prefix, string(commitHash[:8]), commitMsg)
		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMsg, "message", "m", "", "Commit message")
	commitCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "commit all changed files")
	commitCmd.Flags().BoolVar(&amendFlag, "amend", false, "amend previous commit")
	rootCmd.AddCommand(commitCmd)
}
