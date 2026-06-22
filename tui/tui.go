package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/johnvictor/watertogo/config"
	"github.com/johnvictor/watertogo/converter"
	"github.com/johnvictor/watertogo/scanner"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/genai"
)

type screen int

const (
	screenAPIKey screen = iota
	screenModelLoading
	screenModelSelect
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

	models        []string
	modelCursor   int
	selectedModel string
	modelErr      string
	modelInput    textinput.Model

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

	modelInput := textinput.New()
	modelInput.Placeholder = "gemini-3-flash-preview"
	modelInput.Width = 60
	modelInput.Prompt = "🪄 "

	exe, _ := os.Executable()

	return model{
		screen:      screenAPIKey,
		cfg:         cfg,
		apiInput:    apiInput,
		folderInput: folderInput,
		modelInput:  modelInput,
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
			if m.screen != screenAPIKey && m.screen != screenModelLoading && m.screen != screenModelSelect && m.screen != screenFolderSelect {
				return m, tea.Quit
			}
		case "esc":
			switch m.screen {
			case screenFolderSelect:
				m.screen = screenModelSelect
				return m, nil
			}
		}

		switch m.screen {
		case screenAPIKey:
			return m.handleAPIKeyInput(msg)
		case screenModelSelect:
			return m.handleModelSelectInput(msg)
		case screenFolderSelect:
			return m.handleFolderInput(msg)
		case screenDone:
			if msg.String() == "enter" {
				return m, tea.Quit
			}
		}

	case modelsLoadedMsg:
		m.models = msg.models
		m.modelCursor = 0
		if msg.err != nil {
			m.modelErr = msg.err.Error()
		} else {
			m.modelErr = ""
		}
		m.screen = screenModelSelect
		if len(m.models) == 0 {
			m.modelInput.Focus()
		}
		return m, nil

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
		m = m.logLine(fmt.Sprintf("=== Conversion complete: %d converted, %d failed ===", m.convertedCount, m.failedCount))
		m.closeLog()
		m.screen = screenDone
		return m, nil
	case conversionErrMsg:
		m.err = string(msg)
		m = m.logLine(fmt.Sprintf("=== Conversion failed: %s ===", string(msg)))
		m.closeLog()
		m.screen = screenDone
		return m, nil
	}

	return m, nil
}

// ----- API key input -----
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
		m.models = nil
		m.modelCursor = 0
		m.selectedModel = ""
		m.modelErr = ""
		m.screen = screenModelLoading
		return m, loadModelsCmd(keys[0])
	}

	var cmd tea.Cmd
	m.apiInput, cmd = m.apiInput.Update(msg)
	return m, cmd
}

func (m model) handleModelSelectInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modelInput.Focused() {
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.modelInput.Value())
			if val == "" {
				return m, nil
			}
			m.selectedModel = val
			m.screen = screenFolderSelect
			m.folderInput.Focus()
			return m, nil
		case "esc":
			m.modelInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.modelInput, cmd = m.modelInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.modelCursor > 0 {
			m.modelCursor--
		}
		return m, nil
	case "down", "j":
		if m.modelCursor < len(m.models)-1 {
			m.modelCursor++
		}
		return m, nil
	case "enter":
		if len(m.models) > 0 {
			m.selectedModel = m.models[m.modelCursor]
			m.screen = screenFolderSelect
			m.folderInput.Focus()
			return m, nil
		}
		return m, nil
	case "esc":
		if m.modelInput.Focused() {
			m.modelInput.Blur()
			return m, nil
		}
		if m.modelInput.Value() != "" {
			m.modelInput.Focus()
			return m, nil
		}
		m.screen = screenAPIKey
		m.apiInput.Focus()
		return m, nil
	}
	return m, nil
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

// ----- model messages -----
type modelsLoadedMsg struct {
	models []string
	err    error
}

func loadModelsCmd(apiKey string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return modelsLoadedMsg{err: fmt.Errorf("creating client: %w", err)}
		}

		models, err := client.Models.List(ctx, &genai.ListModelsConfig{})
		if err != nil {
			return modelsLoadedMsg{err: fmt.Errorf("listing models: %w", err)}
		}

		var names []string
		for _, m := range models.Items {
			name := m.Name
			if strings.HasPrefix(name, "models/") {
				name = name[7:]
			}
			names = append(names, name)
		}
		return modelsLoadedMsg{models: names}
	}
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
	m = m.logLine(fmt.Sprintf("Found %d files", countFiles(msg.result)))
	m = m.logLine("Running contasty --strip=all...")

	return m, func() tea.Msg {
		mdPath := filepath.Join(msg.root, "codebase.md")
		f, err := os.Create(mdPath)
		if err != nil {
			return codebaseDoneMsg{
				outputRoot: msg.outputRoot,
				root:       msg.root,
				err:        fmt.Errorf("creating codebase.md: %w", err),
			}
		}
		defer f.Close()

		cmd := exec.Command("contasty", "--strip=all")
		cmd.Dir = msg.root
		cmd.Stdout = f
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return codebaseDoneMsg{
				outputRoot: msg.outputRoot,
				root:       msg.root,
				err:        fmt.Errorf("contasty failed: %w", err),
			}
		}

		return codebaseDoneMsg{
			codebasePath: mdPath,
			outputRoot:   msg.outputRoot,
			root:         msg.root,
			scanResult:   msg.result,
			err:          nil,
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
	m = m.logLine("✓ codebase.md ready")

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
	m.conv = converter.New(m.cfg.APIKeys, msg.outputRoot, msg.root, m.selectedModel)
	m.codeFiles = msg.codeFiles
	m.totalFiles = len(msg.codeFiles)
	m = m.logLine(fmt.Sprintf("Converting %d files to Go...", len(msg.codeFiles)))
	return m, m.convertFileCmd(0)
}

func isRetryableTUIError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "429") ||
		strings.Contains(s, "RESOURCE_EXHAUSTED") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "TLS handshake timeout") ||
		strings.Contains(s, "read tcp") ||
		strings.Contains(s, "write tcp") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "deadline exceeded")
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

func (m model) convertFileCmd(index int) tea.Cmd {
	return func() tea.Msg {
		f := m.codeFiles[index]

		if m.conv.IsConverted(f.RelPath) {
			var nextCmd tea.Cmd
			if index+1 < len(m.codeFiles) {
				nextCmd = m.convertFileCmd(index + 1)
			} else {
				nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
			}
			return convertFileMsg{index: index, total: len(m.codeFiles), relPath: f.RelPath, nextCmd: nextCmd, skipped: true}
		}

		sourceData, err := os.ReadFile(f.Path)
		if err != nil {
			var nextCmd tea.Cmd
			if index+1 < len(m.codeFiles) {
				nextCmd = m.convertFileCmd(index + 1)
			} else {
				nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
			}
			return convertFileMsg{index: index, total: len(m.codeFiles), relPath: f.RelPath, nextCmd: nextCmd, err: err}
		}

		_, err = m.conv.Convert(f.RelPath, string(sourceData))

		if isRetryableTUIError(err) {
			delay := retryDelayFromError(err)
			if delay < time.Second {
				delay = 30 * time.Second
			}
			return retryFileMsg{
				index:      index,
				total:      len(m.codeFiles),
				relPath:    f.RelPath,
				sourceData: string(sourceData),
				attempt:    1,
				delay:      delay,
			}
		}

		var nextCmd tea.Cmd
		if index+1 < len(m.codeFiles) {
			nextCmd = m.convertFileCmd(index + 1)
		} else {
			nextCmd = m.conversionDoneCmd(m.conv.OutputRoot)
		}
		return convertFileMsg{index: index, total: len(m.codeFiles), relPath: f.RelPath, nextCmd: nextCmd, err: err}
	}
}

func (m model) conversionDoneCmd(outputRoot string) tea.Cmd {
	return func() tea.Msg {
		adjustProjectStructure(outputRoot)
		return conversionDoneMsg{}
	}
}

func (m model) logLine(line string) model {
	ts := time.Now().Format("2006-01-02 15:04:05")
	m.logs = append(m.logs, line)
	if m.logFile != nil {
		fmt.Fprintf(m.logFile, "[%s] %s\n", ts, line)
	}
	return m
}

func (m model) openLogFile(root string) model {
	logPath := filepath.Join(root, "watertogo.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		m.logs = append(m.logs, fmt.Sprintf("⚠ failed to open log file: %v", err))
		return m
	}
	m.logFile = f
	m = m.logLine(fmt.Sprintf("=== WaterToGo conversion started ==="))
	m = m.logLine(fmt.Sprintf("Model: %s", m.selectedModel))
	m = m.logLine(fmt.Sprintf("Output: %s/0go0", root))
	return m
}

func (m model) closeLog() {
	if m.logFile != nil {
		m.logFile.Close()
	}
}

func (m model) resetConversion() model {
	m.closeLog()
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
		m = m.logLine(fmt.Sprintf("✗ %s: %v", msg.relPath, msg.err))
	} else if msg.skipped {
		m = m.logLine(fmt.Sprintf("– %s: already converted, skipped", msg.relPath))
	} else {
		m.convertedCount++
		m = m.logLine(fmt.Sprintf("✓ %s", msg.relPath))
	}
	return m, msg.nextCmd
}

func (m model) updateRetryFile(msg retryFileMsg) (tea.Model, tea.Cmd) {
	m.currentFile = msg.index + 1
	m.totalFiles = msg.total
	m.currentName = msg.relPath
	m = m.logLine(fmt.Sprintf("⏳ %s: rate limited, retrying in %ds (attempt %d)", msg.relPath, int(msg.delay.Seconds()), msg.attempt))
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
	m = m.logLine(fmt.Sprintf("⟳ %s: retrying...", msg.relPath))

	const maxAttempts = 5
	if msg.attempt >= maxAttempts {
		m.failedCount++
		m.convertErrors = append(m.convertErrors, fmt.Sprintf("%s: exceeded %d retries", msg.relPath, maxAttempts))
		m = m.logLine(fmt.Sprintf("✗ %s: max retries exceeded", msg.relPath))
		if msg.index+1 < m.totalFiles {
			return m, m.convertFileCmd(msg.index + 1)
		}
		return m, m.conversionDoneCmd(m.conv.OutputRoot)
	}

	_, err := m.conv.Convert(msg.relPath, msg.sourceData)
	if isRetryableTUIError(err) {
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
		m = m.logLine(fmt.Sprintf("✗ %s: %v", msg.relPath, err))
	} else {
		m.convertedCount++
		m = m.logLine(fmt.Sprintf("✓ %s", msg.relPath))
	}
	if msg.index+1 < msg.total {
		return m, m.convertFileCmd(msg.index + 1)
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
	case screenModelLoading:
		body = m.modelLoadingView()
	case screenModelSelect:
		body = m.modelSelectView()
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

	if m.cfg.HasKeys() {
		keyMsg := fmt.Sprintf("Saved %d key(s) — press Enter to continue or type new key(s)", len(m.cfg.APIKeys))
		b.WriteString(centerText(dimStyle.Render(keyMsg), 60))
	} else {
		b.WriteString(centerText(dimStyle.Render("Separate multiple keys with commas to rotate on quota errors"), 60))
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

func (m model) modelLoadingView() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := int(time.Now().UnixMilli()/100) % len(spinner)

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		centerText(accentStyle.Render("Loading Available Models"), w),
		"",
		centerText(fmt.Sprintf(" %s  Fetching models from Gemini...", spinner[frame]), w),
		"",
		centerText(renderFooter(), w),
	)
}

func (m model) modelSelectView() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(centerText(accentStyle.Render("Select a Gemini Model"), w))
	b.WriteString("\n")
	b.WriteString(centerText(subtitleStyle.Render("Choose the model to use for conversion"), w))
	b.WriteString("\n\n")

	if m.modelErr != "" {
		b.WriteString(centerText(warningStyle.Render("⚠ Failed to fetch model list: "+m.modelErr), w))
		b.WriteString(centerText(dimStyle.Render("Type a model name manually below"), w))
		b.WriteString("\n\n")
	}

	if len(m.models) > 0 {
		listStyle := lipgloss.NewStyle().Width(w - 10).Align(lipgloss.Left)
		var listContent strings.Builder

		visible := m.models
		if len(visible) > 20 {
			visible = visible[m.modelCursor-min(m.modelCursor, 10):]
			if len(visible) > 20 {
				visible = visible[:20]
			}
		}

		for i, name := range visible {
			globalIdx := i + m.modelCursor - min(m.modelCursor, 10)
			if globalIdx >= len(m.models) {
				break
			}
			if globalIdx == m.modelCursor {
				listContent.WriteString(selectedStyle.Render("▸ " + name))
			} else {
				listContent.WriteString(dimStyle.Render("  " + name))
			}
			listContent.WriteString("\n")
		}

		b.WriteString(centerText(listStyle.Render(listContent.String()), w))
		b.WriteString("\n")
		b.WriteString(centerText(dimStyle.Render("↑/↓ or j/k to navigate  •  Enter to select"), w))
		b.WriteString("\n\n")
		b.WriteString(centerText(accentStyle.Render("— or type a custom model name —"), w))
		b.WriteString("\n")
	} else {
		b.WriteString(centerText(dimStyle.Render("No models loaded — type a model name to continue"), w))
		b.WriteString("\n\n")
	}

	b.WriteString(centerText(m.modelInput.View(), w))
	b.WriteString("\n\n")
	b.WriteString(centerText(dimStyle.Render("Tab/Enter to confirm list selection  •  Enter in input to confirm  •  Esc to switch"), w))
	b.WriteString("\n\n")
	b.WriteString(centerText(renderFooter(), w))

	return b.String()
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
