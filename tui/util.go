package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func centerText(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

func centerBlock(content string, width int) string {
	if width <= 0 {
		width = 80
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		lipgloss.NewStyle().Width(width - 4).Align(lipgloss.Left).Render(content),
	)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
