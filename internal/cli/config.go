package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/repository"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "Get and set repository options",
	Long: `Get and set repository options such as user.name and user.email.
Use --list to view all configuration variables.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		list, _ := cmd.Flags().GetBool("list")
		if list || len(args) == 0 {
			configs, err := repo.ListConfig()
			if err != nil {
				return err
			}
			for k, v := range configs {
				fmt.Printf("%s=%s\n", k, v)
			}
			return nil
		}

		key := args[0]
		if len(args) == 1 {
			val, err := repo.GetConfigValue(key)
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		}

		value := args[1]
		if err := repo.SetConfigValue(key, value); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	configCmd.Flags().BoolP("list", "l", false, "list all configuration variables")
	rootCmd.AddCommand(configCmd)
}
