# CLAUDE.md — IRon AI Assistant

> AI assistant guidance for developing on the IRon codebase. Keep this file updated as the project evolves.

---

## What Is IRon?

**IRon** is a Go-based personal AI assistant (CLI + Web UI) that wraps multiple LLM providers behind a modular middleware plugin system. Think of it as a self-hosted, extensible alternative to OpenWebUI — but with deterministic NLU intent handling, token-efficient IR-based memory, and multi-platform messaging (Telegram, Slack, WhatsApp, Signal).

**Core value proposition:**
- Run AI assistants locally (Ollama) or via cloud APIs (OpenAI, Anthropic, Gemini)
- Intercept/transform prompts and responses via a composable middleware chain
- Bypass the LLM entirely for known intents (alarms, weather, device control) — fast and cheap
- Ship a single binary that embeds the React web UI

---

## Repository Layout

```
IRon/
├── cmd/iron/          # CLI entrypoint (Cobra commands)
├── internal/
│   ├── chat/          # Chat service: history, middleware dispatch, LLM calls
│   ├── llm/           # LLM provider adapters (Ollama, OpenAI, Anthropic, Gemini)
│   ├── middleware/     # Middleware interfaces, chain runner, registry, debug logger
│   ├── memory/         # Lexical KV store for token-efficient IR retrieval
│   ├── nlu/           # Template-based NLU engine (intent matching + slot extraction)
│   ├── skills/         # Tool/skill manager for LLM function calling
│   ├── gateway/        # Orchestrator: loads config, initializes services, routes messages
│   ├── browser/        # Headless Chrome controller (chromedp)
│   ├── communicators/  # External platform adapters (Telegram, Slack, WhatsApp, Signal)
│   ├── onboarding/     # Interactive TUI setup wizard (charmbracelet/bubbletea)
│   └── webui/          # HTTP server + embedded React frontend (go:embed)
├── middlewares/        # Auto-loaded middleware plugins
│   ├── autoload/       # Blank-import all plugins to trigger init() registration
│   ├── alarm/          # "Set alarm for {time}" intent handler
│   ├── cron/           # "Remind me to {action} every {duration}" handler
│   ├── weather/        # "Weather in {location}" handler
│   ├── device/         # "Turn {on|off} {device}" handler
│   ├── greeting/       # Short-circuit simple salutations (saves tokens)
│   ├── trashcleanner/  # Stopword removal (keeps technical tokens)
│   ├── intentcompressor/ # Emit short intent labels
│   ├── tokenbudget/    # Cap MaxTokens when provided
│   ├── codingtools/    # File ops + Python execution tools
│   ├── calendar/       # Calendar integration
│   ├── email/          # Email integration
│   ├── notes/          # Notes management
│   ├── pytools/        # Python execution bridge
│   ├── localcache/     # Local caching layer
│   ├── slack/          # Slack communicator middleware
│   └── emmetbridge/    # Emmet/IDE bridge
├── ui/                 # React 19 + TypeScript + Vite + TailwindCSS frontend
│   └── src/            # React components; built output → internal/webui/static
├── docs/
│   └── WEBUI_PLAN.md   # 3-phase roadmap for advanced UI & autonomous features
├── scripts/            # Helper shell scripts
├── bin/                # Local build output (gitignored); middleware.debug.jsonl written here
├── Makefile            # Build, test, lint, cross-compile targets
├── go.mod              # Go 1.24.4 module
└── .github/workflows/  # CI (go.yml) + Release (release.yml) pipelines
```

---

## Technology Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24.4 |
| CLI framework | [Cobra](https://github.com/spf13/cobra) |
| LLM abstraction | [LangChainGo](https://github.com/tmc/langchaingo) |
| TUI | [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) + bubbles |
| Browser automation | [chromedp](https://github.com/chromedp/chromedp) |
| Telegram | [telebot.v3](https://gopkg.in/telebot.v3) |
| Frontend | React 19 + TypeScript 5.9 + Vite 7 + TailwindCSS 4 |
| Icons | Lucide React |
| Routing (frontend) | React Router DOM 7 |
| Env loading | godotenv |

**LLM providers:** Ollama (default/local), OpenAI, Anthropic, Google Gemini, Azure OpenAI

---

## Build & Run

### Prerequisites

- Go 1.22+ (module-aware)
- Node.js 18+ + npm (for frontend)
- Ollama running locally at `http://localhost:11434` (or configure another provider)

### Common Commands

```bash
# Run directly (no build step)
go run ./cmd/iron

# Build local binary
go build -o bin/iron ./cmd/iron

# Build everything including React UI
make build

# Cross-platform release builds (darwin/linux/windows, amd64/arm64)
make build-all

# Install to GOPATH/bin
make install

# Tests
go test ./...              # all tests
go test -race ./...        # with race detector (required before PR)
go test -v -cover ./...    # verbose with coverage

# Code quality
go vet ./...               # static analysis
gofmt -w .                 # format all Go files
go mod tidy                # clean up dependencies
```

### CLI Commands

```bash
./bin/iron              # default: interactive chat (same as 'chat')
./bin/iron chat         # interactive CLI session
./bin/iron exec "..."   # single-shot prompt, e.g.: exec "summarize this" -t 300
./bin/iron serve        # background daemon (starts all registered communicators)
./bin/iron web          # start web UI server (default: :8080)
./bin/iron web --port 3000
./bin/iron onboard      # interactive TUI setup wizard
./bin/iron doctor       # health check
./bin/iron version      # version info
```

### Frontend Development

```bash
cd ui
npm install
npm run dev     # dev server with hot reload (proxies /api/* to :8080)
npm run build   # production build → internal/webui/static (picked up by go:embed)
```

---

## Configuration

IRon resolves configuration in this priority order (highest wins):

1. Local `.env` file (project root)
2. `~/.iron/.env`
3. `~/.iron/config.json` (created by `onboard`)
4. Environment variables

### Key Environment Variables

```bash
IRON_MODEL="llama3.2"                          # LLM model name
IRON_PROVIDER="ollama"                         # ollama | openai | anthropic | gemini | azure
IRON_OLLAMA_URL="http://localhost:11434"        # Ollama base URL
IRON_OPENAI_API_KEY="sk-..."                   # or OPENAI_API_KEY
IRON_ANTHROPIC_API_KEY="..."                   # or ANTHROPIC_API_KEY
IRON_GEMINI_API_KEY="..."                      # Google Gemini
IRON_DISABLED_MIDDLEWARES="alarm,weather"      # Comma-separated IDs to skip
```

---

## Core Architecture

### Request Processing Pipeline

```
User Input
    │
    ▼
Gateway.HandleMessage()
    │
    ▼
middleware.Chain.Run(event: "before_llm_request")
    │  → TrashCleaner removes stopwords
    │  → IntentCompressor emits short labels
    │  → NLU Tools (alarm/weather/device/cron) may short-circuit here
    │  → TokenBudget caps MaxTokens
    │
    ▼  (if not short-circuited)
chat.Service.Send()
    │  → IR memory retrieval prepends relevant past context
    │  → LLM adapter called (streaming)
    │  → Tool calls extracted and executed via skills.Manager
    │
    ▼
middleware.Chain.Run(event: "after_llm_response")
    │  → Response post-processing plugins
    │
    ▼
middleware.Chain.Run(event: "before_user_reply")
    │  → Final formatting/filtering
    │
    ▼
Output (CLI / WebSocket / Communicator)
```

### Middleware System

Middlewares are the primary extension point. Each plugin implements:

```go
type Middleware interface {
    ID() string
    Priority() int          // lower = runs first
    OnEvent(ctx context.Context, event *Event) error
}

// Optional: skip loading based on request context
type ConditionalMiddleware interface {
    Middleware
    ShouldLoad(ctx context.Context, event *Event) bool
}
```

**Registering a new middleware:**

```go
// In your middleware package's init():
func init() {
    middleware.Register(&MyMiddleware{})
}
```

**Then add a blank import to `middlewares/autoload/autoload.go`:**

```go
import _ "github.com/yourusername/IRon/middlewares/mymiddleware"
```

**Event types:**
- `before_llm_request` — modify/inspect prompt before LLM call
- `after_llm_response` — modify/inspect LLM response
- `before_user_reply` — final transformation before sending to user

**Debug log:** `bin/middleware.debug.jsonl` — every event dispatch is written here as JSONL.

**Disabling at runtime:** `IRON_DISABLED_MIDDLEWARES="id1,id2"` (comma-separated middleware IDs)

### LLM Adapters

All providers implement `chat.Adapter`:

```go
type Adapter interface {
    ReplyStream(ctx context.Context, messages []Message) (<-chan string, error)
    // Tool calls are extracted from the stream automatically
}
```

Available adapters: `internal/llm/ollama`, `openai`, `anthropic`, `gemini`

### NLU Engine

Template-based intent matching that compiles utterance patterns to regexes:

```go
engine := nlu.GetEngine()  // singleton
engine.Register("set_alarm", []string{
    "set alarm for {time}",
    "wake me up at {time}",
})
result, err := engine.Match("set alarm for 7am")
// result.Intent == "set_alarm", result.Slots["time"] == "7am"
```

### IR Memory

Keeps prompts short via lexical retrieval:
- History pruned to last 8 messages
- User/assistant text indexed in `internal/memory` KV store
- Top lexical matches prepended to each new user prompt
- No vector embeddings required — pure TF-IDF/BM25-style scoring

### Skills (Tool Calling)

Register executable tools for LLM function calling:

```go
skills.Register(Skill{
    Name:        "read_file",
    Description: "Read the contents of a file",
    Parameters:  jsonSchemaForParams,
    Execute:     func(ctx context.Context, args map[string]any) (string, error) { ... },
})
```

---

## Adding a New Middleware

1. Create `middlewares/mymiddleware/mymiddleware.go`
2. Implement `middleware.Middleware` interface
3. Register in `init()`: `middleware.Register(&MyMiddleware{})`
4. Add blank import in `middlewares/autoload/autoload.go`
5. Add `middlewares/mymiddleware/README.md` (describe intents, event timing, toggle)
6. Write tests in `middlewares/mymiddleware/mymiddleware_test.go`

See `middlewares/alarm/` or `middlewares/greeting/` as reference implementations.

---

## Adding a New LLM Provider

1. Create `internal/llm/myprovider/adapter.go`
2. Implement `chat.Adapter` interface
3. Register in `internal/gateway/gateway.go` switch statement under `IRON_PROVIDER`

---

## Testing Guidelines

- Use standard `testing` package only (no third-party test frameworks)
- Tests colocated with code as `*_test.go` files
- Run `go test -race ./...` before every PR — race detection is required
- Test middleware behavior via `middleware.Chain` with mock events
- Mock LLM adapters for unit tests (don't require a running Ollama)
- Table-driven tests preferred for NLU pattern matching

---

## CI/CD

### GitHub Actions

**`go.yml`** (on push/PR to `main`/`dev`):
1. `make build-all` — cross-compile all platforms
2. `go test -v ./...` — full test suite
3. Upload artifacts: `dist/*` binaries (5-day retention)

**`release.yml`** (on tag `v*`):
1. Build all platform binaries
2. Create GitHub release with auto-generated notes
3. Upload binaries as release assets

### Release Binaries

| File | Target |
|---|---|
| `iron-darwin-arm64` | macOS Apple Silicon |
| `iron-darwin-amd64` | macOS Intel |
| `iron-linux-amd64` | Linux x86_64 |
| `iron-linux-arm64` | Linux ARM64 |
| `iron-windows-amd64.exe` | Windows |

---

## Coding Conventions

- **Formatting:** `gofmt -w .` — never hand-format
- **Naming:** short lowercase package names; exported identifiers in `CamelCase`
- **Context:** thread `context.Context` through all async call chains
- **Errors:** return errors up the stack; avoid `panic` in library code
- **Packages:** small, focused; shared utilities in `internal/`
- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`, `fix:`, `refactor:`, `docs:`, `test:`
- **PRs:** include problem/solution description + test instructions + env var notes

---

## Planned Features & Recommended Next Steps

### High-Value Quick Wins (fixes/polish)

- [ ] **Remove root-level test files** (`test_all_tools.go`, `test_gemini*.go`) — these belong in `_test.go` files or an `examples/` directory; they currently break `go build ./...` if run as programs
- [ ] **`doctor` command implementation** — currently a placeholder; should check Ollama connectivity, API key validity, communicator auth, and config file integrity
- [ ] **Structured config validation** — `~/.iron/config.json` has no schema; add validation on startup with clear error messages
- [ ] **Middleware priority documentation** — document the full priority order; currently implicit, making debugging hard
- [ ] **Context cancellation in streaming** — ensure all LLM stream goroutines respect `ctx.Done()` to avoid goroutine leaks on early termination
- [ ] **WebSocket vs SSE decision** — the web UI has partial SSE support; commit to one protocol and complete the implementation

### Medium-Priority Features

- [ ] **Multi-session web UI** (Phase 2 from `docs/WEBUI_PLAN.md`) — multiple concurrent chat sessions with persona selection; backend session map exists, UI needs wiring
- [ ] **Plugin management UI** — enable/disable middlewares from the web dashboard; expose `/api/middlewares` endpoint
- [ ] **Memory management UI** — let users view/edit/delete IR memory entries via the web UI
- [ ] **Persistent chat history** — currently in-memory only; add optional SQLite/file persistence
- [ ] **Rate limiting & retry** — add backoff/retry for LLM API calls with configurable limits
- [ ] **Signal communicator** — listed in codebase but implementation status unclear; complete or remove
- [ ] **Tool call approval workflow** (Phase 3) — human-in-the-loop before dangerous tool execution (file writes, shell commands)
- [ ] **Streaming tool results** — stream intermediate tool call output to the UI in real-time

### Longer-Term / Ambitious

- [ ] **Task planner middleware** — break complex prompts into sub-tasks with a progress UI (Phase 3)
- [ ] **Generative UI widgets** — render `<WeatherWidget>`, `<TimerCard>` etc. from structured LLM tool output
- [ ] **Workspace file manager** — split-pane chat + code editor view (OpenDevin-style)
- [ ] **Vector memory upgrade** — replace lexical IR with optional embedding-based retrieval (pgvector or in-process)
- [ ] **Plugin marketplace / registry** — allow third-party middleware packages to register via a config URL
- [ ] **Execution tracing panel** — collapsible "Agent Thoughts" showing intermediate reasoning and tool calls
- [ ] **Mobile-responsive UI** — the current React UI needs responsive breakpoints for mobile use

---

## Is This Project Useful?

**Yes — for the right use case.** Here's an honest assessment:

### Strengths
- **Self-hostable & private** — runs fully locally with Ollama; no data leaves your machine
- **Deterministic intent handling** — NLU short-circuits bypass LLM for common commands (fast, cheap, reliable)
- **Token efficiency** — IR memory trimming is a practical solution to context window costs
- **Multi-platform messaging** — Telegram/Slack/WhatsApp integration makes it usable as a daily-driver assistant
- **Single binary** — embeds the React UI; trivial to deploy on a home server or Raspberry Pi
- **Extensible architecture** — the middleware system is genuinely clean and easy to extend

### Weaknesses / Gaps
- **Thin documentation** — AGENTS.md covers style but not architecture; no API reference for middleware authors
- **Incomplete features** — `doctor`, Signal communicator, and several planned features are stubs
- **No persistence** — chat history is lost on restart; this limits practical daily use
- **Web UI is basic** — current React UI is functional but not polished; no mobile support
- **No auth** — the web server has no authentication; unsafe to expose publicly without a reverse proxy
- **Test coverage** — limited test files visible; more coverage needed especially for LLM adapters

### Bottom Line
IRon fills a real niche: a **self-hosted, hackable AI assistant** that you can extend with Go plugins and deploy anywhere. It's well-architected for its stage. The path to being genuinely useful as a daily tool requires: persistent history, a polished web UI, and completing the communicator integrations. The middleware plugin system is the killer feature — if documented and promoted well, it could attract plugin authors.

---

## Key Files Reference

| File | Purpose |
|---|---|
| `cmd/iron/main.go` | CLI entry, Cobra command registration |
| `internal/gateway/gateway.go` | Top-level orchestrator, config loading |
| `internal/chat/service.go` | Core chat loop, middleware dispatch |
| `internal/middleware/chain.go` | Middleware chain runner |
| `internal/middleware/registry.go` | Plugin registry |
| `internal/llm/*/adapter.go` | LLM provider implementations |
| `internal/nlu/engine.go` | NLU template compiler + matcher |
| `internal/memory/store.go` | IR lexical retriever |
| `internal/webui/server.go` | HTTP + WebSocket server |
| `middlewares/autoload/autoload.go` | Blank-imports all plugins |
| `Makefile` | All build/test commands |
| `docs/WEBUI_PLAN.md` | 3-phase UI/autonomous feature roadmap |
