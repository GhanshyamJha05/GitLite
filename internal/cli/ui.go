package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"gitlite/internal/repository"
	"gitlite/internal/webui"

	"github.com/spf13/cobra"
)

var (
	uiPort   int
	uiNoOpen bool
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive web visualizer for the GitLite repository",
	Long: `Start an embedded zero-dependency local web server that visualizes
the commit DAG history, branches, file changes, and diffs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := repository.Discover("")
		if err != nil {
			return err
		}

		server := webui.NewServer(repo, uiPort)
		readyChan := make(chan string, 1)

		go func() {
			url := <-readyChan
			cmd.Printf("GitLite Visualizer running at: %s\n", url)
			cmd.Printf("Press Ctrl+C to stop.\n")

			if !uiNoOpen {
				openBrowser(url)
			}
		}()

		if err := server.Start(readyChan); err != nil {
			return fmt.Errorf("web visualizer server error: %w", err)
		}
		return nil
	},
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default: // linux, freebsd, etc.
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
	}
}

func init() {
	uiCmd.Flags().IntVarP(&uiPort, "port", "p", 8080, "Port for the web server to listen on")
	uiCmd.Flags().BoolVar(&uiNoOpen, "no-open", false, "Do not automatically launch default web browser")
	rootCmd.AddCommand(uiCmd)
}
