package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UnifiedView represents the main split-pane view from the mockup
type UnifiedView struct {
	fileList      list.Model
	delegate      *compactDelegate
	syncOps       []SyncOperation
	width         int
	height        int
	isConnected   bool
	workspaceName string
	stats         struct {
		synced  int
		pending int
		errors  int
		today   int
	}
	selectedFiles map[string]bool
	focusedPane   int // 0 = file list, 1 = sync status

	// Integration with CLI
	config       *config.Config
	executor     *CommandExecutor
	syncing      bool
	syncProgress string

	// Error handling
	lastError    error
	errorMessage string
}

// NewUnifiedView creates a new unified view matching the mockup
func NewUnifiedView() UnifiedView {
	// Create file list
	items := []list.Item{
		FileItem{Name: "README.md", Status: StatusSynced, Desc: "2 hours ago"},
		FileItem{Name: "docs/guide.md", Status: StatusModified, Desc: "Modified"},
		FileItem{Name: "docs/api.md", Status: StatusError, Desc: "Sync error"},
		FileItem{Name: "drafts/ideas.md", Status: StatusPending, Desc: "Not synced"},
	}

	delegate := newCompactDelegate().(*compactDelegate)
	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	// Start with empty sync operations (no mock data)
	syncOps := []SyncOperation{}

	return UnifiedView{
		fileList:      l,
		delegate:      delegate,
		syncOps:       syncOps,
		isConnected:   true,
		workspaceName: "My Notion Workspace",
		stats: struct {
			synced  int
			pending int
			errors  int
			today   int
		}{
			synced:  3,
			pending: 1,
			errors:  1,
			today:   0,
		},
		selectedFiles: make(map[string]bool),
		focusedPane:   0,
	}
}

// NewUnifiedViewWithConfig creates a new unified view with config and executor
func NewUnifiedViewWithConfig(cfg *config.Config, executor *CommandExecutor) UnifiedView {
	v := NewUnifiedView()
	v.config = cfg
	v.executor = executor

	// Update workspace name and connection status from config if available
	if cfg != nil {
		if cfg.Notion.ParentPageID != "" {
			v.workspaceName = "Notion Workspace"
			v.isConnected = true
		} else {
			v.workspaceName = "Not Connected"
			v.isConnected = false
		}
	} else {
		v.workspaceName = "No Configuration"
		v.isConnected = false
	}

	// Load actual files instead of mock data
	if cfg != nil {
		scanner := NewFileScanner(cfg)
		if items, err := scanner.ScanFiles(); err == nil && len(items) > 0 {
			v.fileList.SetItems(items)
			// Update stats based on actual files
			v.updateStats(items)
		}
	}

	return v
}

// Init implements tea.Model
func (v UnifiedView) Init() tea.Cmd {
	// Refresh file list on startup
	return v.refreshFileList()
}

// Update implements tea.Model
func (v UnifiedView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			// Switch focus between panes
			v.focusedPane = (v.focusedPane + 1) % 2
			return v, nil

		case "s":
			// Start sync
			if v.executor == nil {
				v.errorMessage = "Cannot sync: configuration not loaded"
				return v, v.clearErrorAfterDelay()
			}
			if !v.syncing {
				v.syncing = true
				v.syncProgress = "Starting sync..."
				v.syncOps = []SyncOperation{} // Clear previous operations

				// Get selected files
				var selectedFiles []string
				for file, selected := range v.selectedFiles {
					if selected {
						selectedFiles = append(selectedFiles, file)
					}
				}

				// Execute sync command
				return v, v.executor.ExecuteCommand(CommandSync, selectedFiles)
			}
			return v, nil

		case "p":
			// Pull from Notion
			if v.executor == nil {
				v.errorMessage = "Cannot pull: configuration not loaded"
				return v, v.clearErrorAfterDelay()
			}
			if !v.syncing {
				v.syncing = true
				v.syncProgress = "Starting pull from Notion..."
				v.syncOps = []SyncOperation{} // Clear previous operations

				// Get selected files
				var selectedFiles []string
				for file, selected := range v.selectedFiles {
					if selected {
						selectedFiles = append(selectedFiles, file)
					}
				}

				// Execute pull command
				return v, v.executor.ExecuteCommand(CommandPull, selectedFiles)
			}
			return v, nil

		case "P":
			// Push to Notion
			if v.executor == nil {
				v.errorMessage = "Cannot push: configuration not loaded"
				return v, v.clearErrorAfterDelay()
			}
			if !v.syncing {
				v.syncing = true
				v.syncProgress = "Starting push to Notion..."
				v.syncOps = []SyncOperation{} // Clear previous operations

				// Get selected files
				var selectedFiles []string
				for file, selected := range v.selectedFiles {
					if selected {
						selectedFiles = append(selectedFiles, file)
					}
				}

				// Execute push command
				return v, v.executor.ExecuteCommand(CommandPush, selectedFiles)
			}
			return v, nil

		case "i":
			// Initialize project
			if !v.syncing {
				v.syncing = true
				v.syncProgress = "Initializing project..."
				// Execute init directly without executor since it's a simple file operation
				return v, func() tea.Msg {
					return executeInitDirectly()
				}
			}
			return v, nil

		case "c":
			// Open config - this will be handled by the main model
			return v, nil

		case "h", "?":
			// Show help
			// TODO: Show help modal
			return v, nil

		case " ":
			// Toggle selection if in file list
			if v.focusedPane == 0 {
				if i, ok := v.fileList.SelectedItem().(FileItem); ok {
					v.selectedFiles[i.Name] = !v.selectedFiles[i.Name]
				}
			}
			return v, nil
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.updatePaneSizes()
		return v, nil

	// Handle command messages
	case CommandStartMsg:
		v.syncProgress = fmt.Sprintf("Started %s...", msg.Command)
		v.addSyncOperation(msg.Command, "In Progress", msg.StartTime)
		return v, nil

	case CommandProgressMsg:
		v.syncProgress = msg.Message
		if msg.CurrentFile != "" {
			v.updateFileStatus(msg.CurrentFile, StatusPending)
			// For messages with CurrentFile set, extract the actual status from the message
			status := "Processing"
			if strings.Contains(msg.Message, "✓ Completed:") {
				status = "Completed"
			} else if strings.Contains(msg.Message, "Pulling:") {
				status = "Pulling from Notion"
			} else if strings.Contains(msg.Message, "Pushing:") {
				status = "Pushing to Notion"
			}
			// Add or update sync operation for the current file
			v.addOrUpdateSyncOperation(msg.CurrentFile, status)
		}
		return v, nil

	case CommandCompleteMsg:
		v.syncing = false
		v.syncProgress = msg.Message
		v.updateSyncOperation(msg.Command, "Completed", msg.Duration)
		// Clear selection after successful sync and refresh file list
		v.selectedFiles = make(map[string]bool)
		// Update today's sync count
		v.stats.today += len(v.syncOps)

		// For all commands, refresh the file list
		// (init creates new files, sync operations may change file status)

		return v, v.refreshFileList()

	case CommandErrorMsg:
		v.syncing = false
		v.lastError = msg.Error
		v.errorMessage = msg.Error.Error()
		v.syncProgress = fmt.Sprintf("❌ %s failed: %v", msg.Command, msg.Error)
		v.updateSyncOperation(msg.Command, "Failed", 0)
		// Clear error after 5 seconds
		return v, v.clearErrorAfterDelay()

	case CommandBatchMsg:
		// Process batch messages
		for _, batchMsg := range msg.Messages {
			model, _ := v.Update(batchMsg)
			if updatedView, ok := model.(UnifiedView); ok {
				v = updatedView
			}
		}
		return v, nil

	case FileListRefreshedMsg:
		// Update file list with refreshed items
		v.fileList.SetItems(msg.Items)
		v.updateStats(msg.Items)
		return v, nil

	case ClearErrorMsg:
		// Clear error message
		v.errorMessage = ""
		v.lastError = nil
		return v, nil

	case ProgressTickMsg:
		if v.executor != nil && v.executor.isRunning {
			// Create progress message from executor state
			elapsed := time.Since(v.executor.operationStartTime)
			progressMsg := fmt.Sprintf("%s (%.1fs elapsed)", v.executor.currentOperation, elapsed.Seconds())
			if v.executor.lastProgressMsg != "" {
				progressMsg = v.executor.lastProgressMsg
			}

			v.syncProgress = progressMsg

			// Continue ticking while operation is running
			return v, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return ProgressTickMsg{}
			})
		} else if v.executor != nil && !v.executor.isRunning && v.syncing {
			// Operation completed
			v.syncing = false
			if strings.Contains(v.executor.lastProgressMsg, "Error:") {
				v.errorMessage = v.executor.lastProgressMsg
				return v, v.clearErrorAfterDelay()
			} else {
				v.syncProgress = v.executor.lastProgressMsg
				// Update today's sync count and refresh file list
				v.stats.today++
				return v, v.refreshFileList()
			}
		}
		return v, nil
	}

	// Update the focused pane
	if v.focusedPane == 0 {
		var cmd tea.Cmd
		v.fileList, cmd = v.fileList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

// View implements tea.Model
func (v UnifiedView) View() string {
	// Use fallback dimensions if not set
	width := v.width
	height := v.height
	if width == 0 {
		width = 120 // Default terminal width
	}
	if height == 0 {
		height = 30 // Default terminal height
	}

	// Update dimensions if they were using defaults
	if v.width == 0 || v.height == 0 {
		v.width = width
		v.height = height
		v.updatePaneSizes()
	}

	// Define styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62"))

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	// Connection style changes based on status
	connectionColor := "42" // green
	if v.config == nil || v.config.Notion.Token == "" {
		connectionColor = "196" // red
	} else if v.config.Notion.ParentPageID == "" {
		connectionColor = "214" // orange
	}

	connectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(connectionColor))

	// Build header
	title := titleStyle.Render("notion-md-sync v0.14.0")

	// Show real connection status
	var connectionText string
	if v.config != nil && v.config.Notion.Token != "" {
		if v.config.Notion.ParentPageID != "" {
			connectionText = fmt.Sprintf("⚡ Connected: %s", v.workspaceName)
		} else {
			connectionText = "⚠️  Token set, missing parent page ID"
		}
	} else {
		connectionText = "❌ Not configured - run [i]nit"
	}
	connection := connectionStyle.Render(connectionText)
	headerContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		strings.Repeat(" ", width-lipgloss.Width(title)-lipgloss.Width(connection)-4),
		connection,
	)
	header := headerStyle.Render(headerContent)

	// Build left pane (Files)
	leftPaneTitle := "📁 Files"
	leftPaneContent := v.renderFileList()
	leftPaneStats := fmt.Sprintf("📊 %d synced | %d pending | %d error",
		v.stats.synced, v.stats.pending, v.stats.errors)

	leftPane := v.renderPane(leftPaneTitle, leftPaneContent, leftPaneStats, v.focusedPane == 0)

	// Build right pane (Sync Status)
	rightPaneTitle := "🔄 Sync Status"
	rightPaneContent := v.renderSyncStatus()
	rightPaneStats := fmt.Sprintf("📈 Today: %d files synced", v.stats.today)

	rightPane := v.renderPane(rightPaneTitle, rightPaneContent, rightPaneStats, v.focusedPane == 1)

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Build footer
	footerText := "💡 [s]ync | [p]ull | [P]ush | [i]nit | [c]onfigure | [tab] switch panes | [space] select | [q]uit"
	if v.errorMessage != "" {
		// Show error in red
		footerText = fmt.Sprintf("❌ Error: %s", v.errorMessage)
	} else if v.syncing && v.syncProgress != "" {
		footerText = fmt.Sprintf("⏳ %s", v.syncProgress)
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	// Make errors red
	if v.errorMessage != "" {
		footerStyle = footerStyle.Foreground(lipgloss.Color("196"))
	}

	footer := footerStyle.Render(footerText)

	// Combine all parts
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		panes,
		footer,
	)

	// Apply border to entire UI
	return borderStyle.Width(width - 2).Height(height - 2).Render(content)
}

// updatePaneSizes calculates pane dimensions
func (v *UnifiedView) updatePaneSizes() {
	width := v.width
	height := v.height
	if width == 0 {
		width = 120
	}
	if height == 0 {
		height = 30
	}

	// Account for borders and padding
	contentWidth := width - 4
	contentHeight := height - 6 // Header, footer, borders

	// Split width evenly
	paneWidth := contentWidth / 2

	// Update file list size
	v.fileList.SetSize(paneWidth-4, contentHeight-4)
}

// renderPane renders a single pane with title and content
func (v UnifiedView) renderPane(title, content, stats string, focused bool) string {
	// Use actual dimensions from the view
	width := v.width
	height := v.height
	if width == 0 {
		width = 120
	}
	if height == 0 {
		height = 30
	}

	paneWidth := (width - 4) / 2
	paneHeight := height - 6

	borderColor := "240"
	if focused {
		borderColor = "39"
	}

	paneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(paneWidth - 1).
		Height(paneHeight - 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	// Build pane content
	paneContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(title),
		content,
		statsStyle.Render(stats),
	)

	return paneStyle.Render(paneContent)
}

// renderFileList renders the file list content
func (v UnifiedView) renderFileList() string {
	// Update the delegate with current selected files
	if v.delegate != nil {
		v.delegate.setSelectedFiles(v.selectedFiles)
	}
	return v.fileList.View()
}

// renderSyncStatus renders the sync status content
func (v UnifiedView) renderSyncStatus() string {
	if len(v.syncOps) == 0 && !v.syncing {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(1)
		return emptyStyle.Render("No sync operations in progress")
	}

	var content []string

	// Show overall sync progress at the top
	if v.syncing && v.syncProgress != "" {
		progressStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))
		content = append(content, progressStyle.Render(v.syncProgress))
		content = append(content, "")
	}

	// Show individual operations
	for _, op := range v.syncOps {
		// Determine icon based on status
		var icon string
		statusColor := "214" // orange/yellow
		if strings.Contains(op.Status, "Completed") || strings.Contains(op.Status, "✓") {
			icon = "✅"
			statusColor = "42" // green
		} else if strings.Contains(op.Status, "Failed") || strings.Contains(op.Status, "Error") {
			icon = "❌"
			statusColor = "196" // red
		} else if strings.Contains(op.Status, "Pulling") {
			icon = "⬇️ "
		} else if strings.Contains(op.Status, "Pushing") {
			icon = "⬆️ "
		} else {
			icon = "⏳"
		}

		// Operation header with status color
		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
		header := headerStyle.Render(fmt.Sprintf("%s %s", icon, op.FileName))
		content = append(content, header)

		// Status details in a tree structure
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if op.Status != "" && !strings.Contains(op.Status, op.FileName) {
			content = append(content, detailStyle.Render("├─ "+op.Status))
		}
		if op.ElapsedTime > 0 {
			content = append(content, detailStyle.Render(fmt.Sprintf("└─ %.1fs", op.ElapsedTime.Seconds())))
		} else {
			content = append(content, detailStyle.Render("└─ Processing..."))
		}
		content = append(content, "") // Add spacing between operations
	}

	return lipgloss.NewStyle().Padding(1).Render(strings.Join(content, "\n"))
}

// compactDelegate is a minimal list delegate for the file list
type compactDelegate struct {
	selectedFiles map[string]bool
}

func newCompactDelegate() list.ItemDelegate {
	return &compactDelegate{
		selectedFiles: make(map[string]bool),
	}
}

func (d *compactDelegate) setSelectedFiles(selectedFiles map[string]bool) {
	d.selectedFiles = selectedFiles
}

func (d *compactDelegate) Height() int { return 1 }

func (d *compactDelegate) Spacing() int { return 0 }

func (d *compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d *compactDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(FileItem)
	if !ok {
		return
	}

	statusIcon := i.getStatusIcon()
	fileIcon := "📄"
	if i.IsDirectory {
		fileIcon = "📁"
	}

	// Add selection indicator
	selectionIndicator := "  "
	if d.selectedFiles[i.Name] {
		selectionIndicator = "› "
	}

	str := fmt.Sprintf("%s%s %s %s", selectionIndicator, fileIcon, i.Name, statusIcon)

	if index == m.Index() {
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
		_, _ = fmt.Fprint(w, selectedStyle.Render(str))
	} else {
		_, _ = fmt.Fprint(w, str)
	}
}

// Helper methods for sync operations

// addSyncOperation adds a new sync operation to the list
func (v *UnifiedView) addSyncOperation(command, status string, startTime time.Time) {
	op := SyncOperation{
		FileName:    command,
		Status:      status,
		ElapsedTime: time.Since(startTime),
	}
	v.syncOps = append(v.syncOps, op)

	// Keep only the last 5 operations
	if len(v.syncOps) > 5 {
		v.syncOps = v.syncOps[len(v.syncOps)-5:]
	}
}

// updateSyncOperation updates the status of a sync operation
func (v *UnifiedView) updateSyncOperation(command, status string, duration time.Duration) {
	for i := range v.syncOps {
		if v.syncOps[i].FileName == command {
			v.syncOps[i].Status = status
			if duration > 0 {
				v.syncOps[i].ElapsedTime = duration
			}
			break
		}
	}
}

// addOrUpdateSyncOperation adds a new operation or updates existing one
func (v *UnifiedView) addOrUpdateSyncOperation(fileName, status string) {
	// Check if operation already exists
	for i := range v.syncOps {
		if v.syncOps[i].FileName == fileName {
			// Update status
			v.syncOps[i].Status = status
			// Calculate elapsed time properly
			if v.syncOps[i].StartTime.IsZero() {
				v.syncOps[i].StartTime = time.Now()
			}
			v.syncOps[i].ElapsedTime = time.Since(v.syncOps[i].StartTime)
			// Update progress based on status
			if status == "Completed" {
				v.syncOps[i].Progress = 1.0
			} else if strings.Contains(status, "Pulling") {
				v.syncOps[i].Progress = 0.3
			} else {
				v.syncOps[i].Progress = 0.6
			}
			return
		}
	}

	// Add new operation
	op := SyncOperation{
		FileName:    fileName,
		Status:      status,
		StartTime:   time.Now(),
		ElapsedTime: 0,
		Progress:    0.3,
	}
	v.syncOps = append(v.syncOps, op)

	// Keep only the last 10 operations for better visibility
	if len(v.syncOps) > 10 {
		v.syncOps = v.syncOps[len(v.syncOps)-10:]
	}
}

// updateFileStatus updates the status of a file in the list
func (v *UnifiedView) updateFileStatus(fileName string, status SyncStatus) {
	items := v.fileList.Items()
	for i, item := range items {
		if fileItem, ok := item.(FileItem); ok {
			if fileItem.Name == fileName || strings.HasSuffix(fileItem.Name, fileName) {
				fileItem.Status = status
				fileItem.Desc = "Syncing..."
				items[i] = fileItem
			}
		}
	}
	v.fileList.SetItems(items)
}

// refreshFileList refreshes the file list from the file system
func (v *UnifiedView) refreshFileList() tea.Cmd {
	return func() tea.Msg {
		if v.config != nil {
			scanner := NewFileScanner(v.config)
			if items, err := scanner.ScanFiles(); err == nil {
				return FileListRefreshedMsg{Items: items}
			}
		}
		return nil
	}
}

// updateStats updates file statistics based on the file list
func (v *UnifiedView) updateStats(items []list.Item) {
	v.stats.synced = 0
	v.stats.pending = 0
	v.stats.errors = 0

	for _, item := range items {
		if fileItem, ok := item.(FileItem); ok {
			switch fileItem.Status {
			case StatusSynced:
				v.stats.synced++
			case StatusPending:
				v.stats.pending++
			case StatusError:
				v.stats.errors++
			case StatusModified:
				v.stats.pending++
			}
		}
	}
}

// FileListRefreshedMsg is sent when the file list has been refreshed
type FileListRefreshedMsg struct {
	Items []list.Item
}

// clearErrorAfterDelay returns a command that clears the error message after a delay
func (v *UnifiedView) clearErrorAfterDelay() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return ClearErrorMsg{}
	})
}

// ClearErrorMsg is sent to clear error messages
type ClearErrorMsg struct{}

// executeInitDirectly runs the init command without needing an executor
func executeInitDirectly() tea.Msg {
	startTime := time.Now()

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
		Message:  "Project initialized! Press 'c' to configure your Notion credentials",
	}
}
