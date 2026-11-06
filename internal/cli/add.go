package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlite/internal/repository"
)

var addCmd = &cobra.Command{
	Use:   "add <path>...",
	Short: "Add file contents to the staging area",
	Long:  `Stage changes from the working tree to the GitLite staging area (index) for the next commit snapshot.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		for _, path := range args {
			if err := repo.Add(path); err != nil {
				return fmt.Errorf("adding %s: %w", path, err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

