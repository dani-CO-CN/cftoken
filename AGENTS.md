# Repository Guidelines

## Project Structure & Module Organization
The CLI source lives in `cmd/cftoken/main.go`, while reusable API helpers are under `internal/cloudflare`. Config discovery and parsing live in `internal/config`, including zone defaults in `internal/config/zones.go` that can be overridden via `~/.config/cftoken/zones.json`. Add shared logic inside these internal packages to keep the command layer thin. Place Go tests alongside the files they cover (for example, `internal/cloudflare/client_test.go`). Generated build artifacts or caches belong in ephemeral directories such as `.cache/`.

## Upstream API
We write an internal helper client for the Cloudflare API located at https://developers.cloudflare.com/api/

## Build, Test, and Development Commands
- `go build ./...` — compiles the CLI and verifies packages compile cleanly.
- `go run ./cmd/cftoken -h` — prints the command usage and validates flag wiring.
- `GOCACHE=$(pwd)/.cache go build ./...` — isolates the build cache inside the repo when sandboxed environments block `$HOME/.cache`.
- `go test ./...` — executes unit tests once they are added.

## Coding Style & Naming Conventions
Stick to idiomatic Go: exported identifiers use CamelCase, unexported ones use lowerCamelCase. Let `gofmt` (tabs for indents, trimmed semicolons) and `goimports` shape files before committing. Keep package-level vars private unless they form the public surface. Favor small, focused functions inside `internal/cloudflare` and keep CLI flag parsing in `cmd/cftoken/main.go`.

## Testing Guidelines
Use Go’s built-in `testing` package with table-driven tests. Place mocks or fixtures next to the tests that consume them. Name tests `TestFunctionBehavior` and subtests with descriptive strings. Run `go test ./...` locally and ensure new functionality includes coverage for success and failure paths, especially around HTTP interactions.

## Commit & Pull Request Guidelines
Write commits in the imperative mood (e.g., `Add zone lookup helper`) and keep the subject under 72 characters. Group related refactors or feature work together; separate formatting-only changes. Pull requests should include: a concise summary of the change, reproduction or verification steps (`go build`, `go test`), and any follow-up work. Link to tracking issues when available and add screenshots or sample CLI output if behavior changes.

## Security & Configuration Tips
Never commit real Cloudflare tokens. Reference them via environment variables such as `CLOUDFLARE_API_TOKEN` and document placeholders in examples. The CLI reads credentials exclusively from `CLOUDFLARE_API_TOKEN` to keep secrets out of shell history. Prefer scoping new tokens to the minimal set of permission groups and zones; the CLI now enforces this by resolving zone IDs from the default or local `zones.json` map and defaults to an 8h TTL unless `-ttl 0` is provided. Rotate credentials after testing and scrub logs before sharing.

## Documentation
For end users we want to keep the docs in the README.md located at the root folder level, the documentation is written in markdown - the documentation contains the goal of the project and the most important functions and examples in that file and also the default help for the binary.
