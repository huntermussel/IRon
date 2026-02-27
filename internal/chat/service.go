package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iron/internal/memory"
	"iron/internal/middleware"
	"iron/internal/skills"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type Service struct {
	adapter  Adapter
	history  []Message
	mws      *middleware.Chain
	mem      *memory.Store
	skillMgr *skills.Manager
}

type ServiceOption func(*Service)

func WithMiddlewareChain(chain *middleware.Chain) ServiceOption {
	return func(s *Service) {
		s.mws = chain
	}
}

func WithMemoryStore(mem *memory.Store) ServiceOption {
	return func(s *Service) {
		s.mem = mem
	}
}

func WithSkills(mgr *skills.Manager) ServiceOption {
	return func(s *Service) {
		s.skillMgr = mgr
	}
}

func NewService(adapter Adapter, opts ...ServiceOption) *Service {
	s := &Service{
		adapter:  adapter,
		history:  make([]Message, 0, 16),
		mem:      memory.NewStore(),
		skillMgr: skills.NewManager(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Clear() {
	s.history = s.history[:0]
}

func (s *Service) Send(ctx context.Context, input string) (string, error) {
	return s.SendWithContext(ctx, input, nil)
}

func (s *Service) SendWithContext(ctx context.Context, input string, mwCtx map[string]any) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty input")
	}

	// 1. Prepare history and context
	if len(s.history) > 20 {
		s.history = append([]Message{}, s.history[len(s.history)-20:]...)
	}

	inputWithContext := input
	if s.mem != nil {
		if hits := s.mem.Query("default", input, 2); len(hits) > 0 {
			inputWithContext = fmt.Sprintf("Context:\n%s\n\nUser: %s", strings.Join(hits, "\n"), input)
		}
	}

	// 2. Middleware (Pre-LLM)
	var llmParams *middleware.LLMParams
	if s.mws != nil {
		if mwCtx == nil {
			mwCtx = map[string]any{}
		}
		e := &middleware.Event{
			Name:     middleware.EventBeforeLLMRequest,
			UserText: inputWithContext,
			Context:  mwCtx,
			Params:   &middleware.LLMParams{},
		}
		results, err := s.mws.Dispatch(ctx, e)
		if err != nil {
			return "", err
		}

		updated, canceled := applyTextDecisions(inputWithContext, results)
		if canceled != nil && canceled.Cancel {
			// Middleware canceled the request (e.g. Greeting, Alarm Deterministic)
			// Return the replaced text immediately as the answer.
			if strings.TrimSpace(updated) != "" {
				s.history = append(s.history, Message{Role: RoleUser, Content: input})
				s.history = append(s.history, Message{Role: RoleAssistant, Content: updated})
				fmt.Println(updated) // Print to user since loop won't run
				return updated, nil
			}
			if strings.TrimSpace(canceled.Reason) == "" {
				return "", errors.New("request canceled by middleware")
			}
			return "", errors.New(canceled.Reason)
		}
		inputWithContext = updated
		llmParams = e.Params
	}

	// 3. Register available tools with LLM Params
	if s.skillMgr != nil {
		if llmParams == nil {
			llmParams = &middleware.LLMParams{}
		}
		for _, skill := range s.skillMgr.List() {
			llmParams.Tools = append(llmParams.Tools, llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        skill.Name(),
					Description: skill.Description(),
					Parameters:  skill.Parameters(),
				},
			})
		}
	}

	// Deduplicate tools
	if llmParams != nil && len(llmParams.Tools) > 1 {
		unique := make([]llms.Tool, 0, len(llmParams.Tools))
		seen := make(map[string]bool)
		for _, t := range llmParams.Tools {
			if t.Function == nil {
				unique = append(unique, t)
				continue
			}
			if !seen[t.Function.Name] {
				seen[t.Function.Name] = true
				unique = append(unique, t)
			}
		}
		llmParams.Tools = unique
	}

	// 5. Agent Loop (max 10 iterations)
	var finalResponse string
	const maxIterations = 10

	// Use a temporary slice for the current conversation turn
	currentHistory := append([]Message{}, s.history...)
	currentHistory = append(currentHistory, Message{Role: RoleUser, Content: inputWithContext})

	for i := 0; i < maxIterations; i++ {
		messages := make([]Message, 0, len(currentHistory)+1)
		sysPrompt := fmt.Sprintf("You are IRon, a terminal AI. You have access to tools. If a tool exists to answer the request, YOU MUST CALL THE TOOL. DO NOT generate text instead of calling tools. Time: %s", time.Now().Format(time.RFC1123))

		if llmParams != nil && len(llmParams.Tools) > 0 {
			var toolNames []string
			for _, t := range llmParams.Tools {
				if t.Function != nil {
					toolNames = append(toolNames, t.Function.Name)
				}
			}
			sysPrompt += "\n\nAvailable tools: " + strings.Join(toolNames, ", ") + ". ONLY use these tools."
		}

		messages = append(messages, Message{
			Role:    RoleSystem,
			Content: sysPrompt,
		})
		messages = append(messages, currentHistory...)

		var currentTextBuilder strings.Builder
		streamCallback := func(chunk string) {
			currentTextBuilder.WriteString(chunk)
			fmt.Print(chunk)
		}

		assistantText, toolCalls, err := s.adapter.ReplyStream(ctx, messages, llmParams, streamCallback)
		if i == 0 {
			fmt.Println()
		}

		if err != nil {
			return "", err
		}

		// Record Assistant message in current history
		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   assistantText,
			ToolCalls: toolCalls,
		}
		currentHistory = append(currentHistory, assistantMsg)

		if len(toolCalls) == 0 {
			finalResponse = assistantText
			break
		}

		// 2. Execute tools
		for _, tc := range toolCalls {
			fmt.Printf("🔧 Tool Call: %s(%s)\n", tc.Name, tc.Arguments)

			var result string

			// Try built-in skills first
			skill, found := s.skillMgr.Get(tc.Name)
			if found {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					result = fmt.Sprintf("Error: Invalid arguments JSON: %s", err)
				} else {
					result, err = skill.Execute(ctx, args)
					if err != nil {
						result = fmt.Sprintf("Error executing tool: %v", err)
					}
				}
			} else {
				// Not in built-in skills? Try Middleware execution!
				mwResult, mwErr := s.executeMiddlewareTool(ctx, mwCtx, tc)
				if mwErr == nil && mwResult != "" {
					result = mwResult
				} else {
					result = fmt.Sprintf("Error: Tool '%s' not found.", tc.Name)
				}
			}

			// Truncate result for display but keep full for LLM
			displayResult := result
			if len(displayResult) > 200 {
				displayResult = displayResult[:200] + "..."
			}
			fmt.Printf("   Result: %s\n", displayResult)

			// Add tool response to current history
			currentHistory = append(currentHistory, Message{
				Role:       RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
		}
	}

	// Finalize history with original user input (to keep history compact)
	s.history = append(s.history, Message{Role: RoleUser, Content: input})
	// Append only what was added in the loop (skipping UserWithContext)
	if len(currentHistory) > len(s.history) {
		s.history = append(s.history, currentHistory[len(s.history):]...)
	}

	// 6. Middleware (Post-LLM) - for the final response
	if s.mws != nil {
		e := &middleware.Event{
			Name:     middleware.EventAfterLLMResponse,
			UserText: inputWithContext,
			LLMText:  finalResponse,
			Context:  mwCtx,
		}
		results, err := s.mws.Dispatch(ctx, e)
		if err == nil {

			updated, canceled := applyTextDecisions(finalResponse, results)
			if canceled != nil && canceled.Cancel {
				// If middleware cancels here, it usually means it replaced the response
				// e.g. Greeting or cache. But wait, cache runs on BeforeLLM.
				// AfterLLM might modify text.
				if strings.TrimSpace(updated) != "" {
					finalResponse = updated
				}
			} else {
				finalResponse = updated
			}
		}
	}

	if s.mem != nil {
		s.mem.Index("default", inputWithContext)
		s.mem.Index("default", finalResponse)
	}

	return finalResponse, nil
}

// executeMiddlewareTool tries to execute a tool using the middleware chain
func (s *Service) executeMiddlewareTool(ctx context.Context, mwCtx map[string]any, tc ToolCall) (string, error) {
	if s.mws == nil {
		return "", fmt.Errorf("no middleware chain")
	}

	// Convert chat.ToolCall to middleware.ToolCall
	var args map[string]any
	json.Unmarshal([]byte(tc.Arguments), &args) // best effort

	mwTc := middleware.ToolCall{
		Tool: tc.Name,
		Args: args,
	}

	// Create a context specifically for this execution
	toolCtx := make(map[string]any)
	for k, v := range mwCtx {
		toolCtx[k] = v
	}
	toolCtx["tool_calls"] = []middleware.ToolCall{mwTc}

	e := &middleware.Event{
		Name:    middleware.EventAfterLLMResponse, // Middleware executors listen to this
		Context: toolCtx,
	}

	results, err := s.mws.Dispatch(ctx, e)
	if err != nil {
		return "", err
	}

	// Check if any middleware handled it (Cancel=true + ReplaceText)
	for _, r := range results {
		if r.Decision.Cancel && r.Decision.ReplaceText != nil {
			return *r.Decision.ReplaceText, nil
		}
	}

	return "", fmt.Errorf("no middleware handled this tool")
}

func applyTextDecisions(initial string, results []middleware.DecisionResult) (string, *middleware.Decision) {
	cur := strings.TrimSpace(initial)
	for _, r := range results {
		dec := r.Decision
		if dec.ReplaceText != nil {
			cur = strings.TrimSpace(*dec.ReplaceText)
		}
		if dec.Cancel {
			return cur, &dec
		}
	}
	return cur, nil
}
