package pytools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	mw.Register(PyToolsMode{})
	mw.Register(PyToolsExec{})
}

const scriptsDir = "scripts"

// PyToolsMode discovers Python scripts in the scripts/ directory and
// registers them as available tools for the LLM.
type PyToolsMode struct{}

func (PyToolsMode) ID() string    { return "pytools_mode" }
func (PyToolsMode) Priority() int { return 85 }

func (PyToolsMode) ShouldLoad(_ context.Context, _ *mw.Event) bool { return true }

func (pm PyToolsMode) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	scripts, err := pm.discoverScripts()
	if err != nil {
		return mw.Decision{}, nil
	}

	if len(scripts) == 0 {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	for _, s := range scripts {
		params.Tools = append(params.Tools, s.ToTool())
	}

	return mw.Decision{
		OverrideParams: params,
		Reason:         "pytools_mode: injected dynamic python tools",
	}, nil
}

type scriptInfo struct {
	Name        string
	Description string
	Params      map[string]any
}

func (s scriptInfo) ToTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "py_" + s.Name,
			Description: s.Description,
			Parameters:  s.Params,
		},
	}
}

func (PyToolsMode) discoverScripts() ([]scriptInfo, error) {
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(scriptsDir)
	if err != nil {
		return nil, err
	}

	var infos []scriptInfo
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".py") {
			continue
		}
		name := strings.TrimSuffix(f.Name(), ".py")
		info := scriptInfo{
			Name:        name,
			Description: fmt.Sprintf("Execute python script: %s", f.Name()),
			Params: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		}

		// Heuristic: Read file to find a ToolSchema comment
		path := filepath.Join(scriptsDir, f.Name())
		content, _ := os.ReadFile(path)
		if meta := extractMetadata(string(content)); meta != nil {
			if meta.Description != "" {
				info.Description = meta.Description
			}
			if meta.Params != nil {
				info.Params = meta.Params
			}
		}

		infos = append(infos, info)
	}
	return infos, nil
}

type pyMetadata struct {
	Description string         `json:"description"`
	Params      map[string]any `json:"parameters"`
}

func extractMetadata(content string) *pyMetadata {
	// Look for a line like: # ToolSchema: {"description": "...", "parameters": {...}}
	marker := "# ToolSchema: "
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if idx := strings.Index(line, marker); idx != -1 {
			jsonStr := strings.TrimSpace(line[idx+len(marker):])
			var meta pyMetadata
			if err := json.Unmarshal([]byte(jsonStr), &meta); err == nil {
				return &meta
			}
		}
	}
	return nil
}

// PyToolsExec intercepts tool calls starting with "py_" and executes
// the corresponding script in scripts/ using python3.
type PyToolsExec struct{}

func (PyToolsExec) ID() string    { return "pytools_exec" }
func (PyToolsExec) Priority() int { return 80 }

func (PyToolsExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (PyToolsExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse {
		return mw.Decision{}, nil
	}

	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	if !ok || len(raw) == 0 {
		return mw.Decision{}, nil
	}

	var outputs []string
	handled := false
	for _, tc := range raw {
		if strings.HasPrefix(tc.Tool, "py_") {
			handled = true
			scriptName := strings.TrimPrefix(tc.Tool, "py_")
			out := runPyScript(scriptName, tc.Args)
			if strings.TrimSpace(out) != "" {
				outputs = append(outputs, fmt.Sprintf("### Python Tool: %s\n%s", scriptName, out))
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
		Reason:      "pytools_exec: dynamic script execution completed",
	}, nil
}

func runPyScript(name string, args map[string]any) string {
	scriptPath := filepath.Join(scriptsDir, name+".py")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Sprintf("Error: script %s not found in %s/", name, scriptsDir)
	}

	cmdArgs := []string{scriptPath}

	// Simple mapping of JSON args to CLI flags: {"root": "."} -> --root .
	for k, v := range args {
		// Use --key=value format for safety with complex strings
		val := fmt.Sprintf("%v", v)
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", k), val)
	}

	cmd := exec.Command("python3", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Execution Error: %v\nOutput: %s", err, string(out))
	}

	return string(out)
}
