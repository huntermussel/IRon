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
			inputWithContext = "Context from memory:\n- " + strings.Join(hits, "\n- ") + "\n\n" + input
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
				s.history = append(s.history, Message{Role: RoleUser, Content: inputWithContext})
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

	// 4. Construct messages for this turn
	messages := make([]Message, 0, len(s.history)+2)

	// System Prompt
	systemPrompt := fmt.Sprintf("You are IRon, a capable AI assistant. Current time: %s. ", time.Now().Format(time.RFC1123))
	if len(llmParams.Tools) > 0 {
		systemPrompt += "You have access to tools. Use them to answer questions or perform actions. " +
			"When you use a tool, I will execute it and give you the result. "
	}
	messages = append(messages, Message{Role: RoleSystem, Content: systemPrompt})

	messages = append(messages, s.history...)
	messages = append(messages, Message{Role: RoleUser, Content: inputWithContext})

	// 5. Agent Loop (max 5 iterations)
	var finalResponse string
	const maxIterations = 5

	for i := 0; i < maxIterations; i++ {
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

		// If no tool calls, we are done.
		if len(toolCalls) == 0 {
			finalResponse = assistantText
			s.history = append(s.history, Message{Role: RoleUser, Content: inputWithContext})
			s.history = append(s.history, Message{Role: RoleAssistant, Content: finalResponse})
			break
		}

		// If tools were called:
		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   assistantText,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMsg)

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
				// We simulate an AfterLLMResponse event containing JUST this tool call
				// to see if any middleware picks it up.
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

			messages = append(messages, Message{
				Role:       RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
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
