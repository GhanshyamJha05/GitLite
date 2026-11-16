package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/remote"
	"gitlite/internal/repository"
)

var pullCmd = &cobra.Command{
	Use:   "pull [remote] [branch]",
	Short: "Fetch from and integrate with another repository or a local branch",
	Long:  "Fetch changes from the specified remote and merge them into the current branch.",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		remoteName := "origin"
		branchName := ""

		if len(args) >= 1 {
			remoteName = args[0]
		}
		if len(args) >= 2 {
			branchName = args[1]
		}

		token, _ := cmd.Flags().GetString("token")

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		fmt.Printf("Pulling from %s...\n", remoteName)
		res, err := remote.Pull(repo, remoteName, branchName, token)
		if err != nil {
			return err
		}

		fmt.Printf("From %s\n", res.RemoteURL)
		fmt.Printf("Successfully updated %d commit(s).\n", res.FetchedCommits)
		return nil
	},
}

func init() {
	pullCmd.Flags().StringP("token", "t", "", "GitHub Personal Access Token (or uses GITHUB_TOKEN env var)")
	rootCmd.AddCommand(pullCmd)
}
