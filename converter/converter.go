package converter

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/genai"
	"google.golang.org/genai/tokenizer"
)

const DefaultModelName = "gemini-3-flash-preview"

type Converter struct {
	APIKeys          []string
	keyIndex         int
	OutputRoot       string
	SourceRoot       string
	ModelName        string
	outputTokenLimit int32
	chat             *genai.Chat
	ctx              context.Context
	client           *genai.Client
	tokenizer        *tokenizer.LocalTokenizer
}

func New(apiKeys []string, outputRoot, sourceRoot, modelName string) *Converter {
	if modelName == "" {
		modelName = DefaultModelName
	}
	if len(apiKeys) == 0 {
		apiKeys = []string{""}
	}
	return &Converter{
		APIKeys:    apiKeys,
		OutputRoot: outputRoot,
		SourceRoot: sourceRoot,
		ModelName:  modelName,
		ctx:        context.Background(),
	}
}

func (c *Converter) CurrentKey() string {
	if c.keyIndex < len(c.APIKeys) {
		return c.APIKeys[c.keyIndex]
	}
	return ""
}

func (c *Converter) ExhaustedKeys() bool {
	return c.keyIndex >= len(c.APIKeys)
}

func (c *Converter) ResetKeys() {
	c.keyIndex = 0
	c.client = nil
	c.chat = nil
}

func (c *Converter) initChat() error {
	if c.chat != nil {
		return nil
	}

	key := c.CurrentKey()
	client, err := genai.NewClient(c.ctx, &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("creating genai client: %w", err)
	}
	c.client = client

	tok, err := tokenizer.NewLocalTokenizer(c.ModelName)
	if err == nil {
		c.tokenizer = tok
	}

	getName := c.ModelName
	if !strings.HasPrefix(getName, "models/") {
		getName = "models/" + getName
	}
	modelInfo, err := client.Models.Get(c.ctx, getName, nil)
	if err == nil && modelInfo.OutputTokenLimit > 0 {
		c.outputTokenLimit = modelInfo.OutputTokenLimit
	} else {
		c.outputTokenLimit = 65536
	}

	temp := float32(1.0)
	topP := float32(0.95)
	thinkingLevel := genai.ThinkingLevelHigh
	maxOut := c.outputTokenLimit

	chat, err := client.Chats.Create(c.ctx, c.ModelName, &genai.GenerateContentConfig{
		Temperature:    &temp,
		MaxOutputTokens: maxOut,
		TopP:           &topP,
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: thinkingLevel,
		},
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

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "API_KEY_INVALID") ||
		strings.Contains(s, "API key expired") ||
		strings.Contains(s, "API key not found") ||
		strings.Contains(s, "RESOURCE_EXHAUSTED") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "429") ||
		strings.Contains(s, "PERMISSION_DENIED") ||
		strings.Contains(s, "TLS handshake timeout") ||
		strings.Contains(s, "read tcp") ||
		strings.Contains(s, "write tcp") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "deadline exceeded")
}

func (c *Converter) advanceKey() string {
	c.chat = nil
	c.client = nil
	c.keyIndex++
	return c.CurrentKey()
}

func (c *Converter) Convert(relPath, sourceCode string) (string, error) {
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

	for attempts := 0; attempts < len(c.APIKeys)+1; attempts++ {
		if err := c.initChat(); err != nil {
			if isRetryableError(err) {
				if c.advanceKey() != "" {
					continue
				}
				return "", fmt.Errorf("all API keys exhausted: %w", err)
			}
			return "", err
		}

		chunks := c.splitSource(sourceCode)
		var fullGoCode strings.Builder

		success := true
		for i, chunk := range chunks {
			var prompt string
			switch {
			case len(chunks) == 1:
				prompt = fmt.Sprintf(`Make a complete rewrite of this file in Golang in full and complete. Do not lose any code and keep the code as a Go class. Write idiomatic Go code.

Source file: %s
Original language: %s

--- Source code to convert ---

%s`, relPath, langName, sourceCode)
			case i == 0:
				prompt = fmt.Sprintf(`Convert this first part of a %s file to Go. This is part %d of %d.

Source file: %s

Return only the Go code for this part. More parts will follow.

--- Part %d ---

%s`, langName, i+1, len(chunks), relPath, i+1, chunk)
			default:
				prompt = fmt.Sprintf(`Convert the next part of a %s file to Go. This is part %d of %d.

Source file: %s

The chat history above contains the previous parts and their Go conversions.
Convert only this part. Return only its Go code. Do NOT repeat code from earlier parts.

--- Part %d ---

%s`, langName, i+1, len(chunks), relPath, i+1, chunk)
			}

			resp, err := c.chat.SendMessage(c.ctx, genai.Part{Text: prompt})
			if err != nil {
				if isRetryableError(err) {
					success = false
					if c.advanceKey() != "" {
						// Retry entire file from scratch with new key
						break
					}
					return "", fmt.Errorf("all API keys exhausted: %w", err)
				}
				return "", fmt.Errorf("sending message: %w", err)
			}

			fullGoCode.WriteString(cleanGoCode(resp.Text()))
			if i < len(chunks)-1 {
				fullGoCode.WriteString("\n")
			}
		}

		if success {
			outPath := c.goPath(relPath)
			outDir := filepath.Dir(outPath)
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return "", fmt.Errorf("creating output dir: %w", err)
			}
			if err := os.WriteFile(outPath, []byte(fullGoCode.String()), 0644); err != nil {
				return "", fmt.Errorf("writing file: %w", err)
			}
			return outPath, nil
		}
	}

	return "", fmt.Errorf("all API keys failed")
}

func (c *Converter) splitSource(sourceCode string) []string {
	limit := int(c.outputTokenLimit)
	if limit <= 0 {
		limit = 65536
	}

	sourceTokens := 0
	if c.tokenizer != nil {
		resp, err := c.tokenizer.CountTokens(
			[]*genai.Content{{Parts: []*genai.Part{{Text: sourceCode}}}},
			nil,
		)
		if err == nil {
			sourceTokens = int(resp.TotalTokens)
		}
	}
	if sourceTokens <= 0 {
		sourceTokens = len(sourceCode) / 4
	}

	expansionFactor := 3
	maxSourceTokens := limit / expansionFactor

	if sourceTokens <= maxSourceTokens {
		return []string{sourceCode}
	}

	lines := strings.Split(sourceCode, "\n")
	if len(lines) <= 1 {
		return []string{sourceCode}
	}

	tokensPerLine := sourceTokens / len(lines)
	if tokensPerLine < 1 {
		tokensPerLine = 1
	}
	linesPerChunk := maxSourceTokens / tokensPerLine
	if linesPerChunk < 1 {
		linesPerChunk = 1
	}

	var chunks []string
	for i := 0; i < len(lines); i += linesPerChunk {
		end := i + linesPerChunk
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, strings.Join(lines[i:end], "\n"))
	}

	if len(chunks) == 0 {
		return []string{sourceCode}
	}

	return chunks
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
