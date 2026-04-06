package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Proxy   ProxyConfig
	Ollama  OllamaConfig
	RAG     RAGConfig
	Search  SearchConfig
	Cache   CacheConfig
	Chroma  ChromaConfig
}

type ProxyConfig struct {
	Address string
	Port    int
}

type OllamaConfig struct {
	Address string
	Model   string
}

type ChromaConfig struct {
	Address string
}

type RAGConfig struct {
	Enabled            bool
	CollectionName     string
	TopK               int
	SimilarityThreshold float32
	CompressionRatio   float32
}

type SearchConfig struct {
	Enabled  bool
	Provider string
	APIKey   string
	Endpoint string
}

type CacheConfig struct {
	Enabled            bool
	TTL                time.Duration
	MaxEntries         int
	SimilarityThreshold float32
}

func Load() (*Config, error) {
	cfg := &Config{
		Proxy: ProxyConfig{
			Address: getEnv("PROXY_ADDRESS", "0.0.0.0"),
			Port:    getEnvInt("PROXY_PORT", 8080),
		},
		Ollama: OllamaConfig{
			Address: getEnv("OLLAMA_ADDRESS", "http://localhost:11434"),
			Model:   getEnv("OLLAMA_MODEL", "llama2"),
		},
		Chroma: ChromaConfig{
			Address: getEnv("CHROMA_ADDRESS", "http://localhost:8000"),
		},
		RAG: RAGConfig{
			Enabled:            getEnvBool("RAG_ENABLED", true),
			CollectionName:     getEnv("RAG_COLLECTION", "iron-docs"),
			TopK:               getEnvInt("RAG_TOP_K", 5),
			SimilarityThreshold: float32(getEnvFloat("RAG_THRESHOLD", 0.7)),
			CompressionRatio:   float32(getEnvFloat("RAG_COMPRESSION", 0.5)),
		},
		Search: SearchConfig{
			Enabled:  getEnvBool("SEARCH_ENABLED", false),
			Provider: getEnv("SEARCH_PROVIDER", "duckduckgo"),
			APIKey:   getEnv("SEARCH_API_KEY", ""),
			Endpoint: getEnv("SEARCH_ENDPOINT", ""),
		},
		Cache: CacheConfig{
			Enabled:            getEnvBool("CACHE_ENABLED", true),
			TTL:                getEnvDuration("CACHE_TTL", 1*time.Hour),
			MaxEntries:         getEnvInt("CACHE_MAX_ENTRIES", 1000),
			SimilarityThreshold: float32(getEnvFloat("CACHE_THRESHOLD", 0.85)),
		},
	}

	return cfg, nil
}

func (c *Config) GetProxyAddr() string {
	return fmt.Sprintf("%s:%d", c.Proxy.Address, c.Proxy.Port)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if floatVal, err := strconv.ParseFloat(val, 32); err == nil {
			return floatVal
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			return dur
		}
	}
	return defaultVal
}

var _ = fmt.Sprintf
var _ = os.Getenv
var _ = strconv.Atoi
var _ = time.Now
var _ = time.ParseDuration
