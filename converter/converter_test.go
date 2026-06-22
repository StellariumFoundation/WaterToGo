package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanGoCode_noFence(t *testing.T) {
	input := "package main\n\nfunc main() {}\n"
	got := cleanGoCode(input)
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Errorf("cleanGoCode = %q, want %q", got, want)
	}
}

func TestCleanGoCode_goFence(t *testing.T) {
	input := "```go\npackage main\n\nfunc main() {}\n```"
	got := cleanGoCode(input)
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Errorf("cleanGoCode = %q, want %q", got, want)
	}
}

func TestCleanGoCode_genericFence(t *testing.T) {
	input := "```\npackage main\n\nfunc main() {}\n```"
	got := cleanGoCode(input)
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Errorf("cleanGoCode = %q, want %q", got, want)
	}
}

func TestCleanGoCode_trailingNewlines(t *testing.T) {
	input := "\n\npackage main\n\nfunc main() {}\n\n\n"
	got := cleanGoCode(input)
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Errorf("cleanGoCode = %q, want %q", got, want)
	}
}

func TestCleanGoCode_explanationBeforeFence(t *testing.T) {
	input := "Here is the Go code:\n```go\npackage main\n\nfunc main() {}\n```"
	got := cleanGoCode(input)
	want := "package main\n\nfunc main() {}"
	if got != want {
		t.Errorf("cleanGoCode = %q, want %q", got, want)
	}
}

func TestGoPath(t *testing.T) {
	tmp := t.TempDir()
	c := New([]string{"fake-key"}, tmp, "", "")
	tests := []struct {
		relPath string
		want    string
	}{
		{"src/main.py", filepath.Join(tmp, "src/main.go")},
		{"lib/utils.js", filepath.Join(tmp, "lib/utils.go")},
		{"app.ts", filepath.Join(tmp, "app.go")},
		{"cmd/service.rs", filepath.Join(tmp, "cmd/service.go")},
		{"test.jsx", filepath.Join(tmp, "test.go")},
		{"component.tsx", filepath.Join(tmp, "component.go")},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			got := c.goPath(tt.relPath)
			if got != tt.want {
				t.Errorf("goPath(%q) = %q, want %q", tt.relPath, got, tt.want)
			}
		})
	}
}

func TestIsConverted_missing(t *testing.T) {
	tmp := t.TempDir()
	c := New([]string{"fake-key"}, tmp, "", "")
	if c.IsConverted("nonexistent.py") {
		t.Error("IsConverted should be false for missing file")
	}
}

func TestIsConverted_present(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "test.go")
	os.WriteFile(outPath, []byte("package main"), 0644)
	c := New([]string{"fake-key"}, tmp, "", "")
	if !c.IsConverted("test.py") {
		t.Error("IsConverted should be true for existing .go file")
	}
}

func TestCopyNonCodeFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "readme.md")
	srcContent := "# Hello\n\nThis is a test.\n"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	dstPath := filepath.Join(dstDir, "sub", "readme.md")
	if err := CopyNonCodeFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyNonCodeFile: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading copied file: %v", err)
	}
	if string(data) != srcContent {
		t.Errorf("content = %q, want %q", string(data), srcContent)
	}
}

func TestCopyNonCodeFile_sourceMissing(t *testing.T) {
	dstDir := t.TempDir()
	err := CopyNonCodeFile("nonexistent.txt", filepath.Join(dstDir, "out.txt"))
	if err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestNew(t *testing.T) {
	c := New([]string{"key123"}, "/out", "/src", "test-model")
	if len(c.APIKeys) != 1 || c.APIKeys[0] != "key123" {
		t.Errorf("APIKeys = %v, want [key123]", c.APIKeys)
	}
	if c.OutputRoot != "/out" {
		t.Errorf("OutputRoot = %q", c.OutputRoot)
	}
	if c.SourceRoot != "/src" {
		t.Errorf("SourceRoot = %q", c.SourceRoot)
	}
	if c.ModelName != "test-model" {
		t.Errorf("ModelName = %q, want %q", c.ModelName, "test-model")
	}
	if c.ctx == nil {
		t.Error("ctx should not be nil")
	}
}

func TestNew_emptyModel_usesDefault(t *testing.T) {
	c := New([]string{"key123"}, "/out", "/src", "")
	if c.ModelName != DefaultModelName {
		t.Errorf("ModelName = %q, want %q", c.ModelName, DefaultModelName)
	}
}

func TestNew_emptyKeys_defaults(t *testing.T) {
	c := New(nil, "/out", "/src", "")
	if len(c.APIKeys) != 1 || c.APIKeys[0] != "" {
		t.Errorf("APIKeys = %v, want [\"\"]", c.APIKeys)
	}
}

func TestKeyRotation(t *testing.T) {
	c := New([]string{"key1", "key2", "key3"}, "/out", "/src", "")
	if c.CurrentKey() != "key1" {
		t.Errorf("CurrentKey = %q, want key1", c.CurrentKey())
	}
	if c.ExhaustedKeys() {
		t.Error("should not be exhausted yet")
	}

	c.advanceKey()
	if c.CurrentKey() != "key2" {
		t.Errorf("CurrentKey = %q, want key2", c.CurrentKey())
	}

	c.advanceKey()
	if c.CurrentKey() != "key3" {
		t.Errorf("CurrentKey = %q, want key3", c.CurrentKey())
	}

	c.advanceKey()
	if !c.ExhaustedKeys() {
		t.Error("should be exhausted")
	}
	if c.CurrentKey() != "" {
		t.Errorf("CurrentKey = %q, want empty", c.CurrentKey())
	}

	c.ResetKeys()
	if c.CurrentKey() != "key1" {
		t.Errorf("after ResetKeys, CurrentKey = %q, want key1", c.CurrentKey())
	}
}

func TestIsKeyError(t *testing.T) {
	tests := []struct {
		err  string
		want bool
	}{
		{"API_KEY_INVALID", true},
		{"API key expired", true},
		{"API key not found", true},
		{"RESOURCE_EXHAUSTED", true},
		{"quota exceeded", true},
		{"429 Too Many Requests", true},
		{"PERMISSION_DENIED", true},
		{"TLS handshake timeout", true},
		{"read tcp 192.168.0.5:53463->216.239.34.223:443: wsarecv", true},
		{"write tcp: connection refused", true},
		{"no such host", true},
		{"i/o timeout", true},
		{"deadline exceeded", true},
		{"network error", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isRetryableError(fmt.Errorf("%s", tt.err))
		if got != tt.want {
			t.Errorf("isRetryableError(%q) = %v, want %v", tt.err, got, tt.want)
		}
	}
	got := isRetryableError(nil)
	if got != false {
		t.Error("isRetryableError(nil) should be false")
	}
}

func TestDefaultModelName(t *testing.T) {
	if DefaultModelName == "" {
		t.Error("DefaultModelName should not be empty")
	}
	if !strings.Contains(DefaultModelName, "gemini") {
		t.Errorf("DefaultModelName should contain 'gemini', got %q", DefaultModelName)
	}
}

func TestSplitSource_smallFile(t *testing.T) {
	c := New([]string{"key"}, "", "", "")
	src := "line1\nline2\nline3"
	chunks := c.splitSource(src)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != src {
		t.Errorf("chunk = %q, want %q", chunks[0], src)
	}
}

func TestSplitSource_largeFile(t *testing.T) {
	c := New([]string{"key"}, "", "", "")
	c.outputTokenLimit = 100
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("this is a long line of source code number %d that should take many tokens to represent", i))
	}
	src := strings.Join(lines, "\n")
	chunks := c.splitSource(src)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for large file, got %d", len(chunks))
	}
	var joined strings.Builder
	for i, ch := range chunks {
		if ch == "" {
			t.Errorf("chunk %d is empty", i)
		}
		if i > 0 {
			joined.WriteString("\n")
		}
		joined.WriteString(ch)
	}
	if joined.String() != src {
		t.Error("rejoined chunks don't match original source")
	}
}

func TestSplitSource_zeroLimit(t *testing.T) {
	c := New([]string{"key"}, "", "", "")
	c.outputTokenLimit = 0
	src := "line1\nline2\nline3"
	chunks := c.splitSource(src)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk with zero limit, got %d", len(chunks))
	}
}
