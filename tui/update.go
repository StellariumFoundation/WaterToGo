package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnvictor/watertogo/converter"
	"github.com/johnvictor/watertogo/scanner"

	tea "github.com/charmbracelet/bubbletea"
)

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
			switch m.screen {
			case screenFolderSelect:
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
	if !m.cfg.HasKeys() {
		m.err = "No API keys configured — cannot convert files"
		m.screen = screenDone
		return m, nil
	}
	m.conv = converter.New(m.cfg.APIKeys, msg.outputRoot, msg.root, converter.DefaultModelName)
	m.codeFiles = msg.codeFiles
	m.totalFiles = len(msg.codeFiles)
	m = m.logLine(fmt.Sprintf("Converting %d files to Go...", len(msg.codeFiles)))
	return m, m.convertFileCmd(0)
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
