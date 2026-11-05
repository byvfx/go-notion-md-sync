package tui

import (
	"fmt"
	"log"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/charmbracelet/bubbletea"
)

// ViewType represents different views in the TUI
type ViewType int

const (
	UnifiedViewType ViewType = iota
	ConfigurationView
	SearchView
	ConfigInputView
)

// Model represents the main TUI application model
type Model struct {
	// Current view being displayed
	currentView ViewType

	// Main unified view (mockup style)
	unified UnifiedView

	// Additional views
	config      ConfigModel
	search      SearchModel
	configInput ConfigInputModel

	// Window dimensions
	width  int
	height int

	// Global state
	configPath string
	appConfig  *config.Config
	executor   *CommandExecutor

	// Error state
	initError error
}

// NewModel creates a new TUI model
func NewModel(configPath string) Model {
	m := Model{
		currentView: UnifiedViewType,
		config:      NewConfigModel(),
		search:      NewSearchModel(),
		configInput: NewConfigInputModel(),
		configPath:  configPath,
	}

	// Load configuration
	appConfig, err := config.Load(configPath)
	if err != nil {
		m.initError = fmt.Errorf("failed to load config: %w", err)
		m.unified = NewUnifiedView()
		return m
	}
	m.appConfig = appConfig

	// Create command executor
	executor, err := NewCommandExecutor(appConfig)
	if err != nil {
		m.initError = fmt.Errorf("failed to create command executor: %w", err)
		m.unified = NewUnifiedView()
		return m
	}
	m.executor = executor

	// Create unified view with config and executor
	m.unified = NewUnifiedViewWithConfig(appConfig, executor)

	// If there was an init error, show it in the unified view
	if m.initError != nil {
		m.unified.errorMessage = m.initError.Error()
		m.unified.lastError = m.initError
	}

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	if m.initError != nil {
		// Log the error but continue with limited functionality
		log.Printf("TUI initialization warning: %v", m.initError)
	}
	return m.unified.Init()
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// While the config form is open every key belongs to it (tokens can
		// contain 'q', 'u', 'c', ...). Only ctrl+c and esc stay global.
		if m.currentView == ConfigInputView {
			switch msg.String() {
			case "ctrl+c":
				return m.quit()
			case "esc":
				m.currentView = UnifiedViewType
				return m, nil
			}
			var cmd tea.Cmd
			m.configInput, cmd = m.configInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m.quit()

		// View switching
		case "u":
			m.currentView = UnifiedViewType
			return m, nil
		case "c":
			// Switch to config input view instead of configuration view.
			// The key itself must not be typed into the form.
			m.currentView = ConfigInputView
			return m, m.configInput.Init()
		}

	case ConfigSavedMsg:
		// Configuration saved successfully: reload it and switch back to
		// the unified view so the new credentials take effect immediately.
		m.currentView = UnifiedViewType
		m.configInput = NewConfigInputModel()
		m.configInput.SetSize(m.width, m.height)
		return m, m.reloadConfig()

	case ConfigErrorMsg:
		// Handle configuration error - stay in config input view
		// The error will be displayed by the ConfigInputModel

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Pass window size to unified view
		var newUnified tea.Model
		newUnified, _ = m.unified.Update(msg)
		if u, ok := newUnified.(UnifiedView); ok {
			m.unified = u
		}
		m.config.SetSize(msg.Width, msg.Height)
		m.search.SetSize(msg.Width, msg.Height)
		m.configInput.SetSize(msg.Width, msg.Height)

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
	case ConfigInputView:
		m.configInput, cmd = m.configInput.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// quit releases resources and quits the program.
func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.executor != nil {
		m.executor.Close()
	}
	return m, tea.Quit
}

// reloadConfig loads the configuration again and recreates the executor and
// unified view, so changes saved from the config form apply without a
// restart.
func (m *Model) reloadConfig() tea.Cmd {
	appConfig, err := config.Load(m.configPath)
	if err != nil {
		m.initError = fmt.Errorf("failed to load config: %w", err)
		m.unified.errorMessage = m.initError.Error()
		m.unified.lastError = m.initError
		return m.unified.clearErrorAfterDelay()
	}

	executor, err := NewCommandExecutor(appConfig)
	if err != nil {
		m.initError = fmt.Errorf("failed to create command executor: %w", err)
		m.unified.errorMessage = m.initError.Error()
		m.unified.lastError = m.initError
		return m.unified.clearErrorAfterDelay()
	}

	if m.executor != nil {
		m.executor.Close()
	}
	m.appConfig = appConfig
	m.executor = executor
	m.initError = nil

	m.unified = NewUnifiedViewWithConfig(appConfig, executor)
	if m.width > 0 && m.height > 0 {
		if u, ok := m.unified.updateSize(m.width, m.height); ok {
			m.unified = u
		}
	}

	return m.unified.Init()
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
	case ConfigInputView:
		return m.configInput.View()
	default:
		return m.unified.View()
	}
}
