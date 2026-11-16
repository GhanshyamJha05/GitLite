package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlite/internal/lfs"
	"gitlite/internal/repository"
)

var lfsCmd = &cobra.Command{
	Use:   "lfs",
	Short: "Work with Git LFS (Large File Storage) in GitLite",
	Long:  "Commands to track, untrack, inspect, and manage large file storage assets and pointer specifications.",
}

var lfsTrackCmd = &cobra.Command{
	Use:   "track [pattern]",
	Short: "View or add Git LFS paths to .gitattributes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		if len(args) == 0 {
			patterns, err := lfs.ListTracked(repo.WorkTree)
			if err != nil {
				return err
			}
			if len(patterns) == 0 {
				cmd.Println("No LFS patterns tracked.")
				return nil
			}
			cmd.Println("Listing tracked patterns")
			for _, p := range patterns {
				cmd.Printf("    %s (%s)\n", p, lfs.GitAttributesFile)
			}
			return nil
		}

		pattern := args[0]
		if err := lfs.Track(repo.WorkTree, pattern); err != nil {
			return err
		}
		cmd.Printf("Tracking %q\n", pattern)
		return nil
	},
}

var lfsUntrackCmd = &cobra.Command{
	Use:   "untrack <pattern>",
	Short: "Remove Git LFS path from .gitattributes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		pattern := args[0]
		if err := lfs.Untrack(repo.WorkTree, pattern); err != nil {
			return err
		}
		cmd.Printf("Untracking %q\n", pattern)
		return nil
	},
}

var lfsLsFilesCmd = &cobra.Command{
	Use:   "ls-files",
	Short: "Show information about Git LFS files in the index and working tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		idx, err := repo.LoadIndex()
		if err != nil {
			return err
		}

		patterns, _ := lfs.ListTracked(repo.WorkTree)
		found := 0

		for _, entry := range idx.SortedEntries() {
			isMatch := false
			for _, pat := range patterns {
				if lfs.MatchesPattern(pat, entry.Path) {
					isMatch = true
					break
				}
			}

			if isMatch {
				cmd.Printf("%s * %s\n", entry.Hash[:min(10, len(string(entry.Hash)))], entry.Path)
				found++
			}
		}

		if found == 0 {
			cmd.Println("No LFS files in index.")
		}
		return nil
	},
}

var lfsPointerCmd = &cobra.Command{
	Use:   "pointer --file <path>",
	Short: "Build or inspect a Git LFS pointer file",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("must specify --file <path>")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		if lfs.IsPointer(data) {
			ptr, err := lfs.ParsePointer(data)
			if err != nil {
				return err
			}
			cmd.Printf("Pointer file valid:\n  OID:  %s\n  Size: %d bytes\n", ptr.OID, ptr.Size)
			return nil
		}

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()

		ptr, err := lfs.Store(repo.GitLiteDir, f)
		if err != nil {
			return err
		}

		cmd.Print(string(lfs.FormatPointer(ptr.OID, ptr.Size)))
		return nil
	},
}

var lfsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Initialize Git LFS configuration for GitLite",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("GitLite LFS initialized.")
		return nil
	},
}

func init() {
	lfsPointerCmd.Flags().String("file", "", "Path to file to generate or check pointer for")

	lfsCmd.AddCommand(lfsTrackCmd)
	lfsCmd.AddCommand(lfsUntrackCmd)
	lfsCmd.AddCommand(lfsLsFilesCmd)
	lfsCmd.AddCommand(lfsPointerCmd)
	lfsCmd.AddCommand(lfsInstallCmd)

	rootCmd.AddCommand(lfsCmd)
}
