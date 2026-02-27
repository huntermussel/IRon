package codingtools

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		funcTool("find", "Find files containing specific text. Recursively scans the directory. Use for locating keywords across the project.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":  map[string]any{"type": "string", "description": "Root path to start searching (e.g. '.')"},
				"query": map[string]any{"type": "string", "description": "Text string to search for within files"},
			},
			"required": []string{"path", "query"},
		}),
		funcTool("cp", "Copy a file or directory", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"src":  map[string]any{"type": "string"},
				"dest": map[string]any{"type": "string"},
			},
			"required": []string{"src", "dest"},
		}),
		funcTool("mv", "Move or rename a file or directory", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"src":  map[string]any{"type": "string"},
				"dest": map[string]any{"type": "string"},
			},
			"required": []string{"src", "dest"},
		}),
		funcTool("rm", "Remove a file or directory", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
			"required": []string{"path"},
		}),
		funcTool("git", "Execute a git command", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"args": map[string]any{"type": "string", "description": "Git arguments (e.g., 'status', 'commit -m \"msg\"', 'push')"},
			},
			"required": []string{"args"},
		}),
		funcTool("grep", "Search for patterns in a specific file. Use this when you know which file to search.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Path to the file"},
				"pattern": map[string]any{"type": "string", "description": "Regex pattern or string to search for"},
			},
			"required": []string{"path", "pattern"},
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
	case "grep":
		return toolGrep(tc.Args), true
	case "diff":
		return toolDiff(tc.Args), true
	case "cp":
		return toolCp(tc.Args), true
	case "mv":
		return toolMv(tc.Args), true
	case "rm":
		return toolRm(tc.Args), true
	case "git":
		return toolGit(tc.Args), true
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

func toolGrep(args map[string]any) string {
	path, err := cleanPath(args["path"])
	if err != nil {
		return "grep: " + err.Error()
	}
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "grep: pattern is required"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "grep: " + err.Error()
	}

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		// Fallback to literal contains if regex fails
		lines := strings.Split(string(data), "\n")
		var matches []string
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
				matches = append(matches, fmt.Sprintf("%d: %s", i+1, line))
			}
		}
		if len(matches) == 0 {
			return "grep: no matches"
		}
		return strings.Join(matches, "\n")
	}

	lines := strings.Split(string(data), "\n")
	var matches []string
	for i, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%d: %s", i+1, line))
		}
	}

	if len(matches) == 0 {
		return "grep: no matches"
	}
	return strings.Join(matches, "\n")
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

func toolCp(args map[string]any) string {
	src, err := cleanPath(args["src"])
	if err != nil {
		return "cp: " + err.Error()
	}
	dest, err := cleanPath(args["dest"])
	if err != nil {
		return "cp: " + err.Error()
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return "cp: " + err.Error()
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return "cp: " + err.Error()
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "cp: " + err.Error()
	}
	return fmt.Sprintf("ok: copied %s to %s", src, dest)
}

func toolMv(args map[string]any) string {
	src, err := cleanPath(args["src"])
	if err != nil {
		return "mv: " + err.Error()
	}
	dest, err := cleanPath(args["dest"])
	if err != nil {
		return "mv: " + err.Error()
	}

	if err := os.Rename(src, dest); err != nil {
		return "mv: " + err.Error()
	}
	return fmt.Sprintf("ok: moved %s to %s", src, dest)
}

func toolRm(args map[string]any) string {
	path, err := cleanPath(args["path"])
	if err != nil {
		return "rm: " + err.Error()
	}

	if err := os.RemoveAll(path); err != nil {
		return "rm: " + err.Error()
	}
	return fmt.Sprintf("ok: removed %s", path)
}

func toolGit(args map[string]any) string {
	gitArgs, ok := args["args"].(string)
	if !ok || gitArgs == "" {
		return "git: arguments are required"
	}

	// Split arguments carefully to avoid quoting issues
	fields := strings.Fields(gitArgs)

	cmd := exec.Command("git", fields...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("git error: %v\nOutput: %s", err, string(out))
	}
	return string(out)
}
