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

const DefaultModelName = "gemini-pro-latest"

var DefaultModels = []string{
	"gemini-pro-latest",
	"gemini-3.1-pro-preview",
	"gemini-flash-latest",
	"gemini-3.5-flash",
	"gemini-3-pro-preview",
	"gemini-3-flash-preview",
	"gemini-flash-lite-latest",
	"gemini-3.1-flash-lite",
	"gemini-3.1-flash-lite-preview",
	"gemma-4-31b-it",
	"gemini-2.5-pro",
	"gemini-2.5-flash",
}

type Converter struct {
	APIKeys          []string
	keyIndex         int
	OutputRoot       string
	SourceRoot       string
	ModelName        string
	Models           []string
	modelIndex       int
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
	models := DefaultModels
	startIdx := 0
	for i, m := range models {
		if m == modelName {
			startIdx = i
			break
		}
	}
	return &Converter{
		APIKeys:    apiKeys,
		OutputRoot: outputRoot,
		SourceRoot: sourceRoot,
		ModelName:  modelName,
		Models:     models,
		modelIndex: startIdx,
		ctx:        context.Background(),
	}
}

func (c *Converter) CurrentKey() string {
	if c.keyIndex < len(c.APIKeys) {
		return c.APIKeys[c.keyIndex]
	}
	return ""
}

func (c *Converter) CurrentModel() string {
	return c.ModelName
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
	if key == "" {
		return fmt.Errorf("api key is empty — all keys may be exhausted or none were configured")
	}
	client, err := genai.NewClient(c.ctx, &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("creating genai client: %w", err)
	}
	c.client = client

	savedStderr := os.Stderr
	os.Stderr = nil
	tok, err := tokenizer.NewLocalTokenizer(c.ModelName)
	os.Stderr = savedStderr
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
		c.outputTokenLimit = 0
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
		".ts":  "TypeScript",
		".py":  "Python",
		".rs":  "Rust",
	}[ext]
	if langName == "" {
		langName = "source"
	}

	for c.modelIndex < len(c.Models) {
		c.ModelName = c.Models[c.modelIndex]
		c.ResetKeys()

	keyLoop:
		for attempts := 0; attempts < len(c.APIKeys)+1; attempts++ {
			if err := c.initChat(); err != nil {
				if c.advanceKey() != "" {
					continue
				}
				break // all keys exhausted, try next model
			}

			chunks := c.splitSource(sourceCode)
			var fullGoCode strings.Builder

			if len(chunks) > 1 {
				ctxPrompt := fmt.Sprintf(`Here is the complete %s source file "%s" for context. I will ask you to convert it to Go part by part. Do not output anything yet.

--- Complete source file ---

%s`, langName, relPath, sourceCode)
				_, err := c.chat.SendMessage(c.ctx, genai.Part{Text: ctxPrompt})
				if err != nil {
					if c.advanceKey() != "" {
						continue keyLoop
					}
					break keyLoop
				}
			}

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

The entire file was provided above for context.
Return only the Go code for this part. More parts will follow.

--- Part %d ---

%s`, langName, i+1, len(chunks), relPath, i+1, chunk)
				default:
					prompt = fmt.Sprintf(`Convert the next part of a %s file to Go. This is part %d of %d.

Source file: %s

The entire file was provided above for context.
The chat history contains earlier parts and their Go conversions.
Convert only this part. Return only its Go code. Do NOT repeat code from earlier parts.

--- Part %d ---

%s`, langName, i+1, len(chunks), relPath, i+1, chunk)
				}

				resp, err := c.chat.SendMessage(c.ctx, genai.Part{Text: prompt})
				if err != nil {
					if c.advanceKey() != "" {
						continue keyLoop
					}
					break keyLoop
				}

				fullGoCode.WriteString(cleanGoCode(resp.Text()))
				if i < len(chunks)-1 {
					fullGoCode.WriteString("\n")
				}
			}

			// All chunks succeeded
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

		c.modelIndex++
	}

	return "", fmt.Errorf("all API keys and models exhausted")
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
