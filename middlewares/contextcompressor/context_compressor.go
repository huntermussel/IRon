package contextcompressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"iron/internal/proxy"
)

type ContextCompressor struct {
	ollamaURL        string
	compressionModel string
	thresholdTokens  int
	client          *http.Client
}

func New(ollamaURL, model string, thresholdTokens int) *ContextCompressor {
	return &ContextCompressor{ollamaURL: ollamaURL, compressionModel: model, thresholdTokens: thresholdTokens, client: &http.Client{Timeout: 120 * time.Second}}
}

func (m *ContextCompressor) Name() string { return "contextcompressor" }

func (m *ContextCompressor) Handle(req *proxy.ChatRequest, next func(*proxy.ChatRequest) (*proxy.ChatResponse, error)) (*proxy.ChatResponse, error) {
	tokenCount := estimateTokens(req)
	if tokenCount < m.thresholdTokens {
		return next(req)
	}
	compressed, err := m.summarizeConversation(req)
	if err != nil {
		log.Printf("[contextcompressor] summarization failed: %v", err)
		return next(req)
	}
	req.Messages = []proxy.ChatMessage{
		{Role: "system", Content: fmt.Sprintf("[COMPRESSED CONTEXT]\n%s\n[/COMPRESSED CONTEXT]", compressed)},
	}
	if len(req.Messages) >= 2 {
		req.Messages = append(req.Messages, req.Messages[len(req.Messages)-2:]...)
	}
	return next(req)
}

func (m *ContextCompressor) summarizeConversation(req *proxy.ChatRequest) (string, error) {
	var b strings.Builder
	for _, msg := range req.Messages {
		b.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	prompt := fmt.Sprintf(`Summarize the following conversation concisely, preserving key facts and decisions:\n\n%s`, b.String())
	body, _ := json.Marshal(map[string]any{"model": m.compressionModel, "prompt": prompt, "stream": false, "options": map[string]any{"num_predict": 512}})
	httpReq, _ := http.NewRequest("POST", m.ollamaURL+"/api/generate", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct{ Response string }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Response, nil
}

func estimateTokens(req *proxy.ChatRequest) int {
	count := 0
	for _, msg := range req.Messages {
		count += len(msg.Content) / 4
	}
	return count
}
