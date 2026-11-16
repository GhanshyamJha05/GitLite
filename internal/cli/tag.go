package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlite/internal/repository"
)

var tagCmd = &cobra.Command{
	Use:   "tag [name] [commit]",
	Short: "Create, list, or delete tags",
	Long: `Create, list, or delete tag references.
With no arguments, lists all existing tags.
With one argument, creates a new tag at HEAD.
With -d, deletes the specified tag.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		deleteFlag, _ := cmd.Flags().GetBool("delete")
		if deleteFlag {
			if len(args) == 0 {
				return fmt.Errorf("must specify tag name to delete")
			}
			tagName := args[0]
			if err := repo.Refs().DeleteTag(tagName); err != nil {
				return err
			}
			fmt.Printf("Deleted tag %q\n", tagName)
			return nil
		}

		if len(args) == 0 {
			// List tags
			tags, err := repo.Refs().ListTags()
			if err != nil {
				return err
			}
			for _, t := range tags {
				fmt.Println(t.Name)
			}
			return nil
		}

		tagName := args[0]
		targetRev := "HEAD"
		if len(args) >= 2 {
			targetRev = args[1]
		}

		targetCommit, err := repo.ResolveRevision(targetRev)
		if err != nil {
			return fmt.Errorf("resolving %q: %w", targetRev, err)
		}

		if err := repo.Refs().CreateTag(tagName, targetCommit); err != nil {
			return err
		}

		fmt.Printf("Created tag %q at %s\n", tagName, targetCommit[:min(8, len(targetCommit))])
		return nil
	},
}

func init() {
	tagCmd.Flags().BoolP("delete", "d", false, "delete a tag")
	rootCmd.AddCommand(tagCmd)
}
