package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/repository"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage set of tracked repositories",
	Long:  "Manage the set of repositories ('remotes') whose branches you track.",
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Add a new remote repository",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		url := args[1]
		token, _ := cmd.Flags().GetString("token")

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		if err := repo.AddRemote(name, url, token); err != nil {
			return err
		}

		fmt.Printf("Added remote %s (%s)\n", name, url)
		return nil
	},
}

var remoteRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a remote repository",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		if err := repo.RemoveRemote(name); err != nil {
			return err
		}

		fmt.Printf("Removed remote %s\n", name)
		return nil
	},
}

var remoteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured remotes",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		cfg, err := repo.LoadConfig()
		if err != nil {
			return err
		}

		if len(cfg.Remotes) == 0 {
			fmt.Println("No remotes configured.")
			return nil
		}

		verbose, _ := cmd.Flags().GetBool("verbose")
		for name, rem := range cfg.Remotes {
			if verbose {
				fmt.Printf("%s\t%s\n", name, rem.URL)
			} else {
				fmt.Println(name)
			}
		}
		return nil
	},
}

func init() {
	remoteAddCmd.Flags().StringP("token", "t", "", "GitHub Personal Access Token (or uses GITHUB_TOKEN env var)")
	remoteListCmd.Flags().BoolP("verbose", "v", false, "Show remote URLs")

	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteListCmd)

	rootCmd.AddCommand(remoteCmd)
}
