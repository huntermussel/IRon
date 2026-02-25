package llm

import (
	"fmt"
	"iron/internal/chat"
)

type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderOpenAI    Provider = "openai"
	ProviderAzure     Provider = "azure"
	ProviderGemini    Provider = "gemini"
	ProviderAnthropic Provider = "anthropic"
)

func NewAdapter(provider Provider, model, baseURL string) (chat.Adapter, error) {
	switch provider {
	case ProviderOllama:
		return NewOllamaAdapter(model, baseURL)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
