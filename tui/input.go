package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleAPIKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		raw := m.apiInput.Value()
		raw = strings.TrimSpace(raw)
		raw = strings.TrimRight(raw, "\n\r,")

		if raw == "" && m.cfg.HasKeys() {
			raw = strings.Join(m.cfg.APIKeys, ", ")
		}
		if raw == "" {
			m.apiErr = "API key cannot be empty"
			return m, nil
		}

		parts := strings.Split(raw, ",")
		var keys []string
		for _, p := range parts {
			k := strings.TrimSpace(p)
			if k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			m.apiErr = "API key cannot be empty"
			return m, nil
		}

		m.cfg.APIKeys = keys
		if err := m.cfg.Save(); err != nil {
			m.apiErr = fmt.Sprintf("Failed to save config: %v", err)
			return m, nil
		}
		m.apiErr = ""
		m.screen = screenFolderSelect
		m.folderInput.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.apiInput, cmd = m.apiInput.Update(msg)
	return m, cmd
}

func (m model) handleFolderInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		path := strings.TrimSpace(m.folderInput.Value())
		if path == "" {
			m.folderErr = "Please enter a folder path"
			return m, nil
		}
		info, err := os.Stat(path)
		if err != nil {
			m.folderErr = fmt.Sprintf("Invalid path: %v", err)
			return m, nil
		}
		if !info.IsDir() {
			m.folderErr = "Path must be a directory"
			return m, nil
		}
		m.folderErr = ""
		m.screen = screenConverting
		m = m.resetConversion()
		m = m.openLogFile(path)
		return m, m.startConversion(path)
	}
	var cmd tea.Cmd
	m.folderInput, cmd = m.folderInput.Update(msg)
	return m, cmd
}
