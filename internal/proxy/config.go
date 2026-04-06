package proxy

import (
	"encoding/json"
	"os"
)

func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("IRON_PROXY_CONFIG")
	}
	if path == "" {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			Upstream:     "https://api.openai.com/v1",
			APIKeyHeader: "x-api-key",
		},
		Ollama: OllamaConfig{
			BaseURL:                    "http://localhost:11434",
			EmbeddingModel:             "nomic-embed-text",
			CompressionModel:           "llama3.2:1b",
			FastModel:                  "llama3.2:1b",
			CompressionThresholdTokens: 12000,
		},
		RAG: RAGConfig{
			ChromaPath:        "./chroma_data",
			DefaultCollection: "iron_context",
			TopK:              5,
		},
		Search: SearchConfig{
			MaxResults:     5,
			TimeoutSeconds: 10,
		},
		Cache: CacheConfig{
			SimilarityThreshold: 0.92,
			TTLHours:            24,
		},
	}
}
