package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SyncStatus represents the sync status of a file
type SyncStatus int

const (
	StatusSynced SyncStatus = iota
	StatusPending
	StatusModified
	StatusError
	StatusConflict
)

// FileItem represents a file in the list
type FileItem struct {
	Path         string
	Name         string
	Status       SyncStatus
	LastSync     string
	Size         int64
	IsDirectory  bool
	Desc         string  // Renamed from Description to avoid conflict
}

// FilterValue implements list.Item
func (i FileItem) FilterValue() string {
	return i.Name
}

// Title implements list.DefaultItem
func (i FileItem) Title() string {
	icon := i.getStatusIcon()
	if i.IsDirectory {
		return fmt.Sprintf("%s 📁 %s", icon, i.Name)
	}
	return fmt.Sprintf("%s 📄 %s", icon, i.Name)
}

// Description implements list.DefaultItem
func (i FileItem) Description() string {
	return i.Desc
}

// getStatusIcon returns an icon based on sync status
func (i FileItem) getStatusIcon() string {
	switch i.Status {
	case StatusSynced:
		return "✅"
	case StatusPending:
		return "⏳"
	case StatusModified:
		return "🔄"
	case StatusError:
		return "❌"
	case StatusConflict:
		return "⚠️"
	default:
		return "❓"
	}
}

// FileListModel represents the file browser view
type FileListModel struct {
	list          list.Model
	files         []FileItem
	currentPath   string
	width         int
	height        int
	selectedFiles map[string]bool
}

// NewFileListModel creates a new file list model
func NewFileListModel() FileListModel {
	// Create initial file items (mock data for now)
	items := []list.Item{
		FileItem{
			Path:   "README.md",
			Name:   "README.md",
			Status: StatusSynced,
			Desc:   "Last synced: 2 hours ago",
		},
		FileItem{
			Path:   "docs/guide.md",
			Name:   "guide.md",
			Status: StatusModified,
			Desc:   "Modified locally",
		},
		FileItem{
			Path:   "docs/api.md",
			Name:   "api.md",
			Status: StatusError,
			Desc:   "Sync error: API rate limit",
		},
	}

	// Configure list
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "📁 Files"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Custom styles
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true).
		Padding(0, 1)

	return FileListModel{
		list:          l,
		currentPath:   ".",
		selectedFiles: make(map[string]bool),
	}
}

// Init implements tea.Model
func (m FileListModel) Init() tea.Cmd {
	return m.loadFiles(".")
}

// Update implements tea.Model
func (m FileListModel) Update(msg tea.Msg) (FileListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Handle file/directory selection
			if i, ok := m.list.SelectedItem().(FileItem); ok {
				if i.IsDirectory {
					// Navigate into directory
					newPath := filepath.Join(m.currentPath, i.Name)
					return m, m.loadFiles(newPath)
				} else {
					// Toggle file selection
					m.selectedFiles[i.Path] = !m.selectedFiles[i.Path]
				}
			}
		case "backspace":
			// Navigate to parent directory
			if m.currentPath != "." {
				parentPath := filepath.Dir(m.currentPath)
				return m, m.loadFiles(parentPath)
			}
		case " ":
			// Toggle selection of current item
			if i, ok := m.list.SelectedItem().(FileItem); ok && !i.IsDirectory {
				m.selectedFiles[i.Path] = !m.selectedFiles[i.Path]
			}
		case "a":
			// Select all files
			for _, item := range m.files {
				if !item.IsDirectory {
					m.selectedFiles[item.Path] = true
				}
			}
		case "n":
			// Deselect all files
			m.selectedFiles = make(map[string]bool)
		}

	case filesLoadedMsg:
		m.files = msg.files
		items := make([]list.Item, len(msg.files))
		for i, f := range msg.files {
			items[i] = f
		}
		m.list.SetItems(items)
		m.currentPath = msg.path
		m.list.Title = fmt.Sprintf("📁 %s", m.currentPath)
	}

	// Update list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m FileListModel) View() string {
	// Add selection indicator to items
	items := m.list.Items()
	for i, item := range items {
		if fileItem, ok := item.(FileItem); ok {
			if m.selectedFiles[fileItem.Path] {
				// Add selection marker
				fileItem.Desc = "✓ " + fileItem.Desc
				items[i] = fileItem
			}
		}
	}
	m.list.SetItems(items)

	// Build status bar
	selectedCount := len(m.selectedFiles)
	statusBar := fmt.Sprintf("\n📊 %d files selected", selectedCount)
	
	help := "\n💡 Space: select | Enter: open | a: select all | n: select none | s: sync selected"

	return m.list.View() + statusBar + help
}

// SetSize updates the model size
func (m *FileListModel) SetSize(width, height int) {
	m.width = width
	m.height = height - 4 // Account for status bar and help
	m.list.SetSize(width, m.height)
}

// GetSelectedFiles returns the currently selected files
func (m FileListModel) GetSelectedFiles() []string {
	var selected []string
	for path, isSelected := range m.selectedFiles {
		if isSelected {
			selected = append(selected, path)
		}
	}
	return selected
}

// filesLoadedMsg is sent when files are loaded
type filesLoadedMsg struct {
	path  string
	files []FileItem
}

// loadFiles loads files from a directory
func (m FileListModel) loadFiles(path string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual file loading from filesystem
		// For now, return mock data
		
		files := []FileItem{
			{
				Path:        filepath.Join(path, ".."),
				Name:        "..",
				IsDirectory: true,
				Desc:        "Parent directory",
			},
		}

		// Add mock files based on path
		if strings.Contains(path, "docs") {
			files = append(files, []FileItem{
				{
					Path:   filepath.Join(path, "guide.md"),
					Name:   "guide.md",
					Status: StatusModified,
					Desc:   "Modified locally",
				},
				{
					Path:   filepath.Join(path, "api.md"),
					Name:   "api.md",
					Status: StatusError,
					Desc:   "Sync error",
				},
			}...)
		} else {
			files = append(files, []FileItem{
				{
					Path:        filepath.Join(path, "docs"),
					Name:        "docs",
					IsDirectory: true,
					Desc:        "Documentation",
				},
				{
					Path:   filepath.Join(path, "README.md"),
					Name:   "README.md",
					Status: StatusSynced,
					Desc:   "Last synced: 2 hours ago",
				},
			}...)
		}

		return filesLoadedMsg{
			path:  path,
			files: files,
		}
	}
}