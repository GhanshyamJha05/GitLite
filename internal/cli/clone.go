package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/remote"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repository> [directory]",
	Short: "Clone a repository into a new directory",
	Long:  "Clone a remote repository into a directory, create remote-tracking branches, and checkout the initial branch.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		remoteURL := args[0]
		targetDir := ""
		if len(args) == 2 {
			targetDir = args[1]
		}

		token, _ := cmd.Flags().GetString("token")

		fmt.Printf("Cloning into '%s'...\n", targetDir)
		repo, err := remote.Clone(remoteURL, targetDir, token)
		if err != nil {
			return err
		}

		headCommit, headBranch, err := repo.Refs().ResolveHead()
		if err == nil {
			shortCommit := string(headCommit)
			if len(shortCommit) > 8 {
				shortCommit = shortCommit[:8]
			}
			fmt.Printf("Checked out '%s' at commit %s\n", headBranch, shortCommit)
		}
		return nil
	},
}

func init() {
	cloneCmd.Flags().StringP("token", "t", "", "GitHub Personal Access Token (or uses GITHUB_TOKEN env var)")
	rootCmd.AddCommand(cloneCmd)
}
