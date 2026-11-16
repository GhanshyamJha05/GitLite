package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/remote"
	"gitlite/internal/repository"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch [remote]",
	Short: "Download objects and refs from another repository",
	Long:  "Fetch branches and objects from the remote repository without merging into local branches.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remoteName := "origin"
		if len(args) >= 1 {
			remoteName = args[0]
		}

		token, _ := cmd.Flags().GetString("token")

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		fmt.Printf("Fetching from %s...\n", remoteName)
		res, err := remote.Fetch(repo, remoteName, token)
		if err != nil {
			return err
		}

		fmt.Printf("From %s\n", res.RemoteURL)
		for branch, sha := range res.UpdatedRefs {
			shortSha := sha
			if len(shortSha) > 8 {
				shortSha = shortSha[:8]
			}
			fmt.Printf(" * [new branch]      %s -> %s/%s (%s)\n", branch, remoteName, branch, shortSha)
		}
		fmt.Printf("Fetched %d commit(s).\n", res.FetchedCommits)
		return nil
	},
}

func init() {
	fetchCmd.Flags().StringP("token", "t", "", "GitHub Personal Access Token (or uses GITHUB_TOKEN env var)")
	rootCmd.AddCommand(fetchCmd)
}
