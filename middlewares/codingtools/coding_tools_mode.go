package codingtools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	mw.Register(CodingToolsMode{})
	mw.Register(CodingToolsExec{})
}

// CodingToolsMode injects a minimal set of coding/file tools for LLM tool-calling.
// Runs on before_llm_request and ensures the tools schema is present.
type CodingToolsMode struct{}

func (CodingToolsMode) ID() string    { return "coding_tools_mode" }
func (CodingToolsMode) Priority() int { return 85 }

func (CodingToolsMode) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["coding_tools_mode"].(bool); ok {
		return v
	}
	return true
}

func (CodingToolsMode) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	tools := baseTools()
	params.Tools = append(params.Tools, tools...)
	params.ToolChoice = "auto"

	return mw.Decision{
		OverrideParams: params,
		Reason:         "coding_tools_mode: injected tools",
	}, nil
}

func baseTools() []llms.Tool {
	return []llms.Tool{
		funcTool("ls", "List files in a directory. ALWAYS use relative paths. Use '.' for the current directory.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative path to the directory (e.g. '.', 'internal', '../')"},
			},
			"required": []string{"path"},
		}),
		funcTool("mkdir", "Create directory", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
			"required": []string{"path"},
		}),
		funcTool("find", "Search text", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":  map[string]any{"type": "string", "description": "root path"},
				"query": map[string]any{"type": "string", "description": "text to find"},
			},
			"required": []string{"path", "query"},
		}),
		funcTool("diff", "Diff paths", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "string"},
				"b": map[string]any{"type": "string"},
			},
			"required": []string{"a", "b"},
		}),
		funcTool("pwd", "Show the current working directory path. Use this first to orient yourself.", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
		funcTool("read_file", "Read a file. Use relative paths. If you need to summarize a codebase, read files one by one after listing them.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative path to the file"},
			},
			"required": []string{"path"},
		}),
		funcTool("write_file", "Write file", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		}),
	}
}

func funcTool(name, desc string, params any) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}

// CodingToolsExec executes supported coding tools when tool calls are provided
// via Event.Context["tool_calls"] (type []mw.ToolCall). It runs on
// after_llm_response and returns a combined text, canceling further processing.
type CodingToolsExec struct{}

func (CodingToolsExec) ID() string    { return "coding_tools_exec" }
func (CodingToolsExec) Priority() int { return 80 }

func (CodingToolsExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (CodingToolsExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse {
		return mw.Decision{}, nil
	}
	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	if !ok || len(raw) == 0 {
		return mw.Decision{}, nil
	}

	outputs := make([]string, 0, len(raw))
	handled := false
	for _, tc := range raw {
		out, ok := runTool(tc)
		if ok {
			handled = true
			if strings.TrimSpace(out) != "" {
				outputs = append(outputs, out)
			}
		}
	}
	if !handled {
		return mw.Decision{}, nil
	}
	text := strings.Join(outputs, "\n\n")
	return mw.Decision{
		Cancel:      true,
		ReplaceText: &text,
		Reason:      "coding_tools_exec",
	}, nil
}

/* ---------------------------- Tool runners ---------------------------- */

func runTool(tc mw.ToolCall) (string, bool) {
	switch tc.Tool {
	case "ls":
		return toolLS(tc.Args), true
	case "mkdir":
		return toolMkdir(tc.Args), true
	case "pwd":
		return toolPwd(), true
	case "read_file":
		return toolRead(tc.Args), true
	case "write_file":
		return toolWrite(tc.Args), true
	case "find":
		return toolFind(tc.Args), true
	case "diff":
		return toolDiff(tc.Args), true
	default:
		return "", false
	}
}

func cleanPath(arg any) (string, error) {
	s, ok := arg.(string)
	if !ok {
		return "", fmt.Errorf("path must be string")
	}
	if s == "" {
		return "", fmt.Errorf("path is empty")
	}
	return filepath.Clean(s), nil
}

func toolPwd() string {
	p, err := os.Getwd()
	if err != nil {
		return "pwd: " + err.Error()
	}
	return p
}

func toolLS(args map[string]any) string {
	p, err := cleanPath(args["path"])
	if err != nil {
		return "ls: " + err.Error()
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return "ls: " + err.Error()
	}
	var b strings.Builder
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func toolMkdir(args map[string]any) string {
	p, err := cleanPath(args["path"])
	if err != nil {
		return "mkdir: " + err.Error()
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "mkdir: " + err.Error()
	}
	return "ok: " + p
}

func toolRead(args map[string]any) string {
	p, err := cleanPath(args["path"])
	if err != nil {
		return "read_file: " + err.Error()
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "read_file: " + err.Error()
	}
	return string(data)
}

func toolWrite(args map[string]any) string {
	p, err := cleanPath(args["path"])
	if err != nil {
		return "write_file: " + err.Error()
	}
	content, ok := args["content"].(string)
	if !ok {
		return "write_file: content must be string"
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return "write_file: " + err.Error()
	}
	return "ok: wrote " + p
}

func toolFind(args map[string]any) string {
	root, err := cleanPath(args["path"])
	if err != nil {
		return "find: " + err.Error()
	}
	q, ok := args["query"].(string)
	if !ok || strings.TrimSpace(q) == "" {
		return "find: query must be string"
	}
	q = strings.ToLower(q)
	var matches []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(strings.ToLower(string(data)), q) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "find: " + err.Error()
	}
	if len(matches) == 0 {
		return "find: no matches"
	}
	return strings.Join(matches, "\n")
}

func toolDiff(args map[string]any) string {
	a, err := cleanPath(args["a"])
	if err != nil {
		return "diff: " + err.Error()
	}
	b, err := cleanPath(args["b"])
	if err != nil {
		return "diff: " + err.Error()
	}
	da, err := os.ReadFile(a)
	if err != nil {
		return "diff: " + err.Error()
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return "diff: " + err.Error()
	}
	if string(da) == string(db) {
		return "diff: files are identical"
	}
	return fmt.Sprintf("diff: files differ (%s vs %s)", a, b)
}
