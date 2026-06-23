package scanner

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

var codeExts = map[string]bool{
	".js": true, ".ts": true, ".py": true, ".rs": true,
}

var alwaysSkip = map[string]bool{
	".git":        true,
	"0go0":        true,
	"codebase.md": true,
	"watertogo_config.json": true,
	"node_modules": true,
}

type FileEntry struct {
	Path     string
	RelPath  string
	IsDir    bool
	IsCode   bool
	IsBinary bool
	Size     int64
}

type ScanResult struct {
	Root   string
	Files  []FileEntry
}

func IsCodeFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return codeExts[ext]
}

func IsTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	textExts := map[string]bool{
		".md": true, ".json": true, ".yaml": true, ".yml": true,
		".toml": true, ".xml": true, ".html": true, ".css": true,
		".scss": true, ".less": true, ".sql": true, ".sh": true,
		".bat": true, ".ps1": true, ".env": true, ".cfg": true,
		".ini": true, ".conf": true, ".txt": true, ".csv": true,
		".svg": true, ".go": true, ".mod": true, ".sum": true,
		".proto": true, ".gradle": true, ".properties": true,
		".lock": true, ".h": true, ".c": true, ".cpp": true,
		".hpp": true, ".java": true, ".rb": true, ".php": true,
		".swift": true, ".kt": true, ".dart": true, ".vue": true,
		".svelte": true, ".astro": true, ".gitignore": true, ".dockerignore": true, ".editorconfig": true,
	}
	if textExts[ext] {
		return true
	}
	if codeExts[ext] {
		return true
	}
	return false
}

func shouldSkip(info os.FileInfo, relPath string, gi *gitignore.GitIgnore) bool {
	name := info.Name()
	if alwaysSkip[name] {
		return true
	}
	if gi != nil && gi.MatchesPath(relPath) {
		return true
	}
	return false
}

func Scan(root string) (*ScanResult, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var gi *gitignore.GitIgnore
	gitignorePath := filepath.Join(root, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		gi = gitignore.CompileIgnoreLines(strings.Split(string(data), "\n")...)
	}

	result := &ScanResult{Root: root}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		if shouldSkip(info, relPath, gi) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := FileEntry{
			Path:    path,
			RelPath: relPath,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
		}

		if !info.IsDir() {
			entry.IsCode = IsCodeFile(info.Name())
			entry.IsBinary = !IsTextFile(info.Name())
		}

		result.Files = append(result.Files, entry)
		return nil
	})

	return result, err
}
