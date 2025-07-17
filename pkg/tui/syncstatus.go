package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SyncOperation represents a sync operation in progress
type SyncOperation struct {
	ID          string
	FileName    string
	Operation   string // "pull", "push", "conflict"
	Progress    float64
	Status      string
	StartTime   time.Time
	ElapsedTime time.Duration
	Error       error
}

// SyncStatusModel represents the sync status view
type SyncStatusModel struct {
	operations   []SyncOperation
	spinner      spinner.Model
	progressBars map[string]progress.Model
	width        int
	height       int
	isActive     bool
}

// NewSyncStatusModel creates a new sync status model
func NewSyncStatusModel() SyncStatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return SyncStatusModel{
		operations:   make([]SyncOperation, 0),
		spinner:      s,
		progressBars: make(map[string]progress.Model),
		isActive:     false,
	}
}

// Init implements tea.Model
func (m SyncStatusModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model
func (m SyncStatusModel) Update(msg tea.Msg) (SyncStatusModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			// Cancel current operations
			// TODO: Implement cancellation
		case "p":
			// Pause/resume operations
			m.isActive = !m.isActive
		}

	case syncOperationMsg:
		// Update or add sync operation
		found := false
		for i, op := range m.operations {
			if op.ID == msg.operation.ID {
				m.operations[i] = msg.operation
				found = true
				break
			}
		}
		if !found {
			m.operations = append(m.operations, msg.operation)
			// Create progress bar for new operation
			p := progress.New(progress.WithDefaultGradient())
			p.Width = m.width - 40
			m.progressBars[msg.operation.ID] = p
		}

		// Update progress bar
		if p, ok := m.progressBars[msg.operation.ID]; ok {
			cmd := p.SetPercent(msg.operation.Progress)
			cmds = append(cmds, cmd)
		}

	case syncCompleteMsg:
		// Remove completed operation
		for i, op := range m.operations {
			if op.ID == msg.operationID {
				m.operations = append(m.operations[:i], m.operations[i+1:]...)
				delete(m.progressBars, msg.operationID)
				break
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		// Update all progress bars
		for id, p := range m.progressBars {
			newModel, cmd := p.Update(msg)
			m.progressBars[id] = newModel.(progress.Model)
			cmds = append(cmds, cmd)
		}
	}

	// Continue spinner animation if active
	if m.isActive && len(m.operations) > 0 {
		cmds = append(cmds, m.spinner.Tick)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m SyncStatusModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	operationStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	fileNameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	view := titleStyle.Render("🔄 Sync Status") + "\n"

	if len(m.operations) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			PaddingLeft(2)
		view += emptyStyle.Render("No sync operations in progress")
	} else {
		for _, op := range m.operations {
			// Operation header with spinner
			header := fmt.Sprintf("%s %s %s",
				m.spinner.View(),
				getOperationIcon(op.Operation),
				fileNameStyle.Render(op.FileName))
			view += operationStyle.Render(header) + "\n"

			// Progress bar
			if p, ok := m.progressBars[op.ID]; ok {
				progressLine := fmt.Sprintf("├─ %s %.0f%%",
					p.View(),
					op.Progress*100)
				view += operationStyle.Render(progressLine) + "\n"
			}

			// Status and timing
			statusLine := fmt.Sprintf("├─ %s",
				statusStyle.Render(op.Status))
			view += operationStyle.Render(statusLine) + "\n"

			timingLine := fmt.Sprintf("└─ %s elapsed",
				statusStyle.Render(formatDuration(op.ElapsedTime)))
			view += operationStyle.Render(timingLine) + "\n"

			// Error if present
			if op.Error != nil {
				errorLine := fmt.Sprintf("   %s",
					errorStyle.Render(fmt.Sprintf("Error: %v", op.Error)))
				view += operationStyle.Render(errorLine) + "\n"
			}

			view += "\n"
		}
	}

	// Summary statistics
	summaryStyle := lipgloss.NewStyle().
		MarginTop(1).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		PaddingTop(1)

	summary := fmt.Sprintf("📈 Active operations: %d", len(m.operations))
	if !m.isActive && len(m.operations) > 0 {
		summary += " (Paused)"
	}
	view += summaryStyle.Render(summary)

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	help := "💡 p: pause/resume | c: cancel | q: back"
	view += "\n" + helpStyle.Render(help)

	return view
}

// SetSize updates the model size
func (m *SyncStatusModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Update progress bar widths
	for id, p := range m.progressBars {
		p.Width = width - 40
		m.progressBars[id] = p
	}
}

// AddOperation adds a new sync operation
func (m *SyncStatusModel) AddOperation(op SyncOperation) tea.Cmd {
	return func() tea.Msg {
		return syncOperationMsg{operation: op}
	}
}

// UpdateOperation updates an existing operation
func (m *SyncStatusModel) UpdateOperation(op SyncOperation) tea.Cmd {
	return func() tea.Msg {
		return syncOperationMsg{operation: op}
	}
}

// CompleteOperation marks an operation as complete
func (m *SyncStatusModel) CompleteOperation(operationID string) tea.Cmd {
	return func() tea.Msg {
		return syncCompleteMsg{operationID: operationID}
	}
}

// syncOperationMsg is sent when a sync operation is updated
type syncOperationMsg struct {
	operation SyncOperation
}

// syncCompleteMsg is sent when a sync operation completes
type syncCompleteMsg struct {
	operationID string
}

// getOperationIcon returns an icon for the operation type
func getOperationIcon(operation string) string {
	switch operation {
	case "pull":
		return "⬇️"
	case "push":
		return "⬆️"
	case "conflict":
		return "⚠️"
	default:
		return "🔄"
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}