package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gitlite/internal/repository"
)

var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "Stash the changes in a dirty working directory away",
	Long: `Use gitlite stash when you want to record the current state of the
working directory and the index, but want to go back to a clean working directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to stash push/save
		return stashPushCmd.RunE(cmd, args)
	},
}

var stashPushCmd = &cobra.Command{
	Use:     "push [-m message]",
	Aliases: []string{"save"},
	Short:   "Save your local modifications to a new stash entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		msg, _ := cmd.Flags().GetString("message")
		if msg == "" && len(args) > 0 {
			msg = strings.Join(args, " ")
		}

		entry, err := repo.StashSave(msg)
		if err != nil {
			return err
		}

		fmt.Printf("Saved working directory and index state %s: %s\n", entry.ID, entry.Description)
		return nil
	},
}

var stashPopCmd = &cobra.Command{
	Use:   "pop",
	Short: "Remove a single stashed state from the stash list and apply it on top of the current working tree state",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		entry, err := repo.StashPop()
		if err != nil {
			return err
		}

		fmt.Printf("Dropped %s (%s)\n", entry.ID, entry.Description)
		return nil
	},
}

var stashListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the stashed states that you currently have",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		stashes, err := repo.StashList()
		if err != nil {
			return err
		}

		if len(stashes) == 0 {
			fmt.Println("No stash entries.")
			return nil
		}

		for idx, s := range stashes {
			fmt.Printf("stash@{%d}: %s\n", idx, s.Description)
		}
		return nil
	},
}

func init() {
	stashPushCmd.Flags().StringP("message", "m", "", "Stash message")
	stashCmd.Flags().StringP("message", "m", "", "Stash message")

	stashCmd.AddCommand(stashPushCmd)
	stashCmd.AddCommand(stashPopCmd)
	stashCmd.AddCommand(stashListCmd)

	rootCmd.AddCommand(stashCmd)
}
