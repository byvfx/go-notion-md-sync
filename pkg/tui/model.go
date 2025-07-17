package tui

import (
	"github.com/charmbracelet/bubbletea"
)

// ViewType represents different views in the TUI
type ViewType int

const (
	UnifiedViewType ViewType = iota
	ConfigurationView
	SearchView
)

// Model represents the main TUI application model
type Model struct {
	// Current view being displayed
	currentView ViewType

	// Main unified view (mockup style)
	unified UnifiedView

	// Additional views
	config ConfigModel
	search SearchModel

	// Window dimensions
	width  int
	height int

	// Global state
	configPath string
}

// NewModel creates a new TUI model
func NewModel(configPath string) Model {
	return Model{
		currentView: UnifiedViewType,
		unified:     NewUnifiedView(),
		config:      NewConfigModel(),
		search:      NewSearchModel(),
		configPath:  configPath,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return m.unified.Init()
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		// View switching
		case "u":
			m.currentView = UnifiedViewType
		case "c":
			m.currentView = ConfigurationView

		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Pass window size to unified view
		var newUnified tea.Model
		newUnified, _ = m.unified.Update(msg)
		m.unified = newUnified.(UnifiedView)
		m.config.SetSize(msg.Width, msg.Height)
		m.search.SetSize(msg.Width, msg.Height)
	}

	// Update the current view's model
	var cmd tea.Cmd
	switch m.currentView {
	case UnifiedViewType:
		var newUnified tea.Model
		newUnified, cmd = m.unified.Update(msg)
		m.unified = newUnified.(UnifiedView)
	case ConfigurationView:
		m.config, cmd = m.config.Update(msg)
	case SearchView:
		m.search, cmd = m.search.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m Model) View() string {
	// Render the current view
	switch m.currentView {
	case UnifiedViewType:
		return m.unified.View()
	case ConfigurationView:
		return m.config.View()
	case SearchView:
		return m.search.View()
	default:
		return m.unified.View()
	}
}

