package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnvictor/watertogo/converter"
)

func (m model) logLine(line string) model {
	ts := time.Now().Format("2006-01-02 15:04:05")
	m.logs = append(m.logs, line)
	if m.logFile != nil {
		fmt.Fprintf(m.logFile, "[%s] %s\n", ts, line)
	}
	return m
}

func (m model) openLogFile(root string) model {
	cwd, err := os.Getwd()
	logDir := cwd
	if err != nil {
		logDir = root
	}
	logPath := filepath.Join(logDir, "watertogo.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		m.logs = append(m.logs, fmt.Sprintf("⚠ failed to open log file at %s: %v", logPath, err))
		return m
	}
	m.logFile = f
	m.logPath = logPath
	m = m.logLine(fmt.Sprintf("=== WaterToGo conversion started ==="))
	m = m.logLine(fmt.Sprintf("API keys: %d configured", len(m.cfg.APIKeys)))
	m = m.logLine(fmt.Sprintf("Model: %s", converter.DefaultModelName))
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
