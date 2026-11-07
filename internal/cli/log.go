package cli

import (
	"strings"

	"gitlite/internal/refs"
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show commit logs",
	Long:  `List commit history reachable from the current HEAD in reverse chronological order.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		headHash, _, err := repo.Refs().ResolveHead()
		if err != nil {
			if err == refs.ErrRefNotFound {
				cmd.Println("fatal: your current branch does not have any commits yet")
				return nil
			}
			return err
		}

		commits, err := repo.Log(headHash)
		if err != nil {
			return err
		}

		for i, c := range commits {
			cmd.Printf("commit %s\n", c.Hash)
			if len(c.Commit.Parents) > 1 {
				var pShort []string
				for _, p := range c.Commit.Parents {
					pShort = append(pShort, string(p[:8]))
				}
				cmd.Printf("Merge: %s\n", strings.Join(pShort, " "))
			}
			cmd.Printf("Author: %s <%s>\n", c.Commit.Author.Name, c.Commit.Author.Email)
			cmd.Printf("Date:   %s\n\n", c.Commit.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))

			// Indent commit message
			msgLines := strings.Split(c.Commit.Message, "\n")
			for _, line := range msgLines {
				cmd.Printf("    %s\n", line)
			}

			if i < len(commits)-1 {
				cmd.Println()
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
