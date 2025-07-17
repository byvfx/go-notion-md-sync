package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchModel represents the search view
type SearchModel struct {
	width  int
	height int
}

// NewSearchModel creates a new search model
func NewSearchModel() SearchModel {
	return SearchModel{}
}

// Init implements tea.Model
func (m SearchModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	return m, nil
}

// View implements tea.Model
func (m SearchModel) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Align(lipgloss.Center, lipgloss.Center).
		Width(m.width).
		Height(m.height)

	return style.Render("🔍 Search View\n\n(Coming soon)")
}

// SetSize updates the model size
func (m *SearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}