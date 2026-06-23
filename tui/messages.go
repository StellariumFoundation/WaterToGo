package tui

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/johnvictor/watertogo/scanner"
)

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

func isRetryableTUIError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "429") ||
		strings.Contains(s, "401") ||
		strings.Contains(s, "UNAUTHENTICATED") ||
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
