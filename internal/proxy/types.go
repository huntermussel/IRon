package proxy

type Config struct {
	Proxy  ProxyConfig  `json:"proxy"`
	Ollama OllamaConfig `json:"ollama"`
	RAG    RAGConfig   `json:"rag"`
	Search SearchConfig `json:"search"`
	Cache  CacheConfig  `json:"cache"`
}

type ProxyConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Upstream     string `json:"upstream"`
	APIKeyHeader string `json:"api_key_header"`
}

type OllamaConfig struct {
	BaseURL                    string `json:"base_url"`
	EmbeddingModel             string `json:"embedding_model"`
	CompressionModel           string `json:"compression_model"`
	FastModel                  string `json:"fast_model"`
	CompressionThresholdTokens int    `json:"compression_threshold_tokens"`
}

type RAGConfig struct {
	ChromaPath        string `json:"chroma_path"`
	DefaultCollection string `json:"default_collection"`
	TopK              int    `json:"top_k"`
}

type SearchConfig struct {
	MaxResults     int `json:"max_results"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

type CacheConfig struct {
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TTLHours            int     `json:"ttl_hours"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
