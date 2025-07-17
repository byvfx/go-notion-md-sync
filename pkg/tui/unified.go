package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UnifiedView represents the main split-pane view from the mockup
type UnifiedView struct {
	fileList      list.Model
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

	l := list.New(items, newCompactDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	// Mock sync operation
	syncOps := []SyncOperation{
		{
			FileName:    "Table Page.md",
			Status:      "Converting table blocks",
			Progress:    0.6,
			ElapsedTime: time.Duration(2300) * time.Millisecond, // 2.3s
		},
	}

	return UnifiedView{
		fileList:      l,
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
			today:   15,
		},
		selectedFiles: make(map[string]bool),
		focusedPane:   0,
	}
}

// Init implements tea.Model
func (v UnifiedView) Init() tea.Cmd {
	return nil
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
			// TODO: Implement sync
			return v, nil

		case "c":
			// Open config
			// TODO: Switch to config view
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
	if v.width == 0 || v.height == 0 {
		return "Loading..."
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

	connectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	// Build header
	title := titleStyle.Render("notion-md-sync v1.0")
	connection := connectionStyle.Render(fmt.Sprintf("⚡ Connected to: %s", v.workspaceName))
	headerContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		strings.Repeat(" ", v.width-lipgloss.Width(title)-lipgloss.Width(connection)-4),
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
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1).
		Render("💡 Press 's' to sync, 'c' to configure, 'h' for help, 'q' to quit")

	// Combine all parts
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		panes,
		footer,
	)

	// Apply border to entire UI
	return borderStyle.Width(v.width - 2).Height(v.height - 2).Render(content)
}

// updatePaneSizes calculates pane dimensions
func (v *UnifiedView) updatePaneSizes() {
	// Account for borders and padding
	contentWidth := v.width - 4
	contentHeight := v.height - 6 // Header, footer, borders

	// Split width evenly
	paneWidth := contentWidth / 2

	// Update file list size
	v.fileList.SetSize(paneWidth - 4, contentHeight - 4)
}

// renderPane renders a single pane with title and content
func (v UnifiedView) renderPane(title, content, stats string, focused bool) string {
	paneWidth := (v.width - 4) / 2
	paneHeight := v.height - 6

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
	// Update selection indicators
	items := v.fileList.Items()
	for i, item := range items {
		if fileItem, ok := item.(FileItem); ok {
			// Show selection with arrow
			if v.selectedFiles[fileItem.Name] {
				fileItem.Name = "› " + strings.TrimPrefix(fileItem.Name, "› ")
			} else {
				fileItem.Name = "  " + strings.TrimPrefix(fileItem.Name, "› ")
			}
			items[i] = fileItem
		}
	}
	v.fileList.SetItems(items)

	return v.fileList.View()
}

// renderSyncStatus renders the sync status content
func (v UnifiedView) renderSyncStatus() string {
	if len(v.syncOps) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(1)
		return emptyStyle.Render("No sync operations in progress")
	}

	var content []string
	for _, op := range v.syncOps {
		// Operation header
		header := fmt.Sprintf("⏳ Syncing %s...", op.FileName)
		content = append(content, header)

		// Status tree
		content = append(content, "├─ "+op.Status)
		content = append(content, "├─ Uploading to Notion")
		content = append(content, fmt.Sprintf("└─ %.1fs elapsed", op.ElapsedTime.Seconds()))
	}

	return lipgloss.NewStyle().Padding(1).Render(strings.Join(content, "\n"))
}

// compactDelegate is a minimal list delegate for the file list
type compactDelegate struct{}

func newCompactDelegate() list.ItemDelegate {
	return compactDelegate{}
}

func (d compactDelegate) Height() int { return 1 }

func (d compactDelegate) Spacing() int { return 0 }

func (d compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(FileItem)
	if !ok {
		return
	}

	statusIcon := i.getStatusIcon()
	fileIcon := "📄"
	if i.IsDirectory {
		fileIcon = "📁"
	}

	str := fmt.Sprintf("%s %s %s", fileIcon, i.Name, statusIcon)

	if index == m.Index() {
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
		_, _ = fmt.Fprint(w, selectedStyle.Render(str))
	} else {
		_, _ = fmt.Fprint(w, str)
	}
}