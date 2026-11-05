package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/spf13/cobra"
)

var (
	configPath     string
	verbose        bool
	commandTimeout time.Duration
)

// defaultTimeout bounds a whole command. Large workspaces take a while: the
// Notion API handles only a few requests per second.
const defaultTimeout = 30 * time.Minute

// commandContext returns the context for a command: it ends after --timeout
// or when the user presses Ctrl+C.
func commandContext() (context.Context, context.CancelFunc) {
	timeout := commandTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancelTimeout := context.WithTimeout(ctx, timeout)
	go func() {
		// Once cancelled (first Ctrl+C or timeout), restore the default
		// handling so a second Ctrl+C always exits.
		<-ctx.Done()
		cancelSignal()
	}()
	return ctx, func() {
		cancelTimeout()
		cancelSignal()
	}
}

var rootCmd = &cobra.Command{
	Use:   "notion-md-sync",
	Short: "Bridge between markdown files and Notion pages",
	Long: `notion-md-sync is a CLI tool that synchronizes markdown files with Notion pages.
It supports bidirectional synchronization, allowing you to push changes from markdown
to Notion or pull changes from Notion to markdown files.`,
	Version: "0.17.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().DurationVar(&commandTimeout, "timeout", defaultTimeout, "maximum time a command may run (e.g. 10m, 2h)")

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
	if verbose {
		msg := format
		if len(args) > 0 {
			msg = fmt.Sprintf(format, args...)
		}
		fmt.Fprintf(os.Stderr, "[VERBOSE] %s\n", msg)
	}
}
