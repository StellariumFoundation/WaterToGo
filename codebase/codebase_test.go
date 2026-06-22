package codebase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnvictor/watertogo/scanner"
)

func TestGenerate(t *testing.T) {
	testDir, err := filepath.Abs(filepath.Join("..", "test_codebase"))
	if err != nil {
		t.Fatalf("Failed to get abs path: %v", err)
	}

	scanResult, err := scanner.Scan(testDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	outputPath, err := Generate(testDir, scanResult)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	defer os.Remove(outputPath)

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Read codebase.md failed: %v", err)
	}
	content := string(data)

	t.Logf("codebase.md created at: %s (%d bytes)", outputPath, len(data))

	// Check header
	if !strings.Contains(content, "# Codebase: test_codebase") {
		t.Error("Missing header in codebase.md")
	}

	// Check separator exists
	if !strings.Contains(content, separator) {
		t.Error("Missing separator in codebase.md")
	}

	// Check FILE entries
	for _, expected := range []string{"src/main.js", "src/utils.ts", "lib/helpers.py", "lib/cli.rs", "config.json", "README.md", "assets/icon.svg"} {
		if !strings.Contains(content, "FILE: "+expected) {
			t.Errorf("Missing FILE entry for %s", expected)
		}
	}

	// Check NAME entries for files
	for _, expected := range []string{"main.js", "utils.ts", "helpers.py", "cli.rs"} {
		if !strings.Contains(content, "NAME: "+expected) {
			t.Errorf("Missing NAME entry for %s", expected)
		}
	}

	// Check DIR entries
	if !strings.Contains(content, "DIR: src") && !strings.Contains(content, "DIR: lib") {
		t.Error("Missing DIR entries")
	}

	// Check JS content appears
	if !strings.Contains(content, "function greet(name)") {
		t.Error("Missing file content in codebase.md")
	}

	// Check binary indicator for .svg (text-based but should appear)
	if !strings.Contains(content, "NAME: icon.svg") {
		t.Error("Missing icon.svg entry")
	}

	// Check that .gitignore-content files are listed (the .gitignore itself is not in scan)
	t.Log("codebase.md content preview:")
	lines := strings.Split(content, "\n")
	maxLines := 30
	if len(lines) < maxLines {
		maxLines = len(lines)
	}
	for i := 0; i < maxLines; i++ {
		t.Logf("  %s", lines[i])
	}
}

func TestGenerateContainsNonCodeFiles(t *testing.T) {
	testDir, err := filepath.Abs(filepath.Join("..", "test_codebase"))
	if err != nil {
		t.Fatalf("Failed to get abs path: %v", err)
	}

	scanResult, err := scanner.Scan(testDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	outputPath, err := Generate(testDir, scanResult)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	defer os.Remove(outputPath)

	data, _ := os.ReadFile(outputPath)
	content := string(data)

	// Ensure JSON file content is included (non-code text file)
	if !strings.Contains(content, `"app": "WaterToGo"`) {
		t.Error("JSON file content should be included in codebase.md")
	}

	// Ensure markdown content is included
	if !strings.Contains(content, "Test Codebase") {
		t.Error("README.md content should be included in codebase.md")
	}
}

func TestGenerateRespectsSkip(t *testing.T) {
	testDir, err := filepath.Abs(filepath.Join("..", "test_codebase"))
	if err != nil {
		t.Fatalf("Failed to get abs path: %v", err)
	}

	scanResult, err := scanner.Scan(testDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify codebase.md was NOT in scan result
	for _, f := range scanResult.Files {
		if f.RelPath == "codebase.md" {
			t.Error("codebase.md should not appear in scan results")
		}
	}

	outputPath, err := Generate(testDir, scanResult)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	defer os.Remove(outputPath)

	// Also verify running a second time doesn't include the first codebase.md
	scanResult2, err := scanner.Scan(testDir)
	if err != nil {
		t.Fatalf("Second scan failed: %v", err)
	}
	for _, f := range scanResult2.Files {
		if f.RelPath == "codebase.md" {
			t.Error("codebase.md should be skipped on second scan too")
		}
	}
}
