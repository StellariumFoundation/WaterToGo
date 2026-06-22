package main

import (
	"fmt"
	"os"

	"github.com/johnvictor/watertogo/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := tui.InitialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
