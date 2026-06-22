package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/johnvictor/watertogo/codebase"
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

// ----- styles -----
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

// ----- model -----
type model struct {
	screen        screen
	width         int
	height        int

	cfg           *config.Config

	apiInput      textinput.Model
	apiErr        string

	folderInput   textinput.Model
	folderErr     string
	folderEntries []string
	folderCursor  int
	folderListFocused bool

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
}

func InitialModel() model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	_ = err

	apiInput := textinput.New()
	apiInput.Placeholder = "Paste your Gemini API key here..."
	apiInput.Width = 60
	apiInput.EchoMode = textinput.EchoPassword
	apiInput.Prompt = "🔑 "
	apiInput.Focus()
	if cfg.HasKey() {
		apiInput.SetValue(cfg.APIKey)
		apiInput.SetCursor(len(cfg.APIKey))
	}

	folderInput := textinput.New()
	folderInput.Placeholder = "C:\\Users\\...\\my-project"
	folderInput.Width = 60
	folderInput.Prompt = "📁 "

	exe, _ := os.Executable()

	return model{
		screen:      screenAPIKey,
		cfg:         cfg,
		apiInput:    apiInput,
		folderInput: folderInput,
		progress:      progress.New(progress.WithDefaultGradient()),
		logs:          []string{},
		convertErrors: []string{},
		exePath:       filepath.Dir(exe),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 20
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.screen != screenAPIKey && m.screen != screenFolderSelect {
				return m, tea.Quit
			}
		case "esc":
			if m.screen == screenFolderSelect {
				m.screen = screenAPIKey
				return m, nil
			}
		}

		switch m.screen {
		case screenAPIKey:
			return m.handleAPIKeyInput(msg)
		case screenFolderSelect:
			return m.handleFolderInput(msg)
		case screenDone:
			if msg.String() == "enter" {
				return m, tea.Quit
			}
		}

	case scanDoneMsg:
		return m.updateScanDone(msg)
	case codebaseDoneMsg:
		return m.updateCodebaseDone(msg)
	case copyDoneMsg:
		return m.updateCopyDone(msg)
	case convertFileMsg:
		return m.updateConvertFile(msg)
	case retryFileMsg:
		return m.updateRetryFile(msg)
	case retryTickMsg:
		return m.updateRetryTick(msg)
	case conversionDoneMsg:
		m.done = true
		m.screen = screenDone
		return m, nil
	case conversionErrMsg:
		m.err = string(msg)
		m.screen = screenDone
		return m, nil
	case folderEntriesMsg:
		m.folderEntries = msg
		return m, nil
	}

	return m, nil
}

// ----- API key input -----
func (m model) handleAPIKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		raw := m.apiInput.Value()
		key := strings.TrimSpace(raw)
		key = strings.TrimRight(key, "\n\r")

		if key == "" && m.cfg.HasKey() {
			m.screen = screenFolderSelect
			m.folderInput.Focus()
			return m, m.loadFolderEntries("")
		}
		if key == "" {
			m.apiErr = "API key cannot be empty"
			return m, nil
		}
		m.cfg.APIKey = key
		if err := m.cfg.Save(); err != nil {
			m.apiErr = fmt.Sprintf("Failed to save config: %v", err)
			return m, nil
		}
		m.apiErr = ""
		m.screen = screenFolderSelect
		m.folderInput.Focus()
		return m, m.loadFolderEntries("")
	}

	var cmd tea.Cmd
	m.apiInput, cmd = m.apiInput.Update(msg)
	return m, cmd
}

// ----- folder browser -----
type folderEntriesMsg []string

func (m model) loadFolderEntries(path string) tea.Cmd {
	return func() tea.Msg {
		dir := path
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return folderEntriesMsg{}
		}
		var names []string
		abs, _ := filepath.Abs(dir)
		names = append(names, "..")
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			display := name
			if e.IsDir() {
				display = name + "/"
			}
			names = append(names, display)
		}
		names = append(names, "─── Rewrite in Go ───")
		if path == "" {
			m.folderInput.SetValue(abs)
		}
		return folderEntriesMsg(names)
	}
}

func (m model) handleFolderInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Toggle focus between input and file list
	if msg.String() == "tab" {
		m.folderListFocused = !m.folderListFocused
		if m.folderListFocused {
			m.folderInput.Blur()
		} else {
			m.folderInput.Focus()
		}
		return m, nil
	}

	// List is focused - navigate files
	if m.folderListFocused {
		switch msg.String() {
		case "up", "k":
			if m.folderCursor > 0 {
				m.folderCursor--
			}
			return m, nil
		case "down", "j":
			if m.folderCursor < len(m.folderEntries)-1 {
				m.folderCursor++
			}
			return m, nil
		case "enter":
			if m.folderCursor >= 0 && m.folderCursor < len(m.folderEntries) {
				entry := m.folderEntries[m.folderCursor]
				if entry == "─── Rewrite in Go ───" {
					path := strings.TrimSpace(m.folderInput.Value())
					if path == "" {
						m.folderErr = "Please select a folder"
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
					return m, m.startConversion(path)
				}
				// Navigate into directory
				var newPath string
				if entry == ".." {
					newPath = filepath.Dir(m.folderInput.Value())
				} else {
					entry = strings.TrimSuffix(entry, "/")
					newPath = filepath.Join(m.folderInput.Value(), entry)
				}
				info, err := os.Stat(newPath)
				if err == nil && info.IsDir() {
					m.folderInput.SetValue(newPath)
					m.folderCursor = 0
					return m, m.loadFolderEntries(newPath)
				}
			}
			return m, nil
		}
		return m, nil
	}

	// Input is focused
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
		return m, m.startConversion(path)
	}
	var cmd tea.Cmd
	m.folderInput, cmd = m.folderInput.Update(msg)
	return m, cmd
}

// ----- conversion messages -----
type conversionProgressMsg struct {
	current int
	total   int
	name    string
}

type conversionLogMsg string
type conversionDoneMsg struct{}
type conversionErrMsg string

type scanDoneMsg struct {
	result     *scanner.ScanResult
	root       string
	outputRoot string
	err        error
}

type codebaseDoneMsg struct {
	codebasePath string
	outputRoot   string
	root         string
	scanResult   *scanner.ScanResult
	err          error
}

type copyDoneMsg struct {
	codeFiles    []scanner.FileEntry
	codebasePath string
	outputRoot   string
	root         string
}

type convertFileMsg struct {
	index    int
	total    int
	relPath  string
	nextCmd  tea.Cmd
	err      error
	skipped  bool
}

type retryFileMsg struct {
	index      int
	total      int
	relPath    string
	sourceData string
	attempt    int
	delay      time.Duration
}

type retryTickMsg struct {
	index      int
	total      int
	relPath    string
	sourceData string
	attempt    int
}

// ----- start conversion -----
func (m model) startConversion(path string) tea.Cmd {
	return func() tea.Msg {
		root := path
		outputRoot := filepath.Join(root, "0go0")
		result, err := scanner.Scan(root)
		return scanDoneMsg{result: result, root: root, outputRoot: outputRoot, err: err}
	}
}

func (m model) updateScanDone(msg scanDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = fmt.Sprintf("Scan failed: %v", msg.err)
		m.screen = screenDone
		return m, nil
	}
	m.scanResult = msg.result
	m.logs = append(m.logs, fmt.Sprintf("Found %d files", countFiles(msg.result)))
	m.logs = append(m.logs, "Generating codebase.md...")
	return m, func() tea.Msg {
		path, err := codebase.Generate(msg.root, msg.result)
		return codebaseDoneMsg{
			codebasePath: path,
			outputRoot:   msg.outputRoot,
			root:         msg.root,
			scanResult:   msg.result,
			err:          err,
		}
	}
}

func (m model) updateCodebaseDone(msg codebaseDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = fmt.Sprintf("Codebase generation failed: %v", msg.err)
		m.screen = screenDone
		return m, nil
	}

	m.codebasePath = msg.codebasePath
	tea.Println("✓ codebase.md created")

	var codeFiles, nonCodeFiles []scanner.FileEntry
	for _, f := range msg.scanResult.Files {
		if f.IsDir {
			continue
		}
		if f.IsCode {
			codeFiles = append(codeFiles, f)
		} else {
			nonCodeFiles = append(nonCodeFiles, f)
		}
	}

	tea.Printf("▸ Copying %d non-code files...\n", len(nonCodeFiles))
	for _, f := range nonCodeFiles {
		dst := filepath.Join(msg.outputRoot, f.RelPath)
		if err := converter.CopyNonCodeFile(f.Path, dst); err != nil {
			tea.Printf("  ⚠ %s\n", err)
		}
	}
	tea.Println("✓ Non-code files copied")

	m.totalFiles = len(codeFiles)
	m.currentFile = 0

	return m, func() tea.Msg {
		return copyDoneMsg{
			codeFiles:    codeFiles,
			codebasePath: msg.codebasePath,
			outputRoot:   msg.outputRoot,
			root:         msg.root,
		}
	}
}

func (m model) updateCopyDone(msg copyDoneMsg) (tea.Model, tea.Cmd) {
	if len(msg.codeFiles) == 0 {
		return m, m.conversionDoneCmd(msg.outputRoot)
	}
	m.conv = converter.New(m.cfg.APIKey, msg.codebasePath, msg.outputRoot, msg.root)
	m.totalFiles = len(msg.codeFiles)
	m.logs = append(m.logs, fmt.Sprintf("Converting %d files to Go...", len(msg.codeFiles)))
	return m, m.convertFileCmd(msg.codeFiles, 0)
}

func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "quota")
}

var retryDelayRe = regexp.MustCompile(`retryDelay[:\s]*(\d+(?:\.\d+)?)s`)

func retryDelayFromError(err error) time.Duration {
	if err == nil {
		return 0
	}
	matches := retryDelayRe.FindStringSubmatch(err.Error())
	if len(matches) >= 2 {
		if secs, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return 0
}

func (m model) convertFileCmd(files []scanner.FileEntry, index int) tea.Cmd {
	return func() tea.Msg {
		f := files[index]

		if m.conv.IsConverted(f.RelPath) {
			var nextCmd tea.Cmd
			if index+1 < len(files) {
				nextCmd = m.convertFileCmd(files, index+1)
			} else {
				nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
			}
			return convertFileMsg{index: index, total: len(files), relPath: f.RelPath, nextCmd: nextCmd, skipped: true}
		}

		sourceData, err := os.ReadFile(f.Path)
		if err != nil {
			var nextCmd tea.Cmd
			if index+1 < len(files) {
				nextCmd = m.convertFileCmd(files, index+1)
			} else {
				nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
			}
			return convertFileMsg{index: index, total: len(files), relPath: f.RelPath, nextCmd: nextCmd, err: err}
		}

		_, err = m.conv.Convert(f.RelPath, string(sourceData))

		if isQuotaError(err) {
			delay := retryDelayFromError(err)
			if delay < time.Second {
				delay = 30 * time.Second
			}
			return retryFileMsg{
				index:      index,
				total:      len(files),
				relPath:    f.RelPath,
				sourceData: string(sourceData),
				attempt:    1,
				delay:      delay,
			}
		}

		var nextCmd tea.Cmd
		if index+1 < len(files) {
			nextCmd = m.convertFileCmd(files, index+1)
		} else {
			nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
		}
		return convertFileMsg{index: index, total: len(files), relPath: f.RelPath, nextCmd: nextCmd, err: err}
	}
}

func (m model) conversionDoneCmd(outputRoot string) tea.Cmd {
	return func() tea.Msg {
		adjustProjectStructure(outputRoot)
		return conversionDoneMsg{}
	}
}

func (m model) resetConversion() model {
	m.convertErrors = []string{}
	m.convertedCount = 0
	m.failedCount = 0
	m.currentFile = 0
	m.totalFiles = 0
	m.logs = []string{}
	return m
}

func (m model) updateConvertFile(msg convertFileMsg) (tea.Model, tea.Cmd) {
	m.currentFile = msg.index + 1
	m.totalFiles = msg.total
	m.currentName = msg.relPath
	if msg.err != nil {
		m.failedCount++
		m.convertErrors = append(m.convertErrors, fmt.Sprintf("%s: %v", msg.relPath, msg.err))
		m.logs = append(m.logs, fmt.Sprintf("✗ %s: %v", msg.relPath, msg.err))
	} else if !msg.skipped {
		m.convertedCount++
		m.logs = append(m.logs, fmt.Sprintf("✓ %s", msg.relPath))
	}
	return m, msg.nextCmd
}

func (m model) updateRetryFile(msg retryFileMsg) (tea.Model, tea.Cmd) {
	m.currentFile = msg.index + 1
	m.totalFiles = msg.total
	m.currentName = msg.relPath
	m.logs = append(m.logs, fmt.Sprintf("⏳ %s: rate limited, retrying in %ds (attempt %d)", msg.relPath, int(msg.delay.Seconds()), msg.attempt))
	return m, tea.Tick(msg.delay, func(t time.Time) tea.Msg {
		return retryTickMsg{
			index:      msg.index,
			total:      msg.total,
			relPath:    msg.relPath,
			sourceData: msg.sourceData,
			attempt:    msg.attempt,
		}
	})
}

func (m model) updateRetryTick(msg retryTickMsg) (tea.Model, tea.Cmd) {
	m.logs = append(m.logs, fmt.Sprintf("⟳ %s: retrying...", msg.relPath))

	const maxAttempts = 5
	if msg.attempt >= maxAttempts {
		m.failedCount++
		m.convertErrors = append(m.convertErrors, fmt.Sprintf("%s: exceeded %d retries", msg.relPath, maxAttempts))
		m.logs = append(m.logs, fmt.Sprintf("✗ %s: max retries exceeded", msg.relPath))
		if msg.index+1 < m.totalFiles {
			return m, m.convertFileCmd(nil, msg.index+1)
		}
		return m, m.conversionDoneCmd(m.conv.OutputRoot)
	}

	_, err := m.conv.Convert(msg.relPath, msg.sourceData)
	if isQuotaError(err) {
		delay := retryDelayFromError(err)
		if delay < time.Second {
			delay = 30 * time.Second
		}
		return m, tea.Tick(delay, func(t time.Time) tea.Msg {
			return retryTickMsg{
				index:      msg.index,
				total:      msg.total,
				relPath:    msg.relPath,
				sourceData: msg.sourceData,
				attempt:    msg.attempt + 1,
			}
		})
	}
	if err != nil {
		m.failedCount++
		m.convertErrors = append(m.convertErrors, fmt.Sprintf("%s: %v", msg.relPath, err))
		m.logs = append(m.logs, fmt.Sprintf("✗ %s: %v", msg.relPath, err))
	} else {
		m.convertedCount++
		m.logs = append(m.logs, fmt.Sprintf("✓ %s", msg.relPath))
	}
	if msg.index+1 < msg.total {
		return m, m.convertFileCmd(nil, msg.index+1)
	}
	return m, m.conversionDoneCmd(m.conv.OutputRoot)
}

func countFiles(r *scanner.ScanResult) int {
	n := 0
	for _, f := range r.Files {
		if !f.IsDir {
			n++
		}
	}
	return n
}

func countDirs(r *scanner.ScanResult) int {
	n := 0
	for _, f := range r.Files {
		if f.IsDir {
			n++
		}
	}
	return n
}

func centerText(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

// ----- views -----
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

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, body)
}

func (m model) apiKeyView() string {
	var b strings.Builder

	b.WriteString("\n\n")
	b.WriteString(centerText(renderLogo(), 60))
	b.WriteString("\n\n")
	b.WriteString(centerText(titleStyle.Render("  WaterToGo  "), 60))
	b.WriteString("\n")
	b.WriteString(centerText(subtitleStyle.Render("Convert any JS/TS/Python/Rust codebase to Go using Gemini 3.5 Flash"), 60))
	b.WriteString("\n\n")

	b.WriteString(centerText("Enter your Google Gemini API key:", 60))
	b.WriteString(centerText(dimStyle.Render("Get one at https://aistudio.google.com/apikey"), 60))
	b.WriteString("\n")
	b.WriteString(centerText(m.apiInput.View(), 60))

	if m.cfg.HasKey() {
		b.WriteString(centerText(dimStyle.Render("Saved key detected — press Enter to continue or type a new key"), 60))
	} else {
		b.WriteString(centerText(dimStyle.Render("Right-click or Shift+Insert to paste"), 60))
	}

	if m.apiErr != "" {
		b.WriteString("\n")
		b.WriteString(centerText(errorStyle.Render("✗ "+m.apiErr), 60))
	}

	b.WriteString("\n\n")
	b.WriteString(centerText(dimStyle.Render("Press Enter to confirm"), 60))
	b.WriteString("\n\n")
	b.WriteString(centerText(renderFooter(), 60))

	return b.String()
}

func (m model) folderSelectView() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	titleBlock := lipgloss.JoinVertical(lipgloss.Center,
		centerText(accentStyle.Render("Select Codebase Folder"), w),
		centerText(subtitleStyle.Render("Navigate to the project you want to convert to Go"), w),
	)

	inputBlock := centerText(m.folderInput.View(), w)
	if m.folderErr != "" {
		inputBlock = lipgloss.JoinVertical(lipgloss.Center,
			inputBlock,
			centerText(errorStyle.Render("✗ "+m.folderErr), w),
		)
	}

	var listBlock string
	if len(m.folderEntries) > 0 {
		visible := h - 14
		if visible < 5 {
			visible = 5
		}
		start := 0
		end := len(m.folderEntries)
		if len(m.folderEntries) > visible {
			start = max(0, m.folderCursor-visible/2)
			end = min(len(m.folderEntries), start+visible)
			if end-start < visible {
				start = max(0, end-visible)
			}
		}

		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			entry := m.folderEntries[i]
			entryStyle := dimStyle
			prefix := "  "

			if i == m.folderCursor {
				entryStyle = selectedStyle
				prefix = selectedStyle.Render("▸ ")
			}

			if entry == "─── Rewrite in Go ───" {
				lines = append(lines, prefix+dimStyle.Render("──── Rewrite in Go ────"))
				continue
			}
			if strings.HasSuffix(entry, "/") || entry == ".." {
				lines = append(lines, fmt.Sprintf("%s📁 %s", prefix, entryStyle.Render(entry)))
			} else {
				lines = append(lines, fmt.Sprintf("%s %s", prefix, entryStyle.Render(entry)))
			}
		}
		listBlock = lipgloss.NewStyle().Width(w).Align(lipgloss.Left).Render(strings.Join(lines, "\n"))
	}

	hintBlock := centerText(dimStyle.Render("Type path + Enter to convert  •  Tab to browse  •  Esc back"), w)

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
		listBlock,
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

func adjustProjectStructure(root string) {
	goFiles := 0
	hasMain := false
	hasGoMod := false

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") {
			goFiles++
		}
		if info.Name() == "main.go" {
			hasMain = true
		}
		if info.Name() == "go.mod" {
			hasGoMod = true
		}
		return nil
	})

	tea.Printf("  ⤷ %d Go files generated\n", goFiles)
	if hasMain {
		tea.Println("  ⤷ main.go found ✓")
	} else {
		tea.Println(dimStyle.Render("  ⤷ Library project (no main.go)"))
	}
	if !hasGoMod {
		tea.Println(dimStyle.Render("  ⤷ Tip: run  go mod init <module>  in 0go0/"))
	}
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
