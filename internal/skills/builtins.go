package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ShellSkill executes shell commands.
type ShellSkill struct{}

func (s *ShellSkill) Name() string { return "shell" }
func (s *ShellSkill) Description() string {
	return "Executes a shell command. Use this to run system commands, list files, etc."
}
func (s *ShellSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default 30).",
			},
		},
		"required": []string{"command"},
	}
}
func (s *ShellSkill) Execute(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := 30 * time.Second
	if t, ok := args["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
	}

	if len(output) > 4000 {
		output = output[:4000] + "\n...(truncated)"
	}
	return strings.TrimSpace(output), nil
}

// FileSkill reads and writes files.
type FileSkill struct{}

func (f *FileSkill) Name() string { return "file" }
func (f *FileSkill) Description() string {
	return "Reads or writes files on the local filesystem."
}
func (f *FileSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "write"},
				"description": "The operation to perform.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The file path.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write (for write method).",
			},
			"append": map[string]any{
				"type":        "boolean",
				"description": "Append to file instead of overwriting (for write method).",
			},
		},
		"required": []string{"method", "path"},
	}
}
func (f *FileSkill) Execute(ctx context.Context, args map[string]any) (string, error) {
	method, _ := args["method"].(string)
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	switch method {
	case "read":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read failed: %w", err)
		}
		content := string(data)
		if len(content) > 8000 {
			content = content[:8000] + "\n...(truncated)"
		}
		return content, nil

	case "write":
		content, _ := args["content"].(string)
		appendMode, _ := args["append"].(bool)
		flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if appendMode {
			flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}

		// Ensure directory exists
		_ = os.MkdirAll(filepath.Dir(path), 0755)

		file, err := os.OpenFile(path, flags, 0644)
		if err != nil {
			return "", fmt.Errorf("open failed: %w", err)
		}
		defer file.Close()

		if _, err := file.WriteString(content); err != nil {
			return "", fmt.Errorf("write failed: %w", err)
		}
		return fmt.Sprintf("Written %d bytes to %s", len(content), path), nil

	default:
		return "", fmt.Errorf("unknown method: %s", method)
	}
}

// FetchSkill fetches URLs.
type FetchSkill struct{}

func (f *FetchSkill) Name() string { return "fetch" }
func (f *FetchSkill) Description() string {
	return "Fetches content from a URL."
}
func (f *FetchSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch.",
			},
		},
		"required": []string{"url"},
	}
}
func (f *FetchSkill) Execute(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64KB limit
	if err != nil {
		return "", err
	}

	// Simple text extraction (strip HTML tags)
	text := stripHTML(string(body))
	if len(text) > 4000 {
		text = text[:4000] + "\n...(truncated)"
	}
	return fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, text), nil
}

func stripHTML(s string) string {
	// Minimal HTML stripper
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
