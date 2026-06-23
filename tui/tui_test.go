package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialModel_defaults(t *testing.T) {
	m := InitialModel()
	if m.screen != screenAPIKey {
		t.Errorf("initial screen = %d, want %d", m.screen, screenAPIKey)
	}
	if m.cfg == nil {
		t.Fatal("cfg should not be nil")
	}
	if m.apiInput.Value() != "" {
		t.Errorf("apiInput should be empty, got %q", m.apiInput.Value())
	}
	if m.folderInput.Value() != "" {
		t.Errorf("folderInput should be empty, got %q", m.folderInput.Value())
	}
	if m.progress.Width == 0 {
		t.Error("progress should have Width set")
	}
	if m.exePath == "" {
		t.Error("exePath should not be empty")
	}
}

func TestInitialModel_cfgWithKey(t *testing.T) {
	// Simulate config with key by setting env to control config path
	orig := os.Getenv("WATERTOGO_CONFIG")
	defer os.Setenv("WATERTOGO_CONFIG", orig)
	m := InitialModel()
	if !m.apiInput.Focused() {
		t.Error("apiInput should be focused initially")
	}
}

func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestCtrlC_alwaysQuits(t *testing.T) {
	m := InitialModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if !isQuitCmd(cmd) {
		t.Error("Ctrl+C should always quit")
	}
}

func TestQ_onAPIKeyScreen_doesNotQuit(t *testing.T) {
	m := InitialModel()
	m.screen = screenAPIKey
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)
	if isQuitCmd(cmd) {
		t.Error("q should not quit on API key screen")
	}
}

func TestQ_onFolderSelectScreen_doesNotQuit(t *testing.T) {
	m := InitialModel()
	m.screen = screenFolderSelect
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)
	if isQuitCmd(cmd) {
		t.Error("q should not quit on folder select screen")
	}
}

func TestQ_onConvertingScreen_quits(t *testing.T) {
	m := InitialModel()
	m.screen = screenConverting
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)
	if !isQuitCmd(cmd) {
		t.Error("q should quit on converting screen")
	}
}

func TestEsc_fromFolderSelect_toAPIKey(t *testing.T) {
	m := InitialModel()
	m.screen = screenFolderSelect
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.screen != screenAPIKey {
		t.Errorf("esc should go to API key screen, got %d", m2.screen)
	}
}

func TestEnter_emptyAPIKey_noSavedKey_showsError(t *testing.T) {
	m := InitialModel()
	m.cfg.APIKeys = nil
	m.apiInput.SetValue("")
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.screen != screenAPIKey {
		t.Errorf("should stay on API key screen, got %d", m2.screen)
	}
	if m2.apiErr == "" {
		t.Error("should show error for empty API key")
	}
}

func TestWindowSize_setsWidthHeight(t *testing.T) {
	m := InitialModel()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.width != 120 {
		t.Errorf("width = %d, want 120", m2.width)
	}
	if m2.height != 40 {
		t.Errorf("height = %d, want 40", m2.height)
	}
}

func TestEnterOnDoneScreen_quits(t *testing.T) {
	m := InitialModel()
	m.screen = screenDone
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)
	if !isQuitCmd(cmd) {
		t.Error("Enter on done screen should quit")
	}
}

func TestResetConversion(t *testing.T) {
	m := InitialModel()
	m.convertErrors = []string{"err1", "err2"}
	m.convertedCount = 5
	m.failedCount = 2
	m.currentFile = 10
	m.totalFiles = 20
	m.logs = []string{"log1", "log2"}

	m = m.resetConversion()

	if len(m.convertErrors) != 0 {
		t.Error("convertErrors should be empty after reset")
	}
	if m.convertedCount != 0 {
		t.Error("convertedCount should be 0 after reset")
	}
	if m.failedCount != 0 {
		t.Error("failedCount should be 0 after reset")
	}
	if m.currentFile != 0 {
		t.Error("currentFile should be 0 after reset")
	}
	if m.totalFiles != 0 {
		t.Error("totalFiles should be 0 after reset")
	}
	if len(m.logs) != 0 {
		t.Error("logs should be empty after reset")
	}
}

func TestMinMax(t *testing.T) {
	if min(3, 7) != 3 {
		t.Error("min(3, 7) should be 3")
	}
	if min(7, 3) != 3 {
		t.Error("min(7, 3) should be 3")
	}
	if max(3, 7) != 7 {
		t.Error("max(3, 7) should be 7")
	}
	if max(7, 3) != 7 {
		t.Error("max(7, 3) should be 7")
	}
}

func TestCenterText(t *testing.T) {
	width := 20
	result := centerText("hello", width)
	if len(result) != width {
		t.Errorf("centerText len = %d, want %d", len(result), width)
	}
}

func TestRenderLogo(t *testing.T) {
	logo := renderLogo()
	if logo == "" {
		t.Error("logo should not be empty")
	}
	if len(logo) < 10 {
		t.Errorf("logo too short: %d", len(logo))
	}
}

func TestRenderFooter(t *testing.T) {
	footer := renderFooter()
	if footer == "" {
		t.Error("footer should not be empty")
	}
}

func TestAPIKeyView_renders(t *testing.T) {
	m := InitialModel()
	m.width = 80
	m.height = 24
	view := m.apiKeyView()
	if view == "" {
		t.Error("apiKeyView should not be empty")
	}
	if len(view) < 50 {
		t.Errorf("apiKeyView too short: %d", len(view))
	}
}

func TestFolderSelectView_renders(t *testing.T) {
	m := InitialModel()
	m.screen = screenFolderSelect
	m.width = 80
	m.height = 24
	view := m.folderSelectView()
	if view == "" {
		t.Error("folderSelectView should not be empty")
	}
	if len(view) < 50 {
		t.Errorf("folderSelectView too short: %d", len(view))
	}
}

func TestConvertingView_renders(t *testing.T) {
	m := InitialModel()
	m.screen = screenConverting
	m.width = 80
	m.height = 24
	m.totalFiles = 10
	m.currentFile = 3
	m.logs = []string{"✓ src/main.py"}
	view := m.convertingView()
	if view == "" {
		t.Error("convertingView should not be empty")
	}
	if len(view) < 50 {
		t.Errorf("convertingView too short: %d", len(view))
	}
}

func TestDoneView_renders(t *testing.T) {
	m := InitialModel()
	m.screen = screenDone
	m.width = 80
	m.height = 24
	m.convertedCount = 10
	m.folderInput.SetValue("/path/to/project")
	view := m.doneView()
	if view == "" {
		t.Error("doneView should not be empty")
	}
	if len(view) < 50 {
		t.Errorf("doneView too short: %d", len(view))
	}
}

func TestDoneView_withErrors(t *testing.T) {
	m := InitialModel()
	m.screen = screenDone
	m.width = 80
	m.height = 24
	m.convertedCount = 8
	m.failedCount = 2
	m.convertErrors = []string{
		"src/bad.py: API error",
		"lib/wrong.rs: timeout",
	}
	m.folderInput.SetValue("/path/to/project")
	view := m.doneView()
	if view == "" {
		t.Error("doneView should not be empty")
	}
}

func TestUpdateConvertFile_tracksSuccess(t *testing.T) {
	m := InitialModel()
	msg := convertFileMsg{
		index:   0,
		total:   5,
		relPath: "src/main.py",
		err:     nil,
		skipped: false,
	}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.convertedCount != 1 {
		t.Errorf("convertedCount = %d, want 1", m2.convertedCount)
	}
	if m2.failedCount != 0 {
		t.Errorf("failedCount = %d, want 0", m2.failedCount)
	}
}

func TestUpdateConvertFile_tracksFailure(t *testing.T) {
	m := InitialModel()
	msg := convertFileMsg{
		index:   0,
		total:   5,
		relPath: "src/main.py",
		err:     os.ErrNotExist,
		skipped: false,
	}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.failedCount != 1 {
		t.Errorf("failedCount = %d, want 1", m2.failedCount)
	}
	if m2.convertedCount != 0 {
		t.Errorf("convertedCount = %d, want 0", m2.convertedCount)
	}
	if len(m2.convertErrors) != 1 {
		t.Errorf("convertErrors len = %d, want 1", len(m2.convertErrors))
	}
}

func TestUpdateConvertFile_skipped(t *testing.T) {
	m := InitialModel()
	msg := convertFileMsg{
		index:   0,
		total:   5,
		relPath: "src/main.py",
		err:     nil,
		skipped: true,
	}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.convertedCount != 0 {
		t.Errorf("convertedCount should not increment for skipped")
	}
	if m2.failedCount != 0 {
		t.Errorf("failedCount should not increment for skipped")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAPIKeyView_withSavedKey(t *testing.T) {
	m := InitialModel()
	m.cfg.APIKeys = []string{"saved-key-123"}
	m.width = 80
	m.height = 24
	view := m.apiKeyView()
	if !contains(view, "Saved") {
		t.Error("view should mention 'Saved'")
	}
	if !contains(view, "key") {
		t.Error("view should mention 'key'")
	}
}

func TestDoneView_empty(t *testing.T) {
	m := InitialModel()
	m.screen = screenDone
	m.width = 80
	m.height = 24
	m.convertedCount = 0
	m.failedCount = 0
	view := m.doneView()
	if !contains(view, "No files") {
		t.Error("empty done view should say 'No files'")
	}
}

func TestFolderSelectView_emptyEntries(t *testing.T) {
	m := InitialModel()
	m.screen = screenFolderSelect
	m.width = 80
	m.height = 24
	view := m.folderSelectView()
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestView_dispatchesCorrectScreen(t *testing.T) {
	m := InitialModel()
	m.width = 80
	m.height = 24

	tests := []struct {
		name   string
		screen screen
		check  string
	}{
		{"API key", screenAPIKey, "WaterToGo"},
		{"Folder select", screenFolderSelect, "Select"},
		{"Converting", screenConverting, "Converting"},
		{"Done", screenDone, "WATER"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.screen = tt.screen
			view := m.View()
			if !contains(view, tt.check) {
				t.Errorf("View() for screen %d should contain %q", tt.screen, tt.check)
			}
		})
	}
}

func TestAPIKeyEnter_savesKey(t *testing.T) {
	m := InitialModel()
	m.cfg.APIKeys = nil
	m.apiInput.SetValue("my-real-key-12345")
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.screen != screenFolderSelect {
		t.Errorf("should go to folder select, got screen %d", m2.screen)
	}
	if m2.apiErr != "" {
		t.Errorf("unexpected error: %s", m2.apiErr)
	}
}
