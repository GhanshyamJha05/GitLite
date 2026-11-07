package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gitlite",
	Short: "GitLite is a lightweight, independent version control system written in Go",
	Long: `GitLite is an educational, production-grade distributed version control system
built from scratch in Go. It implements content-addressable storage, object hashing,
trees, commits, branching, diffing, and 3-way merge resolution.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// GetRootCmd exposes the root command for testing purposes.
func GetRootCmd() *cobra.Command {
	return rootCmd
}
