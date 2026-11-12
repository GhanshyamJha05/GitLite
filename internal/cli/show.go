package cli

import (
	"fmt"
	"strings"

	"gitlite/internal/object"
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show [commit-hash]",
	Short: "Show various types of objects, starting with commits",
	Long:  `Shows the commit header, metadata, log message, and root tree hash for the specified commit or current HEAD.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		var targetHash object.Hash
		if len(args) == 0 {
			headHash, err := repo.ResolveRevision("HEAD")
			if err != nil {
				return err
			}
			targetHash = headHash
		} else {
			h, err := repo.ResolveRevision(args[0])
			if err != nil {
				return fmt.Errorf("invalid object %s: %w", args[0], err)
			}
			targetHash = h
		}

		commitObj, err := repo.Store.ReadCommit(targetHash)
		if err != nil {
			return fmt.Errorf("reading commit %s: %w", targetHash, err)
		}

		cmd.Printf("commit %s\n", targetHash)
		cmd.Printf("tree   %s\n", commitObj.TreeHash)
		for _, p := range commitObj.Parents {
			cmd.Printf("parent %s\n", p)
		}
		cmd.Printf("Author: %s <%s>\n", commitObj.Author.Name, commitObj.Author.Email)
		cmd.Printf("Date:   %s\n\n", commitObj.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))

		for _, line := range strings.Split(commitObj.Message, "\n") {
			cmd.Printf("    %s\n", line)
		}

		// Display commit diff against its primary parent (if any)
		if len(commitObj.Parents) > 0 {
			cmd.Println()
			diffOutput, err := repo.DiffCommits(commitObj.Parents[0], targetHash)
			if err == nil && diffOutput != "" {
				cmd.Print(diffOutput)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
