# AGENTS.md

## Project overview

`helix-assist` is a dependency-free Go language server that provides AI-backed
completions and code actions for the Helix editor. It communicates with Helix
over LSP on stdin/stdout and supports OpenAI and Anthropic providers.

## Repository layout

- `cmd/helix-assist`: production CLI and LSP server entry point.
- `cmd/helix-assist-test`: provider-backed completion fixture runner.
- `internal/config`: flags, environment variables, defaults, and validation.
- `internal/lsp`: JSON-RPC/LSP transport, buffers, protocol types, and logging.
- `internal/handlers`: completion and code-action request handling.
- `internal/providers`: provider interfaces, prompts, and API clients.
- `internal/testing`: completion fixture runner support.
- `tests/completions`: language-specific live completion fixtures.

## Development guidelines

- Keep the project on the Go standard library unless a dependency is clearly
  justified; the zero-dependency design is intentional.
- Format changed Go files with `gofmt`.
- Follow existing package boundaries and keep provider-specific behavior in
  `internal/providers`.
- Preserve LSP protocol output: do not write diagnostics or logs to stdout.
  Use the existing LSP logger or stderr where appropriate.
- When adding configuration, update flags/environment loading, defaults,
  validation, tests, and the README configuration table as applicable.
- Never commit API keys, local log files, or generated files under `build/`.

## Validation

Run these checks after changing Go code:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
make build
```

Prefer focused package tests while iterating, then run the full suite before
finishing. Add or update `_test.go` coverage for behavior changes.

`make run-tests` is a live provider integration check, not the unit test suite.
It builds `cmd/helix-assist-test`, reads fixtures from `tests/completions`, and
requires credentials for the selected provider:

```sh
OPENAI_API_KEY=... make run-tests PROVIDER=openai
ANTHROPIC_API_KEY=... make run-tests PROVIDER=anthropic
```

Do not run provider-backed checks unless suitable credentials are already
available and the task warrants external API calls.
