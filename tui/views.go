package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	var body string
	switch m.screen {
	case screenAPIKey:
		body = m.apiKeyView()
	case screenFolderSelect:
		body = m.folderSelectView()
	case screenConverting:
		body = m.convertingView()
	case screenDone:
		body = m.doneView()
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	bodyLines := strings.Count(body, "\n") + 1
	padding := (h - bodyLines) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat("\n", padding) + body
}

func (m model) apiKeyView() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	titleBlock := lipgloss.JoinVertical(lipgloss.Center,
		centerText(renderLogo(), w),
		centerText(titleStyle.Render("  WaterToGo  "), w),
		centerText(subtitleStyle.Render("Convert JS/TS/Python/Rust codebases to Go using Gemini"), w),
	)

	inputBlock := lipgloss.JoinVertical(lipgloss.Center,
		centerText("Enter your Google Gemini API key:", w),
		centerText(m.apiInput.View(), w),
	)
	if m.apiErr != "" {
		inputBlock = lipgloss.JoinVertical(lipgloss.Center,
			inputBlock,
			centerText(errorStyle.Render("✗ "+m.apiErr), w),
		)
	}

	var savedBlock string
	if m.cfg.HasKeys() {
		keyMsg := fmt.Sprintf("Saved %d key(s) — press Enter to continue or type new key(s)", len(m.cfg.APIKeys))
		savedBlock = lipgloss.JoinVertical(lipgloss.Center,
			centerText(dimStyle.Render(keyMsg), w),
			centerText(dimStyle.Render("Get one at https://aistudio.google.com/apikey"), w),
		)
	} else {
		savedBlock = lipgloss.JoinVertical(lipgloss.Center,
			centerText(dimStyle.Render("Separate multiple keys with commas to rotate on quota errors"), w),
			centerText(dimStyle.Render("Get one at https://aistudio.google.com/apikey"), w),
		)
	}

	confirmBlock := centerText(dimStyle.Render("Press Enter to confirm"), w)
	footerBlock := centerText(renderFooter(), w)

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		titleBlock,
		"",
		inputBlock,
		"",
		savedBlock,
		"",
		confirmBlock,
		"",
		footerBlock,
	)
}

func (m model) folderSelectView() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	titleBlock := lipgloss.JoinVertical(lipgloss.Center,
		centerText(accentStyle.Render("Select Codebase Folder"), w),
		centerText(subtitleStyle.Render("Select the project folder to convert to Go"), w),
	)

	inputBlock := centerText(m.folderInput.View(), w)
	if m.folderErr != "" {
		inputBlock = lipgloss.JoinVertical(lipgloss.Center,
			inputBlock,
			centerText(errorStyle.Render("✗ "+m.folderErr), w),
		)
	}

	hintBlock := centerText(dimStyle.Render("Type path + Enter to convert  •  Esc to change model"), w)

	btnStyle := lipgloss.NewStyle().
		Background(appTeal).
		Foreground(appDark).
		Bold(true).
		Padding(0, 6)
	btnBlock := centerText(btnStyle.Render("  Rewrite in Go  "), w)

	footerBlock := centerText(dimStyle.Render("Ctrl+C to quit"), w)

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		titleBlock,
		"",
		inputBlock,
		"",
		hintBlock,
		"",
		btnBlock,
		"",
		footerBlock,
	)
}

func (m model) convertingView() string {
	var b strings.Builder

	w := m.width
	if w <= 0 {
		w = 80
	}

	b.WriteString("\n\n")
	b.WriteString(centerText(accentStyle.Render("Converting to Go"), w))
	b.WriteString("\n")

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := int(time.Now().UnixMilli()/100) % len(spinner)
	b.WriteString(centerText(fmt.Sprintf(" %s  Processing...", spinner[frame]), w))
	b.WriteString("\n")

	if m.totalFiles > 0 {
		pct := float64(min(m.currentFile, m.totalFiles)) / float64(m.totalFiles)
		if pct > 1.0 {
			pct = 1.0
		}
		bar := m.progress.ViewAs(pct)
		b.WriteString(centerText(bar, w))
		b.WriteString(centerText(fmt.Sprintf("%d / %d files", min(m.currentFile, m.totalFiles), m.totalFiles), w))
		b.WriteString("\n")
		if m.conv != nil {
			b.WriteString(centerText(dimStyle.Render(fmt.Sprintf("Model: %s", m.conv.CurrentModel())), w))
			b.WriteString("\n")
		}
	}

	var logsBody strings.Builder
	visible := m.logs
	if len(visible) > 6 {
		visible = visible[len(visible)-6:]
	}

	logStyle := lipgloss.NewStyle().Foreground(appLight)
	for _, line := range visible {
		logsBody.WriteString(logStyle.Render(line))
		logsBody.WriteString("\n")
	}

	logsBody.WriteString(dimStyle.Render("This may take a while for large codebases..."))

	b.WriteString(centerBlock(logsBody.String(), w))
	b.WriteString("\n")
	b.WriteString(centerText(renderFooter(), w))

	return b.String()
}

func (m model) doneView() string {
	var b strings.Builder

	w := m.width
	if w <= 0 {
		w = 80
	}

	b.WriteString("\n\n")
	b.WriteString(centerText(renderLogo(), w))
	b.WriteString("\n\n")

	if m.err != "" {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(appRed).
			Padding(1, 3).
			Width(w - 10).
			Render(errorStyle.Render("✗ " + m.err))
		b.WriteString(centerText(box, w))
	} else {
		summary := fmt.Sprintf("✓  %d converted", m.convertedCount)
		if m.failedCount > 0 {
			summary += fmt.Sprintf("  |  ✗ %d failed", m.failedCount)
		}
		if m.convertedCount+m.failedCount == 0 {
			summary = "No files processed"
		}
		lines := []string{
			successStyle.Render(summary),
			"",
			fmt.Sprintf("Output:  %s/0go0/", m.folderInput.Value()),
			fmt.Sprintf("Log:     %s", m.logPath),
		}
		if len(m.convertErrors) > 0 {
			lines = append(lines, "")
			show := m.convertErrors
			if len(show) > 5 {
				show = show[:5]
			}
			for _, e := range show {
				lines = append(lines, errorStyle.Render("  ✗ "+e))
			}
			if len(m.convertErrors) > 5 {
				lines = append(lines, dimStyle.Render(fmt.Sprintf("  ... and %d more", len(m.convertErrors)-5)))
			}
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(appGreen).
			Padding(1, 3).
			Width(w - 10).
			Render(lipgloss.JoinVertical(lipgloss.Center, lines...))
		b.WriteString(centerText(box, w))
	}

	b.WriteString("\n\n")
	b.WriteString(centerText(dimStyle.Render("Press Enter to quit"), w))
	b.WriteString("\n")
	b.WriteString(centerText(renderFooter(), w))

	return b.String()
}
