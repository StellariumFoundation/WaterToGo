package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/johnvictor/watertogo/config"
	"github.com/johnvictor/watertogo/converter"
	"github.com/johnvictor/watertogo/scanner"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenAPIKey screen = iota
	screenFolderSelect
	screenConverting
	screenDone
)

var (
	appTeal    = lipgloss.Color("#00D4AA")
	appPurple  = lipgloss.Color("#A855F7")
	appPink    = lipgloss.Color("#EC4899")
	appOrange  = lipgloss.Color("#F59E0B")
	appBlue    = lipgloss.Color("#3B82F6")
	appGray    = lipgloss.Color("#6B7280")
	appLight   = lipgloss.Color("#F3F4F6")
	appDark    = lipgloss.Color("#1F2937")
	appRed     = lipgloss.Color("#EF4444")
	appGreen   = lipgloss.Color("#10B981")
)

var (
	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(appTeal).
		Padding(1, 2).
		MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(appTeal).
		Background(lipgloss.Color("#064E3B")).
		Padding(0, 3)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(appGray).
		Italic(true).
		MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
		Foreground(appRed).
		Bold(true).
		MarginTop(1)

	successStyle = lipgloss.NewStyle().
		Foreground(appGreen).
		Bold(true)

	warningStyle = lipgloss.NewStyle().
		Foreground(appOrange)

	dimStyle = lipgloss.NewStyle().
		Foreground(appGray).
		Italic(true)

	selectedStyle = lipgloss.NewStyle().
		Foreground(appTeal).
		Bold(true)

	folderStyle = lipgloss.NewStyle().
		Foreground(appOrange)

	fileStyle = lipgloss.NewStyle().
		Foreground(appLight)

	stepStyle = lipgloss.NewStyle().
		Foreground(appBlue).
		Bold(true)

	accentStyle = lipgloss.NewStyle().
		Foreground(appPurple).
		Bold(true)
)

func renderLogo() string {
	return accentStyle.Render("WATER TO GO")
}

func renderFooter() string {
	return dimStyle.Render("Ctrl+C to quit")
}

type model struct {
	screen        screen
	width         int
	height        int

	cfg           *config.Config

	apiInput      textinput.Model
	apiErr        string

	folderInput   textinput.Model
	folderErr     string

	scanResult    *scanner.ScanResult
	codebasePath  string
	conv          *converter.Converter
	codeFiles     []scanner.FileEntry

	totalFiles    int
	currentFile   int
	currentName   string
	progress      progress.Model
	logs          []string
	done          bool
	err           string

	convertErrors    []string
	convertedCount   int
	failedCount      int

	exePath       string
	logFile       *os.File
	logPath       string
}

func InitialModel() model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	_ = err

	apiInput := textinput.New()
	apiInput.Placeholder = "Paste API key(s), comma-separated for multiple..."
	apiInput.Width = 60
	apiInput.EchoMode = textinput.EchoPassword
	apiInput.Prompt = "🔑 "
	apiInput.Focus()
	if cfg.HasKeys() {
		apiInput.SetValue(strings.Join(cfg.APIKeys, ", "))
		apiInput.SetCursor(len(strings.Join(cfg.APIKeys, ", ")))
	}

	folderInput := textinput.New()
	folderInput.Placeholder = "C:\\Users\\...\\my-project"
	folderInput.Width = 60
	folderInput.Prompt = "📁 "

	exe, _ := os.Executable()
	pb := progress.New(progress.WithSolidFill("#7571F9"))
	pb.EmptyColor = ""
	pb.Empty = ' '

	return model{
		screen:      screenAPIKey,
		cfg:         cfg,
		apiInput:    apiInput,
		folderInput: folderInput,
		progress:      pb,
		logs:          []string{},
		convertErrors: []string{},
		exePath:       filepath.Dir(exe),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink)
}
