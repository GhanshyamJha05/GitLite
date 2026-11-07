package cli

import (
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout <branch-or-commit>",
	Short: "Switch branches or restore working tree files",
	Long:  `Updates files in the working tree to match the version in the index or the specified tree.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		if err := repo.Checkout(target); err != nil {
			return err
		}

		cmd.Printf("Switched to branch '%s'\n", target)
		return nil
	},
}

var switchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Switch branches",
	Long:  `Switch to a specified branch. The working tree and the index are updated to match the branch.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		if err := repo.Checkout(target); err != nil {
			return err
		}

		cmd.Printf("Switched to branch '%s'\n", target)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkoutCmd)
	rootCmd.AddCommand(switchCmd)
}
