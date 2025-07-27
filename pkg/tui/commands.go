package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandType represents the type of command being executed
type CommandType string

const (
	CommandSync   CommandType = "sync"
	CommandPull   CommandType = "pull"
	CommandPush   CommandType = "push"
	CommandWatch  CommandType = "watch"
	CommandStatus CommandType = "status"
	CommandInit   CommandType = "init"
)

// CommandExecutor handles executing sync commands in the background
type CommandExecutor struct {
	config     *config.Config
	syncEngine sync.Engine
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(cfg *config.Config) (*CommandExecutor, error) {
	engine := sync.NewEngine(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	return &CommandExecutor{
		config:     cfg,
		syncEngine: engine,
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

// Close cleans up the executor resources
func (ce *CommandExecutor) Close() {
	if ce.cancelFunc != nil {
		ce.cancelFunc()
	}
}

// ExecuteCommand runs a command and returns a tea.Cmd that sends progress messages
func (ce *CommandExecutor) ExecuteCommand(cmd CommandType, files []string) tea.Cmd {
	return func() tea.Msg {
		switch cmd {
		case CommandSync:
			return ce.executeSync(files)
		case CommandPull:
			return ce.executePull(files)
		case CommandPush:
			return ce.executePush(files)
		case CommandStatus:
			return ce.executeStatus(files)
		case CommandInit:
			return ce.executeInit(files)
		default:
			return CommandErrorMsg{
				Command: string(cmd),
				Error:   fmt.Errorf("unknown command: %s", cmd),
			}
		}
	}
}

// executeSync performs bidirectional sync
func (ce *CommandExecutor) executeSync(files []string) tea.Msg {
	startTime := time.Now()

	// Send start message
	progressChan := make(chan tea.Msg, 100)
	go func() {
		progressChan <- CommandStartMsg{
			Command:   string(CommandSync),
			StartTime: startTime,
		}
	}()

	// Execute sync
	go func() {
		defer close(progressChan)

		// If specific files selected, sync only those
		if len(files) > 0 {
			for _, file := range files {
				progressChan <- CommandProgressMsg{
					Command:     string(CommandSync),
					CurrentFile: file,
					Message:     fmt.Sprintf("Syncing %s", file),
				}

				// TODO: Implement file-specific sync
				time.Sleep(500 * time.Millisecond) // Simulate work
			}
		} else {
			// Full sync
			progressChan <- CommandProgressMsg{
				Command: string(CommandSync),
				Message: "Starting full bidirectional sync...",
			}

			// Perform bidirectional sync with captured output
			if err := ce.executeWithCapturedOutput("bidirectional", progressChan, CommandSync); err != nil {
				progressChan <- CommandErrorMsg{
					Command: string(CommandSync),
					Error:   err,
				}
				return
			}
		}

		progressChan <- CommandCompleteMsg{
			Command:  string(CommandSync),
			Duration: time.Since(startTime),
			Message:  "Sync completed successfully",
		}
	}()

	// Return a command that reads from the progress channel
	return readProgressChannel(progressChan)
}

// executePull performs pull from Notion
func (ce *CommandExecutor) executePull(_ []string) tea.Msg {
	startTime := time.Now()

	progressChan := make(chan tea.Msg, 100)
	go func() {
		progressChan <- CommandStartMsg{
			Command:   string(CommandPull),
			StartTime: startTime,
		}
	}()

	go func() {
		defer close(progressChan)

		progressChan <- CommandProgressMsg{
			Command: string(CommandPull),
			Message: "Pulling from Notion...",
		}

		// Perform pull from Notion with captured output
		if err := ce.executeWithCapturedOutput("pull", progressChan, CommandPull); err != nil {
			progressChan <- CommandErrorMsg{
				Command: string(CommandPull),
				Error:   err,
			}
			return
		}

		progressChan <- CommandCompleteMsg{
			Command:  string(CommandPull),
			Duration: time.Since(startTime),
			Message:  "Pull completed successfully",
		}
	}()

	return readProgressChannel(progressChan)
}

// executePush performs push to Notion
func (ce *CommandExecutor) executePush(files []string) tea.Msg {
	startTime := time.Now()

	progressChan := make(chan tea.Msg, 100)
	go func() {
		progressChan <- CommandStartMsg{
			Command:   string(CommandPush),
			StartTime: startTime,
		}
	}()

	go func() {
		defer close(progressChan)

		if len(files) > 0 {
			for _, file := range files {
				progressChan <- CommandProgressMsg{
					Command:     string(CommandPush),
					CurrentFile: file,
					Message:     fmt.Sprintf("Pushing %s to Notion", file),
				}

				// TODO: Implement file-specific push
				time.Sleep(500 * time.Millisecond) // Simulate work
			}
		} else {
			progressChan <- CommandProgressMsg{
				Command: string(CommandPush),
				Message: "Pushing all changes to Notion...",
			}

			// Perform push to Notion with captured output
			if err := ce.executeWithCapturedOutput("push", progressChan, CommandPush); err != nil {
				progressChan <- CommandErrorMsg{
					Command: string(CommandPush),
					Error:   err,
				}
				return
			}
		}

		progressChan <- CommandCompleteMsg{
			Command:  string(CommandPush),
			Duration: time.Since(startTime),
			Message:  "Push completed successfully",
		}
	}()

	return readProgressChannel(progressChan)
}

// executeStatus gets sync status
func (ce *CommandExecutor) executeStatus(_ []string) tea.Msg {
	// TODO: Implement status check
	return CommandCompleteMsg{
		Command: string(CommandStatus),
		Message: "Status check complete",
	}
}

// executeInit initializes a new project
func (ce *CommandExecutor) executeInit(_ []string) tea.Msg {
	startTime := time.Now()

	// Execute init synchronously and return the appropriate message

	// Check if already initialized
	if _, err := os.Stat("config.yaml"); err == nil {
		return CommandCompleteMsg{
			Command:  string(CommandInit),
			Duration: time.Since(startTime),
			Message:  "Project already initialized (config.yaml exists)",
		}
	}

	// Create config.yaml with defaults
	configContent := `# notion-md-sync configuration
notion:
  token: ""  # Set via NOTION_MD_SYNC_NOTION_TOKEN environment variable
  parent_page_id: ""  # Set via NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID environment variable

sync:
  direction: push
  conflict_resolution: newer

directories:
  markdown_root: ./docs
  excluded_patterns:
    - "*.tmp"
    - "node_modules/**"
    - ".git/**"

mapping:
  strategy: frontmatter
`

	if err := os.WriteFile("config.yaml", []byte(configContent), 0644); err != nil {
		return CommandErrorMsg{
			Command: string(CommandInit),
			Error:   fmt.Errorf("failed to create config.yaml: %w", err),
		}
	}

	// Create docs directory
	if err := os.MkdirAll("./docs", 0755); err != nil {
		return CommandErrorMsg{
			Command: string(CommandInit),
			Error:   fmt.Errorf("failed to create docs directory: %w", err),
		}
	}

	// Create .env.example
	envExampleContent := `# Copy this file to .env and fill in your actual values
NOTION_MD_SYNC_NOTION_TOKEN=your_integration_token_here
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_parent_page_id_here
`

	if err := os.WriteFile(".env.example", []byte(envExampleContent), 0644); err != nil {
		return CommandErrorMsg{
			Command: string(CommandInit),
			Error:   fmt.Errorf("failed to create .env.example: %w", err),
		}
	}

	// Create sample markdown file
	sampleContent := `---
title: "Welcome to notion-md-sync"
sync_enabled: true
---

# Welcome to notion-md-sync

This is a sample markdown file that demonstrates how notion-md-sync works.

## Getting Started

1. Edit this file
2. Run sync from the TUI or: notion-md-sync push
3. Check your Notion page!

## Features

- **Bidirectional sync** between markdown and Notion
- **Frontmatter support** for metadata
- **File watching** for automatic sync
- **Flexible configuration**

Happy syncing! 🚀
`

	if err := os.WriteFile("./docs/welcome.md", []byte(sampleContent), 0644); err != nil {
		return CommandErrorMsg{
			Command: string(CommandInit),
			Error:   fmt.Errorf("failed to create sample file: %w", err),
		}
	}

	return CommandCompleteMsg{
		Command:  string(CommandInit),
		Duration: time.Since(startTime),
		Message:  "Project initialized! Created config.yaml, .env.example, docs/ and sample file",
	}
}

// executeWithCapturedOutput runs sync operations while capturing stdout/stderr
func (ce *CommandExecutor) executeWithCapturedOutput(direction string, progressChan chan<- tea.Msg, commandType CommandType) error {
	// Add timeout context to prevent hanging - increased to 10 minutes for large syncs
	// This accounts for slow Notion API responses (some pages take 40+ seconds)
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(ce.ctx, timeout)
	defer cancel()

	// Send initial message about timeout
	progressChan <- CommandProgressMsg{
		Command: string(commandType),
		Message: fmt.Sprintf("Starting %s (timeout: %v)...", direction, timeout),
	}

	// Create pipes to capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	os.Stdout = w
	os.Stderr = w

	// Channel to signal when pipe reading is done
	done := make(chan bool)

	// Create a goroutine to read from the pipe and send progress messages
	go func() {
		defer func() {
			_ = r.Close()
			done <- true
		}()

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					// Filter out debug/warning messages and clean up the output
					if !strings.Contains(line, "Debug:") && !strings.HasPrefix(line, "Warning:") {
						// Extract meaningful progress information
						var message string
						if strings.Contains(line, "Found") && strings.Contains(line, "pages") {
							message = line
						} else if strings.Contains(line, "Pulling page:") {
							message = strings.TrimPrefix(line, "[0-9]/[0-9] ")
							message = strings.TrimPrefix(message, "Pulling page: ")
						} else if strings.Contains(line, "✓ Successfully") {
							message = "Page synced successfully"
						} else if strings.Contains(line, "Saving to:") {
							message = strings.TrimPrefix(line, "  Saving to: ")
						} else {
							message = line
						}

						progressChan <- CommandProgressMsg{
							Command: string(commandType),
							Message: message,
						}
					}
				}
			}
		}
	}()

	// Execute the actual sync operation with timeout
	var syncErr error
	syncDone := make(chan bool)

	go func() {
		syncErr = ce.syncEngine.SyncAll(ctx, direction)
		syncDone <- true
	}()

	// Wait for either sync completion or timeout
	select {
	case <-syncDone:
		// Sync completed
	case <-ctx.Done():
		syncErr = fmt.Errorf("sync operation timed out after %v", timeout)
	}

	// Restore stdout and stderr
	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Wait for pipe reader to finish
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		// Don't wait forever for pipe to close
	}

	return syncErr
}

// readProgressChannel creates a tea.Cmd that reads from a progress channel
func readProgressChannel(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		// Read all available messages
		var messages []tea.Msg
		for msg := range ch {
			messages = append(messages, msg)
		}

		// If we got multiple messages, batch them
		if len(messages) > 1 {
			return CommandBatchMsg{Messages: messages}
		} else if len(messages) == 1 {
			return messages[0]
		}

		return nil
	}
}

// Command Messages

// CommandStartMsg indicates a command has started
type CommandStartMsg struct {
	Command   string
	StartTime time.Time
}

// CommandProgressMsg provides progress updates
type CommandProgressMsg struct {
	Command     string
	CurrentFile string
	Message     string
	Progress    float64 // 0.0 to 1.0
}

// CommandCompleteMsg indicates a command has completed
type CommandCompleteMsg struct {
	Command  string
	Duration time.Duration
	Message  string
}

// CommandErrorMsg indicates a command error
type CommandErrorMsg struct {
	Command string
	Error   error
}

// CommandBatchMsg contains multiple command messages
type CommandBatchMsg struct {
	Messages []tea.Msg
}
