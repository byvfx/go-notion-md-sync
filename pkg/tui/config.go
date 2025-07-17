package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfigModel represents the configuration view
type ConfigModel struct {
	width  int
	height int
}

// NewConfigModel creates a new config model
func NewConfigModel() ConfigModel {
	return ConfigModel{}
}

// Init implements tea.Model
func (m ConfigModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ConfigModel) Update(msg tea.Msg) (ConfigModel, tea.Cmd) {
	return m, nil
}

// View implements tea.Model
func (m ConfigModel) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Align(lipgloss.Center, lipgloss.Center).
		Width(m.width).
		Height(m.height)

	return style.Render("⚙️ Configuration View\n\n(Coming soon)")
}

// SetSize updates the model size
func (m *ConfigModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}