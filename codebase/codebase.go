package codebase

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/johnvictor/watertogo/scanner"
)

const separator = "####################"

func Generate(root string, scanResult *scanner.ScanResult) (string, error) {
	outputPath := filepath.Join(root, "codebase.md")
	f, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Codebase: %s\n\n", filepath.Base(root))

	entries := buildTree(scanResult.Files)
	writeEntries(f, entries, "")
	return outputPath, nil
}

type treeEntry struct {
	entry *scanner.FileEntry
	children []*treeEntry
}

func buildTree(entries []scanner.FileEntry) []*treeEntry {
	dirMap := make(map[string]*treeEntry)

	for i := range entries {
		e := &entries[i]
		dirMap[e.RelPath] = &treeEntry{entry: e}
	}

	var roots []*treeEntry
	for _, te := range dirMap {
		parentPath := filepath.Dir(te.entry.RelPath)
		if parentPath == "." {
			roots = append(roots, te)
		} else {
			parent, ok := dirMap[parentPath]
			if ok {
				parent.children = append(parent.children, te)
			} else {
				roots = append(roots, te)
			}
		}
	}

	sort.Slice(roots, func(i, j int) bool {
		return roots[i].entry.RelPath < roots[j].entry.RelPath
	})
	for _, te := range roots {
		sortChildren(te)
	}

	return roots
}

func sortChildren(te *treeEntry) {
	sort.Slice(te.children, func(i, j int) bool {
		return te.children[i].entry.RelPath < te.children[j].entry.RelPath
	})
	for _, c := range te.children {
		sortChildren(c)
	}
}

func writeEntries(f *os.File, entries []*treeEntry, prefix string) {
	for _, te := range entries {
		fmt.Fprintln(f, separator)

		if te.entry.IsDir {
			fmt.Fprintf(f, "DIR: %s\nNAME: %s\n\n", te.entry.RelPath, filepath.Base(te.entry.RelPath))
			writeEntries(f, te.children, te.entry.RelPath+"/")
		} else {
			fmt.Fprintf(f, "FILE: %s\nNAME: %s\n", te.entry.RelPath, filepath.Base(te.entry.RelPath))
			if te.entry.IsBinary {
				fmt.Fprintf(f, "SIZE: %d bytes (binary file)\n\n", te.entry.Size)
			} else {
				fmt.Fprintln(f)
				data, err := os.ReadFile(te.entry.Path)
				if err != nil {
					fmt.Fprintf(f, "[Error reading file: %v]\n\n", err)
					continue
				}
				content := string(data)
				if !strings.HasSuffix(content, "\n") {
					content += "\n"
				}
				fmt.Fprint(f, content)
			}
		}
	}
}
