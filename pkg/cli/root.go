package cli

import (
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "notion-md-sync",
	Short: "Bridge between markdown files and Notion pages",
	Long: `notion-md-sync is a CLI tool that synchronizes markdown files with Notion pages.
It supports bidirectional synchronization, allowing you to push changes from markdown
to Notion or pull changes from Notion to markdown files.`,
	Version: "0.16.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Set up logging based on verbose flag
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if verbose {
			util.SetLogLevel(util.DEBUG)
		} else {
			util.SetLogLevel(util.INFO)
		}
	}

	// Add subcommands
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(watchCmd)
}

func printVerbose(format string, args ...interface{}) {
	util.Debug(format, args...)
}
