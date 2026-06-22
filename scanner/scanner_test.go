package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	testDir := filepath.Join("..", "test_codebase")
	result, err := Scan(testDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.Root == "" {
		t.Fatal("Root should not be empty")
	}

	var files, dirs, codeFiles, binaryFiles int
	for _, f := range result.Files {
		if f.IsDir {
			dirs++
		} else {
			files++
			if f.IsCode {
				codeFiles++
			}
			if f.IsBinary {
				binaryFiles++
			}
		}
	}

	t.Logf("Root: %s", result.Root)
	t.Logf("Directories: %d", dirs)
	t.Logf("Files: %d (code: %d, binary: %d)", files, codeFiles, binaryFiles)

	if files == 0 {
		t.Error("Expected at least 1 file")
	}
	if dirs == 0 {
		t.Error("Expected at least 1 directory")
	}

	for _, f := range result.Files {
		t.Logf("  %s %s (code=%v, binary=%v, size=%d)",
			map[bool]string{true: "DIR", false: "FILE"}[f.IsDir],
			f.RelPath, f.IsCode, f.IsBinary, f.Size)
	}
}

func TestScanSkipsCodebaseMd(t *testing.T) {
	// Create a temporary codebase.md
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "codebase.md")
	os.WriteFile(tmpFile, []byte("test"), 0644)

	result, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, f := range result.Files {
		if f.RelPath == "codebase.md" {
			t.Error("codebase.md should be skipped")
		}
	}
}

func TestScanSkipsDotGit(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".git", "config"), []byte("test"), 0644)

	result, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, f := range result.Files {
		if f.RelPath == ".git" || f.RelPath == ".git/config" || f.RelPath == ".git/objects" {
			t.Errorf("Should skip .git, but found: %s", f.RelPath)
		}
	}
}

func TestScanSkipsZeroGoZero(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "0go0", "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "0go0", "src", "main.go"), []byte("package main"), 0644)

	result, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, f := range result.Files {
		if f.RelPath == "0go0" || f.RelPath == "0go0/src" || f.RelPath == "0go0/src/main.go" {
			t.Errorf("Should skip 0go0, but found: %s", f.RelPath)
		}
	}
}

func TestIsCodeFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"file.js", true},
		{"file.jsx", true},
		{"file.ts", true},
		{"file.tsx", true},
		{"file.py", true},
		{"file.rs", true},
		{"file.go", false},
		{"file.json", false},
		{"file.md", false},
	}

	for _, tt := range tests {
		got := IsCodeFile(tt.name)
		if got != tt.want {
			t.Errorf("IsCodeFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
