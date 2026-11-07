package cli

import (
	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Create an empty GitLite repository or reinitialize an existing one",
	Long:  `Initialize a new, empty GitLite repository. Creates a .gitlite directory with objects, refs, HEAD, and config.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		repo, err := repository.Init(targetDir)
		if err != nil {
			return err
		}

		cmd.Printf("Initialized empty GitLite repository in %s\n", repo.GitLiteDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
