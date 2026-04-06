# IRon Proxy — Integration Guide

This guide shows how to swap a direct LLM endpoint for the IRon proxy in popular chatbot frameworks. All requests pass through IRon's middleware pipeline (SemanticCache → RAG → WebSearch → ContextCompressor) before reaching your upstream LLM.

---

## Architecture Overview

```
[Chatbot] 
    |
    | (was: POST /v1/chat/completions)
    | (now: POST http://ourironendpoint/v1/chat/completions)
    v
[IRon Proxy]  ──→ SemanticCache (cache hit? return cached response)
    |               │
    |               └── cache miss ──→ RAG (intent=rag_request? inject context)
    |                                            │
    |                                            └── WebSearch (intent=web_search? inject web results)
    |                                                     │
    |                                                     └── ContextCompressor (token budget exceeded? summarize)
    |                                                               │
    v                                                               v
[Upstream LLM] ← (OpenAI-compatible /v1/chat/completions)
```

The key change: **your chatbot points to `http://ourironendpoint/v1/chat/completions` instead of `https://api.openai.com/v1/chat/completions`**. Everything else stays the same — the OpenAI SDK, LangChain, LlamaIndex, etc. work without modification.

---

## Before & After

| | Before (Direct) | After (IRon Proxy) |
|---|---|---|
| Endpoint | `https://api.openai.com/v1/chat/completions` | `http://ourironendpoint/v1/chat/completions` |
| API Key | Bearer token to OpenAI | Bearer token to your upstream LLM |
| Semantic Cache | ❌ | ✅ 0.92 similarity threshold |
| RAG Context | ❌ | ✅ ChromaDB on `rag_request` intent |
| Web Search | ❌ | ✅ DuckDuckGo on `web_search` intent |
| Context Compression | ❌ | ✅ Ollama summarization when >12k tokens |

---

## OpenAI Python SDK

Install:
```bash
pip install openai
```

**Before:**
```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-...",
    base_url="https://api.openai.com/v1"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "What's the latest on the Python 3.13 release?"}]
)
```

**After:**
```python
from openai import OpenAI

client = OpenAI(
    api_key="your-upstream-api-key",
    base_url="http://ourironendpoint/v1"   # <-- IRon proxy
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "What's the latest on the Python 3.13 release?"}]
)
```

The middleware pipeline runs **before** your request reaches the LLM. The `user` message is analyzed:
- IRon detects "latest" → triggers **WebSearch** → injects DuckDuckGo snippets → LLM answers with current info
- No code changes beyond the `base_url` and `api_key`

---

## LangChain Python

Install:
```bash
pip install langchain-openai
```

**Before:**
```python
from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage

llm = ChatOpenAI(
    model_name="gpt-4o",
    openai_api_key="sk-...",
    openai_api_base="https://api.openai.com/v1"
)

response = llm([HumanMessage(content="Summarize our stored documentation on deployment.")])
```

**After:**
```python
from langchain_openai import ChatOpenAI
from langchain.schema import HumanMessage

llm = ChatOpenAI(
    model_name="gpt-4o",
    openai_api_key="your-upstream-api-key",
    openai_api_base="http://ourironendpoint/v1"   # <-- IRon proxy
)

response = llm([HumanMessage(content="Summarize our stored documentation on deployment.")])
```

LangChain's `ChatOpenAI` wrapper sends to the same `/v1/chat/completions` endpoint. IRon receives the request and, if the message contains keywords like "stored" or "documentation", activates **RAG** — querying ChromaDB for relevant context and injecting it before forwarding to the LLM.

---

## LangChain.js / TypeScript

Install:
```bash
npm install @langchain/openai
```

**Before:**
```typescript
import { ChatOpenAI } from "@langchain/openai";

const llm = new ChatOpenAI({
  modelName: "gpt-4o",
  openAIApiKey: "sk-...",
  configuration: {
    basePath: "https://api.openai.com/v1",
  },
});
```

**After:**
```typescript
import { ChatOpenAI } from "@langchain/openai";

const llm = new ChatOpenAI({
  modelName: "gpt-4o",
  openAIApiKey: "your-upstream-api-key",
  configuration: {
    basePath: "http://ourironendpoint/v1",   // <-- IRon proxy
  },
});
```

---

## LlamaIndex (Python)

Install:
```bash
pip install llama-index-openai
```

**Before:**
```python
from llama_index.llms.openai import OpenAI

llm = OpenAI(
    model="gpt-4o",
    api_key="sk-..."
)
```

**After:**
```python
from llama_index.llms.openai import OpenAI

llm = OpenAI(
    model="gpt-4o",
    api_key="your-upstream-api-key",
    api_base="http://ourironendpoint/v1"   # <-- IRon proxy
)
```

---

## Node.js / Fetch (No SDK)

```javascript
// Before
const response = await fetch("https://api.openai.com/v1/chat/completions", {
  method: "POST",
  headers: {
    "Authorization": `Bearer ${process.env.OPENAI_KEY}`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    model: "gpt-4o",
    messages: [{ role: "user", content: "..." }]
  })
});

// After
const response = await fetch("http://ourironendpoint/v1/chat/completions", {
  method: "POST",
  headers: {
    "Authorization": `Bearer ${process.env.UPSTREAM_KEY}`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    model: "gpt-4o",
    messages: [{ role: "user", content: "..." }]
  })
});
```

---

## How the Middleware Pipeline Works

IRon intercepts every request and runs it through four middleware stages:

### 1. SemanticCache
- Embeds the user query using Ollama (`nomic-embed-text`)
- Queries ChromaDB for a cached response with ≥0.92 similarity
- **Cache hit** → returns the cached response immediately, no LLM call
- **Cache miss** → passes to next middleware

### 2. RAG (Retrieval-Augmented Generation)
- Runs the query through the NLU intent classifier
- If intent is `rag_request` (keywords: "context", "knowledge base", "documentation", "stored", "remember", "previously", "from earlier"):
  - Embeds the query
  - Retrieves top-5 chunks from ChromaDB collection `iron_context`
  - Injects results as a `[RAG CONTEXT]` system message
- **No RAG signal** → passes to next middleware

### 3. WebSearch
- If intent is `web_search` (keywords: "current", "latest", "today", "news", "weather", "search for", "find on the web"):
  - Scrapes DuckDuckGo HTML results via `goquery`
  - Returns top-5 results with title, URL, and snippet
  - Injects as a `[WEB SEARCH RESULTS]` system message
- **No search signal** → passes to next middleware

### 4. ContextCompressor
- Counts total tokens across all messages
- If token count exceeds 12,000:
  - Asks Ollama (`llama3.2:1b`) to summarize the conversation
  - Replaces the message history with the compressed summary
- **Within budget** → passes to upstream LLM unchanged

---

## NLU Intent Routing

IRon classifies every user query into one of five intents:

| Intent | Trigger | Middleware Action |
|---|---|---|
| `simple_query` | Default | No augmentation |
| `complex_task` | Keywords: plan, analyze, compare, design | No augmentation (chain-of-thought) |
| `rag_request` | Keywords: context, knowledge base, documentation, stored, remember, previously, from earlier | Inject ChromaDB context |
| `web_search` | Keywords: current, latest, today, news, weather, search for, find on the web | Inject DuckDuckGo results |
| `code_generation` | Keywords: write code, generate, implement, or language patterns | No augmentation |

Keyword matching runs first (fast path). If no keyword matches, Ollama classifies with the `fast_model` (`llama3.2:1b`).

---

## Configuration

IRon reads from `config.json` (or `IRON_PROXY_CONFIG` env var):

```json
{
  "proxy": {
    "host": "0.0.0.0",
    "port": 8080,
    "upstream": "https://api.openai.com/v1",
    "api_key_header": "x-api-key"
  },
  "ollama": {
    "base_url": "http://localhost:11434",
    "embedding_model": "nomic-embed-text",
    "compression_model": "llama3.2:1b",
    "fast_model": "llama3.2:1b"
  },
  "rag": {
    "chroma_path": "./chroma_data",
    "default_collection": "iron_context",
    "top_k": 5
  },
  "search": {
    "max_results": 5,
    "timeout_seconds": 10
  },
  "cache": {
    "similarity_threshold": 0.92,
    "ttl_hours": 24
  }
}
```

### Key fields:
- **`proxy.upstream`** — your actual LLM endpoint (e.g. OpenAI, Azure, self-hosted)
- **`proxy.api_key_header`** — header name for the upstream API key (IRon reads it from incoming requests and forwards it)
- **`ollama.*`** — local Ollama instance for embeddings and small models
- **`cache.similarity_threshold`** — minimum cosine similarity (0.0–1.0) for cache hits
- **`cache.ttl_hours`** — TTL for cached responses in ChromaDB

---

## Running IRon

```bash
# From the IRon repo root
go build -o bin/proxy ./cmd/proxy

# With config file
./bin/proxy --config config.json

# With env var
IRON_PROXY_CONFIG=config.json ./bin/proxy

# Docker
docker run -p 8080:8080 \
  -v $(pwd)/config.json:/app/config.json \
  -e IRON_PROXY_CONFIG=/app/config.json \
  iron-proxy
```

IRon listens on `0.0.0.0:8080` by default. Your chatbots connect to `http://ourironendpoint:8080/v1/chat/completions`.

---

## Health Check

```bash
curl http://ourironendpoint:8080/health
# {"status":"ok"}
```

---

## Middleware Toggle

To disable a middleware, simply don't register it in the pipeline in `cmd/proxy/main.go`:

```go
// Example: disable web search
middlewares := []proxy.Middleware{
    semanticcache.New("http://localhost:8000", embedClient, cfg.Cache.SimilarityThreshold, cfg.Cache.TTLHours),
    rag.New("http://localhost:8000", cfg.RAG.ChromaPath, cfg.RAG.DefaultCollection, cfg.RAG.TopK, embedClient, nluRouter),
    // websearch.New(nluRouter, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds), // disabled
    contextcompressor.New(cfg.Ollama.BaseURL, cfg.Ollama.CompressionModel, 12000),
}
```
