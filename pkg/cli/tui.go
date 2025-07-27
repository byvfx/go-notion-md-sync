package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/byvfx/go-notion-md-sync/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// tuiCmd represents the TUI command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive terminal user interface",
	Long: `Launch the interactive terminal user interface (TUI) for notion-md-sync.

The TUI provides a visual interface for:
- Browsing and selecting files with sync status
- Monitoring real-time sync progress
- Resolving conflicts interactively
- Viewing sync statistics and health
- Configuring settings visually

Navigation:
- Use arrow keys to navigate within panes
- Press 'tab' to switch between file list and sync status
- Press 'c' to open configuration
- Press 's' to sync selected files
- Press 'q' or Ctrl+C to quit`,
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Get config path
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Create the TUI model
	model := tui.NewModel(configPath)

	// Create the Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
		tea.WithInput(os.Stdin),   // Explicit input handling
		tea.WithOutput(os.Stderr), // Use stderr for TUI output to avoid conflicts with captured stdout
	)

	// Set up signal handling for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		p.Quit()
	}()

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	// Return the configPath as-is (empty string if not provided)
	// This allows config.Load to use its own discovery logic
	return configPath, nil
}
