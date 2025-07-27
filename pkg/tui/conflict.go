package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConflictModel represents the conflict resolution view
type ConflictModel struct {
	width  int
	height int
}

// NewConflictModel creates a new conflict model
func NewConflictModel() ConflictModel {
	return ConflictModel{}
}

// Init implements tea.Model
func (m ConflictModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ConflictModel) Update(msg tea.Msg) (ConflictModel, tea.Cmd) {
	return m, nil
}

// View implements tea.Model
func (m ConflictModel) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Align(lipgloss.Center, lipgloss.Center).
		Width(m.width).
		Height(m.height)

	return style.Render("⚠️ Conflict Resolution View\n\n(Coming soon)")
}

// SetSize updates the model size
func (m *ConflictModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
