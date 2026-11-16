package cli

import (
	"fmt"
	"strings"

	"gitlite/internal/repository"

	"github.com/spf13/cobra"
)

var (
	diffStaged bool
	diffColor  bool
)

func colorizeDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")
	var colored []string
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			colored = append(colored, "\033[1m"+line+"\033[0m")
		} else if strings.HasPrefix(line, "+") {
			colored = append(colored, "\033[32m"+line+"\033[0m")
		} else if strings.HasPrefix(line, "-") {
			colored = append(colored, "\033[31m"+line+"\033[0m")
		} else if strings.HasPrefix(line, "@@") {
			colored = append(colored, "\033[36m"+line+"\033[0m")
		} else if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") {
			colored = append(colored, "\033[1m"+line+"\033[0m")
		} else {
			colored = append(colored, line)
		}
	}
	return strings.Join(colored, "\n")
}

var diffCmd = &cobra.Command{
	Use:   "diff [commit-a] [commit-b]",
	Short: "Show changes between commits, commit and working tree, etc",
	Long:  `Show changes between the working tree and the index, changes between the index and HEAD (--staged), or differences between two commits.`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			diffStaged = false
			diffColor = false
			_ = cmd.Flags().Set("staged", "false")
			_ = cmd.Flags().Set("cached", "false")
			_ = cmd.Flags().Set("color", "false")
		}()

		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		printOut := func(out string) {
			if diffColor {
				out = colorizeDiff(out)
			}
			cmd.Print(out)
		}

		// Case 1: Two revisions provided
		if len(args) == 2 {
			hashA, err := repo.ResolveRevision(args[0])
			if err != nil {
				return fmt.Errorf("invalid commit A (%s): %w", args[0], err)
			}
			hashB, err := repo.ResolveRevision(args[1])
			if err != nil {
				return fmt.Errorf("invalid commit B (%s): %w", args[1], err)
			}
			out, err := repo.DiffCommits(hashA, hashB)
			if err != nil {
				return err
			}
			printOut(out)
			return nil
		}

		// Case 2: diff --staged (index vs HEAD)
		isStaged, _ := cmd.Flags().GetBool("staged")
		isCached, _ := cmd.Flags().GetBool("cached")
		if isStaged || isCached {
			out, err := repo.DiffIndexAgainstHead()
			if err != nil {
				return err
			}
			printOut(out)
			return nil
		}

		// Case 3: diff (working tree vs index)
		out, err := repo.DiffWorkingTreeAgainstIndex()
		if err != nil {
			return err
		}
		printOut(out)
		return nil
	},
}

func init() {
	diffCmd.Flags().BoolVar(&diffStaged, "staged", false, "View staged changes (index vs HEAD)")
	diffCmd.Flags().BoolVar(&diffStaged, "cached", false, "Synonym for --staged")
	diffCmd.Flags().BoolVar(&diffColor, "color", false, "Show colored diff with ANSI escape codes")
	rootCmd.AddCommand(diffCmd)
}
