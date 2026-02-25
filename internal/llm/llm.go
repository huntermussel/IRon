package llm

import (
	"fmt"
	"iron/internal/chat"
)

type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
)

func NewAdapter(provider Provider, model, baseURL string) (chat.Adapter, error) {
	switch provider {
	case ProviderOllama:
		return NewOllamaAdapter(model, baseURL)
	case ProviderOpenAI:
		return NewOpenAIAdapter(model, baseURL)
	case ProviderAnthropic:
		return NewAnthropicAdapter(model)
	case ProviderGemini:
		return NewGeminiAdapter(model, baseURL)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
