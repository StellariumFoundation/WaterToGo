package converter

import (
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
	c := New("fake-key", "", tmp, "")
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
	c := New("fake-key", "", tmp, "")
	if c.IsConverted("nonexistent.py") {
		t.Error("IsConverted should be false for missing file")
	}
}

func TestIsConverted_present(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "test.go")
	os.WriteFile(outPath, []byte("package main"), 0644)
	c := New("fake-key", "", tmp, "")
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
	c := New("key123", "codebase.md", "/out", "/src")
	if c.APIKey != "key123" {
		t.Errorf("APIKey = %q, want %q", c.APIKey, "key123")
	}
	if c.CodebasePath != "codebase.md" {
		t.Errorf("CodebasePath = %q", c.CodebasePath)
	}
	if c.OutputRoot != "/out" {
		t.Errorf("OutputRoot = %q", c.OutputRoot)
	}
	if c.SourceRoot != "/src" {
		t.Errorf("SourceRoot = %q", c.SourceRoot)
	}
	if c.ctx == nil {
		t.Error("ctx should not be nil")
	}
}

func TestModelName(t *testing.T) {
	if modelName == "" {
		t.Error("modelName should not be empty")
	}
	if !strings.Contains(modelName, "gemini") {
		t.Errorf("modelName should contain 'gemini', got %q", modelName)
	}
}
