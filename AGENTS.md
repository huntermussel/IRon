# Repository Guidelines

## Project Structure & Module Organization

- `cmd/iron/`: CLI entrypoint (`main.go`).
- `internal/chat/`: chat service, message types, and adapter interface.
- `internal/llm/`: LLM providers/adapters (currently Ollama via LangChainGo).
- `internal/middleware/`: middleware interfaces + chain runner (see `internal/middleware/README.md`).
- `middlewares/`: built-in middleware implementations (example: `middlewares/trashcleanner/`).
- `bin/`: local build output (kept out of git via `bin/.gitignore`).

## Build, Test, and Development Commands

This is a Go module (`go.mod` targets Go 1.22.x). Common workflows:

```bash
go run ./cmd/iron                # run the CLI locally
go build -o bin/iron ./cmd/iron  # build a local binary into ./bin
go test ./...                    # run all unit tests
go test -race ./...              # run tests with the race detector
go vet ./...                     # run basic static analysis
gofmt -w .                       # format all Go files
go mod tidy                      # keep go.mod/go.sum clean after dependency changes
```

## Coding Style & Naming Conventions

- Use standard Go formatting (`gofmt`); do not hand-format.
- Package names are short and lowercase; exported identifiers use `CamelCase`.
- Prefer small, focused packages and keep shared utilities in `internal/`.
- Thread `context.Context` through call chains; keep timeouts/cancellation explicit (see `cmd/iron/main.go`).
- Middleware is WordPress-hook-like: register a chain, dispatch `Event`s, and optionally implement `ConditionalMiddleware.ShouldLoad` for per-request enablement (see `internal/middleware/README.md`).

## Testing Guidelines

- Use the standard library `testing` package.
- Keep tests colocated with code and named `*_test.go` (example: `internal/middleware/chain_test.go`).
- Run `go test ./...` before opening a PR; add tests for behavior changes where practical.

## Configuration Tips

Runtime is configured via environment variables:

- `IRON_MODEL` (default: `llama3.2`)
- `IRON_PROVIDER` (default: `ollama`)
- `IRON_OLLAMA_URL` (default: `http://localhost:11434`)

If the CLI can’t reach Ollama, verify the server is running and the URL is correct.

## Commit & Pull Request Guidelines

- Follow Conventional Commits (example in history: `feat: ...`).
- PRs should include: a short problem/solution description, how to test (`go test ./...`, `go run ./cmd/iron`), and any new config/env var notes.
