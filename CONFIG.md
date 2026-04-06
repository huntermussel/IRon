# Configuration Reference

This document describes all configuration fields available in `config.json`.

## Full Configuration Example

See `config.json.example` for a complete template.

## Field Reference

### proxy

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `"0.0.0.0"` | Host to bind the proxy server |
| `port` | int | `8080` | Port to bind the proxy server |
| `upstream` | string | `"https://api.openai.com/v1"` | Upstream LLM endpoint (OpenAI-compatible) |
| `api_key_header` | string | `"x-api-key"` | Header name for passing the upstream API key |

### ollama

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base_url` | string | `"http://localhost:11434"` | Ollama API base URL |
| `embedding_model` | string | `"nomic-embed-text"` | Model used for semantic embeddings |
| `compression_model` | string | `"llama3.2:1b"` | Small model used for context summarization |
| `fast_model` | string | `"llama3.2:1b"` | Small model used for NLU intent classification |
| `compression_threshold_tokens` | int | `12000` | Token threshold for triggering context compression |

### chroma

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `"http://localhost:8000"` | ChromaDB server HTTP URL |

### cache

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `similarity_threshold` | float | `0.92` | Minimum cosine similarity (0.0–1.0) for cache hits |
| `ttl_hours` | int | `24` | Time-to-live for cached responses in hours |

### rag

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `chroma_path` | string | `"./chroma_data"` | Local path for ChromaDB persistence |
| `default_collection` | string | `"iron_context"` | ChromaDB collection name for RAG context |
| `top_k` | int | `5` | Number of top chunks to retrieve |

### search

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_results` | int | `5` | Maximum number of web search results |
| `timeout_seconds` | int | `10` | Timeout for web search requests |

## Environment Variables

Alternatively, you can set configuration via environment variables using the `IRON_PROXY_CONFIG` env var pointing to your config file:

```bash
IRON_PROXY_CONFIG=/path/to/config.json ./proxy
```

## Collections

The IRon proxy uses two ChromaDB collections:

- `iron_sem_cache`: Stores semantic cache entries (query embeddings → cached responses)
- `iron_context`: Stores RAG context documents for retrieval
