package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// TestModel_ConfigInputReceivesGlobalKeys is a regression test: typing a
// token containing 'q', 'u' or 'c' in the config form used to quit or
// switch views.
func TestModel_ConfigInputReceivesGlobalKeys(t *testing.T) {
	model := NewModel("/test/config.yaml")

	updated, _ := model.Update(keyRunes("c"))
	model = updated.(Model)
	if model.currentView != ConfigInputView {
		t.Fatalf("expected config input view, got %v", model.currentView)
	}
	if got := model.configInput.inputs[tokenInput].Value(); got != "" {
		t.Fatalf("the 'c' that opened the form was typed into it: %q", got)
	}

	// Global keys would have been swallowed (quit, view switch) instead of
	// reaching the input, so the typed value proves they were routed.
	for _, r := range "secret_quc" {
		updated, _ = model.Update(keyRunes(string(r)))
		model = updated.(Model)
		if model.currentView != ConfigInputView {
			t.Fatalf("key %q left the config view", r)
		}
	}

	if got := model.configInput.inputs[tokenInput].Value(); got != "secret_quc" {
		t.Errorf("token = %q, want %q", got, "secret_quc")
	}

	// esc leaves the form without quitting.
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.currentView != UnifiedViewType {
		t.Errorf("esc: expected unified view, got %v", model.currentView)
	}
	if isQuit(cmd) {
		t.Error("esc must not quit")
	}

	// ctrl+c still quits from the form.
	updated, _ = model.Update(keyRunes("c"))
	model = updated.(Model)
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuit(cmd) {
		t.Error("ctrl+c in config view should quit")
	}
}

// TestModel_ReloadsConfigAfterSave is a regression test: saving the config
// form must make the TUI usable without a restart.
func TestModel_ReloadsConfigAfterSave(t *testing.T) {
	t.Setenv("GO_TEST", "1") // don't read .env files
	t.Setenv("NOTION_MD_SYNC_NOTION_TOKEN", "")
	t.Setenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID", "")

	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	cfgYAML := "directories:\n  markdown_root: " + docs + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	model := NewModel(cfgPath)
	if model.executor != nil || model.initError == nil {
		t.Fatal("expected model without credentials to have no executor")
	}

	t.Setenv("NOTION_MD_SYNC_NOTION_TOKEN", "ntn_test")
	t.Setenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID", "parent-page")

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = updated.(Model)
	model.currentView = ConfigInputView

	updated, _ = model.Update(ConfigSavedMsg{})
	model = updated.(Model)
	defer func() {
		if model.executor != nil {
			model.executor.Close()
		}
	}()

	if model.currentView != UnifiedViewType {
		t.Errorf("expected unified view after save, got %v", model.currentView)
	}
	if model.initError != nil {
		t.Errorf("initError = %v, want nil", model.initError)
	}
	if model.executor == nil || model.unified.executor != model.executor {
		t.Fatal("executor not recreated after config save")
	}
	if model.appConfig == nil || model.appConfig.Notion.Token != "ntn_test" {
		t.Errorf("config not reloaded: %+v", model.appConfig)
	}
	if model.unified.width != 100 || model.unified.height != 40 {
		t.Errorf("unified view size = %dx%d, want 100x40", model.unified.width, model.unified.height)
	}
}
