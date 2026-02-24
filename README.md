# IRon

IRon is a minimal Go 1.22 CLI chat app that layers a middleware “plugin” system on top of LangChainGo. It includes built-in middlewares for prompt cleaning, greetings, intent compression, token budgeting, coding tool schemas/execution, and a simple IR-based context retriever to trim token usage.

## Project Layout

- `cmd/iron/` – CLI entrypoint.
- `internal/chat/` – chat service (history, middleware dispatch, IR memory trim).
- `internal/llm/` – provider adapters (Ollama by default).
- `internal/middleware/` – middleware interfaces, chain, debug log, registry.
- `internal/memory/` – tiny KV + lexical retriever to keep prompts short.
- `middlewares/` – auto-loaded plugins. See per-plugin READMEs:
  - [greeting](middlewares/greeting/README.md)
  - [trash cleaner](middlewares/trashcleanner/README.md)
  - [intent compressor](middlewares/intentcompressor/README.md)
  - [token budget](middlewares/tokenbudget/README.md)
  - [coding tools](middlewares/codingtools/README.md)

## Run & Develop

```bash
go run ./cmd/iron                # start CLI (Ollama by default)
go build -o bin/iron ./cmd/iron  # build binary
go test ./...                    # run tests
```

Env vars:
- `IRON_MODEL` (default `llama3.2`)
- `IRON_PROVIDER` (`ollama`, `openai`, `azure`, `gemini`, `anthropic`; default `ollama`)
- `IRON_OLLAMA_URL` (default `http://localhost:11434`)

Middleware debug log is always written to `bin/middleware.debug.jsonl`.

## Middleware at a Glance

- Greeting short-circuits simple salutations.
- Trash cleaner drops filler/stopwords while keeping technical tokens.
- Intent compressor emits short intent labels/qualifiers.
- Token budget caps `MaxTokens` when provided.
- Coding tools inject tool schemas and execute ls/mkdir/find/diff/pwd/read/write when tool calls are supplied.

## IR Token Memory

The chat service keeps a tiny KV retriever (`internal/memory`) and:
- Prunes history to the last 8 messages.
- Prepends top lexical matches from prior turns to each user prompt.
- Indexes user prompts and assistant replies for later reuse.

This keeps prompts short while surfacing relevant context without full histories.
