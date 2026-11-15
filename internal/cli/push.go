package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/remote"
	"gitlite/internal/repository"
)

var pushCmd = &cobra.Command{
	Use:   "push [remote] [branch]",
	Short: "Update remote refs along with associated objects",
	Long:  "Push local commits, trees, and blobs directly to GitHub via its Git Data API.",
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

		fmt.Printf("Pushing to %s...\n", remoteName)
		res, err := remote.Push(repo, remoteName, branchName, token)
		if err != nil {
			return err
		}

		fmt.Printf("To %s\n", res.RepoURL)
		fmt.Printf(" * [%s] %s -> %s\n", res.HeadCommit[:min(8, len(res.HeadCommit))], res.Branch, res.RemoteRef)
		fmt.Printf("Synced %d commit(s), %d tree(s), %d blob(s).\n", res.PushedCommits, res.PushedTrees, res.PushedBlobs)
		return nil
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	pushCmd.Flags().StringP("token", "t", "", "GitHub Personal Access Token (or uses GITHUB_TOKEN env var)")
	rootCmd.AddCommand(pushCmd)
}
