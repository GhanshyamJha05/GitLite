package cli

import (
	"github.com/spf13/cobra"
	"gitlite/internal/integrity"
	"gitlite/internal/repository"
)

var fsckCmd = &cobra.Command{
	Use:   "fsck",
	Short: "Verify the connectivity and validity of objects in the database",
	Long:  `Verifies the connectivity and validity of the objects in the database, checking hashes, tree pointers, commit parents, and identifying dangling objects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		report, err := integrity.Fsck(repo)
		if err != nil {
			return err
		}

		hasErrors := false
		for _, issue := range report.Issues {
			cmd.Printf("%s: %s\n", issue.Level, issue.Message)
			if issue.Level == "ERROR" {
				hasErrors = true
			}
		}

		for _, d := range report.DanglingObjects {
			cmd.Printf("dangling commit/tree/blob %s\n", d)
		}

		if !hasErrors && len(report.Issues) == 0 {
			cmd.Printf("Checking object directories: 100%% (%d/%d), done.\n", report.ValidObjects, report.TotalObjects)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fsckCmd)
}
