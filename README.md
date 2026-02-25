# IRon

IRon is a minimal Go 1.22 CLI chat app that layers a middleware “plugin” system on top of LangChainGo. It includes built-in middlewares for NLU-driven intent handling, prompt cleaning, greetings, intent compression, token budgeting, coding tool schemas/execution, and a simple IR-based context retriever to trim token usage.

## Project Layout

- `cmd/iron/` – CLI entrypoint.
- `internal/chat/` – chat service (history, middleware dispatch, IR memory trim).
- `internal/llm/` – provider adapters (Ollama by default).
- `internal/middleware/` – middleware interfaces, chain, debug log, registry.
- `internal/nlu/` – Template-based NLU engine for intent matching and entity extraction.
- `internal/memory/` – tiny KV + lexical retriever to keep prompts short.
- `middlewares/` – auto-loaded plugins:
  - `alarm`: Handles "set alarm for {time}" intents.
  - `weather`: Handles "weather in {location}" intents.
  - `device`: Handles "turn {state} {device}" intents.
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

- **NLU Tools (Alarm, Weather, Device):** Deterministically handle specific commands using template matching, bypassing the LLM for speed and reliability.
- **Greeting:** Short-circuits simple salutations.
- **Trash Cleaner:** Drops filler/stopwords while keeping technical tokens.
- **Intent Compressor:** Emits short intent labels/qualifiers.
- **Token Budget:** Caps `MaxTokens` when provided.
- **Coding Tools:** Inject tool schemas and execute ls/mkdir/find/diff/pwd/read/write when tool calls are supplied.

## IR Token Memory

The chat service keeps a tiny KV retriever (`internal/memory`) and:
- Prunes history to the last 8 messages.
- Prepends top lexical matches from prior turns to each user prompt.
- Indexes user prompts and assistant replies for later reuse.

This keeps prompts short while surfacing relevant context without full histories.
