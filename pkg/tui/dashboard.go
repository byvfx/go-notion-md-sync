package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardStats holds statistics for the dashboard
type DashboardStats struct {
	TotalFiles      int
	SyncedFiles     int
	PendingFiles    int
	ErrorFiles      int
	ConflictFiles   int
	LastSyncTime    time.Time
	TotalSyncTime   time.Duration
	SyncToday       int
	SyncThisWeek    int
	WorkspaceStatus string
	APICallsToday   int
	APIRateLimit    int
}

// DashboardModel represents the dashboard view
type DashboardModel struct {
	stats         DashboardStats
	width         int
	height        int
	lastRefresh   time.Time
	isConnected   bool
	workspaceName string
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel() DashboardModel {
	return DashboardModel{
		stats: DashboardStats{
			TotalFiles:      42,
			SyncedFiles:     38,
			PendingFiles:    3,
			ErrorFiles:      1,
			ConflictFiles:   0,
			LastSyncTime:    time.Now().Add(-2 * time.Hour),
			SyncToday:       15,
			SyncThisWeek:    87,
			WorkspaceStatus: "Connected",
			APICallsToday:   234,
			APIRateLimit:    1000,
		},
		lastRefresh:   time.Now(),
		isConnected:   true,
		workspaceName: "My Notion Workspace",
	}
}

// Init implements tea.Model
func (m DashboardModel) Init() tea.Cmd {
	// Refresh stats periodically
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return refreshStatsMsg{}
	})
}

// Update implements tea.Model
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshStatsMsg:
		// TODO: Fetch real stats
		m.lastRefresh = time.Now()
		// Continue refreshing
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return refreshStatsMsg{}
		})

	case dashboardStatsMsg:
		m.stats = msg.stats
		m.lastRefresh = time.Now()
	}

	return m, nil
}

// View implements tea.Model
func (m DashboardModel) View() string {
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	valueStyle := lipgloss.NewStyle().
		Bold(true)

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	// Header
	header := titleStyle.Render("📊 notion-md-sync Dashboard")

	// Connection status
	var connectionStatus string
	if m.isConnected {
		connectionStatus = successStyle.Render("⚡ Connected to: " + m.workspaceName)
	} else {
		connectionStatus = errorStyle.Render("⚠️ Disconnected")
	}

	// File statistics section
	fileStats := fmt.Sprintf(
		"%s\n\n"+
			"%s %s  %s %s  %s %s\n"+
			"%s %s  %s %s",
		labelStyle.Render("📁 File Statistics"),
		labelStyle.Render("Total:"),
		valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalFiles)),
		labelStyle.Render("Synced:"),
		successStyle.Render(fmt.Sprintf("%d", m.stats.SyncedFiles)),
		labelStyle.Render("Pending:"),
		warningStyle.Render(fmt.Sprintf("%d", m.stats.PendingFiles)),
		labelStyle.Render("Errors:"),
		errorStyle.Render(fmt.Sprintf("%d", m.stats.ErrorFiles)),
		labelStyle.Render("Conflicts:"),
		warningStyle.Render(fmt.Sprintf("%d", m.stats.ConflictFiles)),
	)

	// Sync activity section
	syncActivity := fmt.Sprintf(
		"%s\n\n"+
			"%s %s\n"+
			"%s %s\n"+
			"%s %s",
		labelStyle.Render("🔄 Sync Activity"),
		labelStyle.Render("Last sync:"),
		valueStyle.Render(formatTimeSince(m.stats.LastSyncTime)),
		labelStyle.Render("Today:"),
		valueStyle.Render(fmt.Sprintf("%d files", m.stats.SyncToday)),
		labelStyle.Render("This week:"),
		valueStyle.Render(fmt.Sprintf("%d files", m.stats.SyncThisWeek)),
	)

	// API usage section
	apiUsagePercent := float64(m.stats.APICallsToday) / float64(m.stats.APIRateLimit) * 100
	var apiUsageStyle lipgloss.Style
	if apiUsagePercent > 80 {
		apiUsageStyle = errorStyle
	} else if apiUsagePercent > 50 {
		apiUsageStyle = warningStyle
	} else {
		apiUsageStyle = successStyle
	}

	apiUsage := fmt.Sprintf(
		"%s\n\n"+
			"%s %s\n"+
			"%s %s",
		labelStyle.Render("🌐 API Usage"),
		labelStyle.Render("Calls today:"),
		apiUsageStyle.Render(fmt.Sprintf("%d/%d (%.0f%%)",
			m.stats.APICallsToday,
			m.stats.APIRateLimit,
			apiUsagePercent)),
		labelStyle.Render("Status:"),
		successStyle.Render("Healthy"),
	)

	// Quick actions
	quickActions := fmt.Sprintf(
		"%s\n\n"+
			"%s Sync all files\n"+
			"%s Pull from Notion\n"+
			"%s Push to Notion\n"+
			"%s Open configuration",
		labelStyle.Render("⚡ Quick Actions"),
		labelStyle.Render("[s]"),
		labelStyle.Render("[p]"),
		labelStyle.Render("[u]"),
		labelStyle.Render("[c]"),
	)

	// Layout the sections
	fileStatsBox := sectionStyle.Render(fileStats)
	syncActivityBox := sectionStyle.Render(syncActivity)
	apiUsageBox := sectionStyle.Render(apiUsage)
	quickActionsBox := sectionStyle.Render(quickActions)

	// Combine sections side by side
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		fileStatsBox,
		"  ",
		syncActivityBox,
	)

	bottomRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		apiUsageBox,
		"  ",
		quickActionsBox,
	)

	// Status bar
	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		Render(fmt.Sprintf("Last refresh: %s", formatTimeSince(m.lastRefresh)))

	// Combine all elements
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		connectionStatus,
		"",
		topRow,
		bottomRow,
		statusBar,
	)
}

// SetSize updates the model size
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// refreshStatsMsg triggers a stats refresh
type refreshStatsMsg struct{}

// dashboardStatsMsg updates dashboard statistics
type dashboardStatsMsg struct {
	stats DashboardStats
}

// formatTimeSince formats a time as "X ago"
func formatTimeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
