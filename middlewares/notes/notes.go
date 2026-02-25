package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	store, err := newNotesStore(defaultNotesPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not initialize notes store: %v\n", err)
	}
	mw.Register(NotesMode{})
	mw.Register(NotesExec{store: store})
}

func defaultNotesPath() string {
	return filepath.Join("bin", "notes.json")
}

func noteTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "notes",
			Description: "Save, view, list, or delete personal notes and reminders.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "One of add, view, list, delete.",
						"enum":        []string{"add", "view", "list", "delete"},
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Note title (required for add/view/delete).",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "Note body (required for add).",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "Optional substring to filter listed notes.",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

// NotesMode injects the notes tool schema before the LLM call.
type NotesMode struct{}

func (NotesMode) ID() string    { return "notes_mode" }
func (NotesMode) Priority() int { return 70 }

func (NotesMode) ShouldLoad(_ context.Context, e *mw.Event) bool {
	return e != nil && e.Name == mw.EventBeforeLLMRequest
}

func (NotesMode) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}
	params := e.Params
	if params == nil {
		params = &mw.LLMParams{}
	} else {
		copyParams := *params
		params = &copyParams
	}
	if !toolRegistered(params.Tools, "notes") {
		params.Tools = append(params.Tools, noteTool())
	}
	params.ToolChoice = "auto"
	return mw.Decision{OverrideParams: params, Reason: "notes_mode: inject tool schema"}, nil
}

func toolRegistered(tools []llms.Tool, name string) bool {
	for _, t := range tools {
		if t.Function != nil && t.Function.Name == name {
			return true
		}
	}
	return false
}

// NotesExec runs notes tool calls after the model responds.
type NotesExec struct {
	store *notesStore
}

func (NotesExec) ID() string    { return "notes_exec" }
func (NotesExec) Priority() int { return 60 }

func (NotesExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok && len(raw) > 0
}

func (n NotesExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse || e.Context == nil {
		return mw.Decision{}, nil
	}
	calls, _ := e.Context["tool_calls"].([]mw.ToolCall)
	var outputs []string
	for _, tc := range calls {
		if tc.Tool != "notes" {
			continue
		}
		outputs = append(outputs, runNotesCall(n.store, tc))
	}
	if len(outputs) == 0 {
		return mw.Decision{}, nil
	}
	text := strings.Join(outputs, "\n\n")
	return mw.Decision{Cancel: true, ReplaceText: &text, Reason: "notes_exec"}, nil
}

func runNotesCall(store *notesStore, tc mw.ToolCall) string {
	action := strings.ToLower(stringArg(tc.Args, "action"))
	title := stringArg(tc.Args, "title")
	body := strings.TrimSpace(stringArg(tc.Args, "body"))
	query := stringArg(tc.Args, "query")
	if store == nil {
		return "notes store unavailable"
	}
	switch action {
	case "add":
		if title == "" || body == "" {
			return "notes.add requires title and body"
		}
		if err := store.Add(title, body); err != nil {
			return fmt.Sprintf("notes.add failed: %v", err)
		}
		return fmt.Sprintf("saved note '%s'", title)
	case "view", "get":
		if title == "" {
			return "notes.view requires title"
		}
		val, ok := store.Get(title)
		if !ok {
			return fmt.Sprintf("note '%s' not found", title)
		}
		return fmt.Sprintf("%s:\n%s", title, val)
	case "list":
		items := store.List(query)
		if len(items) == 0 {
			return "no notes found"
		}
		return "notes:\n" + strings.Join(items, "\n")
	case "delete", "remove":
		if title == "" {
			return "notes.delete requires title"
		}
		deleted, err := store.Delete(title)
		if err != nil {
			return fmt.Sprintf("notes.delete failed: %v", err)
		}
		if !deleted {
			return fmt.Sprintf("note '%s' not found", title)
		}
		return fmt.Sprintf("deleted note '%s'", title)
	default:
		return fmt.Sprintf("unsupported notes action '%s'", action)
	}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key]; ok {
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		case fmt.Stringer:
			return strings.TrimSpace(t.String())
		}
	}
	return ""
}

type notesStore struct {
	mu    sync.RWMutex
	path  string
	notes map[string]noteEntry
}

type noteEntry struct {
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

func newNotesStore(path string) (*notesStore, error) {
	if path == "" {
		path = defaultNotesPath()
	}
	store := &notesStore{
		path:  path,
		notes: map[string]noteEntry{},
	}
	if err := store.load(); err != nil {
		return store, err
	}
	return store, nil
}

func (s *notesStore) Add(title, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	entry := noteEntry{Body: body, Created: now, Updated: now}
	if prev, ok := s.notes[title]; ok {
		entry.Created = prev.Created
		entry.Updated = now
	}
	s.notes[title] = entry
	return s.persistLocked()
}

func (s *notesStore) Get(title string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.notes[title]
	if !ok {
		return "", false
	}
	return entry.Body, true
}

func (s *notesStore) List(filter string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(filter))
	out := make([]string, 0, len(s.notes))
	for title, entry := range s.notes {
		if q != "" {
			if !strings.Contains(strings.ToLower(title), q) && !strings.Contains(strings.ToLower(entry.Body), q) {
				continue
			}
		}
		out = append(out, fmt.Sprintf("%s (updated %s)", title, entry.Updated.Format(time.RFC1123)))
	}
	sort.Strings(out)
	return out
}

func (s *notesStore) Delete(title string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notes[title]; !ok {
		return false, nil
	}
	delete(s.notes, title)
	return true, s.persistLocked()
}

func (s *notesStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data := map[string]noteEntry{}
	for k, v := range s.notes {
		data[k] = v
	}
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, buf, 0o644)
}

func (s *notesStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload map[string]noteEntry
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range payload {
		s.notes[k] = v
	}
	return nil
}
