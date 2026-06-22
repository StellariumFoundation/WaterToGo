package converter

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/genai"
)

const modelName = "gemini-3.5-flash"

type Converter struct {
	APIKey       string
	CodebasePath string
	OutputRoot   string
	SourceRoot   string
	chat         *genai.Chat
	ctx          context.Context
	client       *genai.Client
}

func New(apiKey, codebasePath, outputRoot, sourceRoot string) *Converter {
	return &Converter{
		APIKey:       apiKey,
		CodebasePath: codebasePath,
		OutputRoot:   outputRoot,
		SourceRoot:   sourceRoot,
		ctx:          context.Background(),
	}
}

func (c *Converter) initChat() error {
	if c.chat != nil {
		return nil
	}

	client, err := genai.NewClient(c.ctx, &genai.ClientConfig{
		APIKey:  c.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("creating genai client: %w", err)
	}
	c.client = client

	codebaseData, err := os.ReadFile(c.CodebasePath)
	if err != nil {
		return fmt.Errorf("reading codebase: %w", err)
	}

	systemInstruction := fmt.Sprintf(`You are a Go code conversion expert. You convert JavaScript, TypeScript, Python, and Rust code to idiomatic Go.

The entire codebase context is provided below. Use it to understand the project structure, dependencies, patterns, and naming conventions.

CONTEXT (full codebase):
%s

Rules for every conversion:
1. Output ONLY valid Go code — no explanations, no markdown formatting
2. Preserve all logic, functionality, comments, and business rules
3. Use idiomatic Go patterns, proper error handling, Go conventions
4. Package name must match the directory structure
5. If the file is a test, use Go's testing package
6. Never lose any code or functionality

When given a source file, respond with ONLY the Go equivalent.`, string(codebaseData))

	temp := float32(0.2)
	chat, err := client.Chats.Create(c.ctx, modelName, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemInstruction}},
		},
		Temperature: &temp,
		MaxOutputTokens: 65536,
	}, nil)
	if err != nil {
		return fmt.Errorf("creating chat session: %w", err)
	}

	c.chat = chat
	return nil
}

func (c *Converter) IsConverted(relPath string) bool {
	goPath := c.goPath(relPath)
	_, err := os.Stat(goPath)
	return err == nil
}

func (c *Converter) goPath(relPath string) string {
	goName := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".go"
	return filepath.Join(c.OutputRoot, goName)
}

func (c *Converter) Convert(relPath, sourceCode string) (string, error) {
	if err := c.initChat(); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	langName := map[string]string{
		".js":  "JavaScript",
		".jsx": "JavaScript (JSX)",
		".ts":  "TypeScript",
		".tsx": "TypeScript (TSX)",
		".py":  "Python",
		".rs":  "Rust",
	}[ext]
	if langName == "" {
		langName = "source"
	}

	prompt := fmt.Sprintf(`Make a complete rewrite of this file in Golang in full and complete. Do not lose any code and keep the code as a Go class. Write idiomatic Go code.

Source file: %s
Original language: %s

--- Source code to convert ---

%s`, relPath, langName, sourceCode)

	resp, err := c.chat.SendMessage(c.ctx, genai.Part{Text: prompt})
	if err != nil {
		return "", fmt.Errorf("sending message: %w", err)
	}

	text := resp.Text()
	text = cleanGoCode(text)

	outPath := c.goPath(relPath)
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return outPath, nil
}

func cleanGoCode(text string) string {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "```go")
	if start >= 0 {
		text = text[start+5:]
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
	} else if start = strings.Index(text, "```"); start >= 0 {
		text = text[start+3:]
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
	}
	return strings.TrimSpace(text)
}

func CopyNonCodeFile(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
