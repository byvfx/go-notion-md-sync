package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfigInputModel handles interactive configuration setup
type ConfigInputModel struct {
	width    int
	height   int
	inputs   []textinput.Model
	focused  int
	err      error
	complete bool
}

// Input field indices
const (
	tokenInput = iota
	pageIDInput
)

// NewConfigInputModel creates a new config input model
func NewConfigInputModel() ConfigInputModel {
	m := ConfigInputModel{
		inputs: make([]textinput.Model, 2),
	}

	// Setup token input
	m.inputs[tokenInput] = textinput.New()
	m.inputs[tokenInput].Placeholder = "Enter your Notion Integration Token"
	m.inputs[tokenInput].Focus()
	m.inputs[tokenInput].CharLimit = 200
	m.inputs[tokenInput].Width = 80
	m.inputs[tokenInput].EchoMode = textinput.EchoPassword

	// Setup page ID input
	m.inputs[pageIDInput] = textinput.New()
	m.inputs[pageIDInput].Placeholder = "Enter your Notion Parent Page ID"
	m.inputs[pageIDInput].CharLimit = 100
	m.inputs[pageIDInput].Width = 80

	return m
}

// Init implements tea.Model
func (m ConfigInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m ConfigInputModel) Update(msg tea.Msg) (ConfigInputModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, save the configuration
			if s == "enter" && m.focused == len(m.inputs) {
				return m, m.saveConfig()
			}

			// Cycle between inputs
			if s == "up" || s == "shift+tab" {
				m.focused--
			} else {
				m.focused++
			}

			if m.focused > len(m.inputs) {
				m.focused = 0
			} else if m.focused < 0 {
				m.focused = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focused {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
					m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = lipgloss.NewStyle()
				m.inputs[i].TextStyle = lipgloss.NewStyle()
			}

			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case ConfigSavedMsg:
		m.complete = true
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tea.Quit
		})

	case ConfigErrorMsg:
		m.err = msg.Error
		return m, nil
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *ConfigInputModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

// View implements tea.Model
func (m ConfigInputModel) View() string {
	if m.complete {
		return m.completionView()
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(1, 0)

	header := headerStyle.Render("🔧 Configure Notion Integration")

	// Instructions
	instructionsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 0, 1, 0)

	instructions := instructionsStyle.Render(
		"Enter your Notion credentials to complete setup.\n" +
			"Get your integration token from: https://www.notion.so/my-integrations\n" +
			"Get your page ID from the Notion page URL (the long string after the last /)",
	)

	// Form
	var inputs []string
	inputs = append(inputs, "Notion Integration Token:")
	inputs = append(inputs, m.inputs[tokenInput].View())
	inputs = append(inputs, "")
	inputs = append(inputs, "Parent Page ID:")
	inputs = append(inputs, m.inputs[pageIDInput].View())

	// Submit button
	var button string
	if m.focused == len(m.inputs) {
		button = lipgloss.NewStyle().
			Background(lipgloss.Color("205")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 3).
			Render("[ Save Configuration ]")
	} else {
		button = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 3).
			Render("[ Save Configuration ]")
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 0, 0, 0)

	footer := footerStyle.Render("Tab/Enter: Next • Esc: Cancel")

	// Error display
	var errorMsg string
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Padding(1, 0)
		errorMsg = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Combine all parts
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		instructions,
		strings.Join(inputs, "\n"),
		"",
		button,
		errorMsg,
		footer,
	)

	// Center the content
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Render(content),
		)
	}

	return content
}

func (m ConfigInputModel) completionView() string {
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		Padding(1, 0)

	message := successStyle.Render("✅ Configuration saved successfully!")

	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 0)

	details := detailStyle.Render(
		"Your Notion credentials have been saved to .env\n" +
			"You can now use sync commands to connect with Notion!",
	)

	content := lipgloss.JoinVertical(lipgloss.Left, message, details)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			content,
		)
	}

	return content
}

// saveConfig saves the configuration to .env file
func (m ConfigInputModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		token := strings.TrimSpace(m.inputs[tokenInput].Value())
		pageID := strings.TrimSpace(m.inputs[pageIDInput].Value())

		if token == "" {
			return ConfigErrorMsg{Error: fmt.Errorf("token cannot be empty")}
		}

		if pageID == "" {
			return ConfigErrorMsg{Error: fmt.Errorf("parent page ID cannot be empty")}
		}

		// Create .env content
		envContent := fmt.Sprintf(`# notion-md-sync environment variables
# Generated by TUI configuration
NOTION_MD_SYNC_NOTION_TOKEN=%s
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=%s
`, token, pageID)

		// Write .env file
		if err := os.WriteFile(".env", []byte(envContent), 0600); err != nil {
			return ConfigErrorMsg{Error: fmt.Errorf("failed to write .env file: %w", err)}
		}

		return ConfigSavedMsg{}
	}
}

// SetSize sets the model dimensions
func (m *ConfigInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ConfigSavedMsg indicates configuration was saved successfully
type ConfigSavedMsg struct{}

// ConfigErrorMsg indicates an error saving configuration
type ConfigErrorMsg struct {
	Error error
}
