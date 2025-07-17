package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	configPath := "/test/config.yaml"
	model := NewModel(configPath)

	if model.configPath != configPath {
		t.Errorf("Expected configPath %s, got %s", configPath, model.configPath)
	}

	if model.currentView != UnifiedViewType {
		t.Errorf("Expected default view to be UnifiedViewType, got %v", model.currentView)
	}
}

func TestModelViewSwitching(t *testing.T) {
	model := NewModel("/test/config.yaml")

	tests := []struct {
		key          string
		expectedView ViewType
	}{
		{"u", UnifiedViewType},
		{"c", ConfigurationView},
	}

	for _, test := range tests {
		// Create key message
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.key)}
		
		// Update model
		updatedModel, _ := model.Update(msg)
		model = updatedModel.(Model)

		if model.currentView != test.expectedView {
			t.Errorf("Key %s: expected view %v, got %v", test.key, test.expectedView, model.currentView)
		}
	}
}

func TestModelQuit(t *testing.T) {
	model := NewModel("/test/config.yaml")

	// Test 'q' key
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	_, cmd := model.Update(qMsg)

	// Check if quit command was returned
	if cmd == nil {
		t.Error("Expected quit command to be returned")
	}

	// Test Ctrl+C
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd = model.Update(ctrlCMsg)

	if cmd == nil {
		t.Error("Expected quit command to be returned for Ctrl+C")
	}
}

func TestModelWindowSize(t *testing.T) {
	model := NewModel("/test/config.yaml")
	width, height := 100, 50

	// Send window size message
	msg := tea.WindowSizeMsg{Width: width, Height: height}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	if model.width != width {
		t.Errorf("Expected width %d, got %d", width, model.width)
	}

	if model.height != height {
		t.Errorf("Expected height %d, got %d", height, model.height)
	}
}