package cli

import (
	"sort"

	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var statusShort bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the working tree status",
	Long:  `Displays paths that have differences between the index file and the current HEAD commit, paths that have differences between the working tree and the index file, and paths in the working tree that are not tracked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			statusShort = false
			_ = cmd.Flags().Set("short", "false")
		}()

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		res, err := repo.Status()
		if err != nil {
			return err
		}

		if statusShort {
			type fileStat struct {
				x byte // staged
				y byte // worktree
			}
			stats := make(map[string]*fileStat)
			getStat := func(path string) *fileStat {
				s, ok := stats[path]
				if !ok {
					s = &fileStat{x: ' ', y: ' '}
					stats[path] = s
				}
				return s
			}

			for _, f := range res.StagedNew {
				getStat(f).x = 'A'
			}
			for _, f := range res.StagedModified {
				getStat(f).x = 'M'
			}
			for _, f := range res.StagedDeleted {
				getStat(f).x = 'D'
			}
			for _, f := range res.Modified {
				getStat(f).y = 'M'
			}
			for _, f := range res.Deleted {
				getStat(f).y = 'D'
			}

			var paths []string
			for p := range stats {
				paths = append(paths, p)
			}
			sort.Strings(paths)

			for _, p := range paths {
				s := stats[p]
				cmd.Printf("%c%c %s\n", s.x, s.y, p)
			}

			// Untracked files
			sort.Strings(res.Untracked)
			for _, f := range res.Untracked {
				cmd.Printf("?? %s\n", f)
			}

			return nil
		}

		clean := true

		if len(res.StagedNew) > 0 || len(res.StagedModified) > 0 || len(res.StagedDeleted) > 0 {
			clean = false
			cmd.Println("Changes to be committed:")
			for _, file := range res.StagedNew {
				cmd.Printf("  \tnew file:   %s\n", file)
			}
			for _, file := range res.StagedModified {
				cmd.Printf("  \tmodified:   %s\n", file)
			}
			for _, file := range res.StagedDeleted {
				cmd.Printf("  \tdeleted:    %s\n", file)
			}
			cmd.Println()
		}

		if len(res.Modified) > 0 || len(res.Deleted) > 0 {
			clean = false
			cmd.Println("Changes not staged for commit:")
			for _, file := range res.Modified {
				cmd.Printf("  \tmodified:   %s\n", file)
			}
			for _, file := range res.Deleted {
				cmd.Printf("  \tdeleted:    %s\n", file)
			}
			cmd.Println()
		}

		if len(res.Untracked) > 0 {
			clean = false
			cmd.Println("Untracked files:")
			for _, file := range res.Untracked {
				cmd.Printf("  \t%s\n", file)
			}
			cmd.Println()
		}

		if clean {
			cmd.Println("nothing to commit, working tree clean")
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusShort, "short", "s", false, "Give the output in the short-format")
	rootCmd.AddCommand(statusCmd)
}
