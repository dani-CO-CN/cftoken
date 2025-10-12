# cftoken CLI

## Overview
`cftoken` provisions scoped Cloudflare API tokens. The CLI now relies on the official [`cloudflare-go`](https://github.com/cloudflare/cloudflare-go) SDK to fetch permission groups and create tokens.

## Prerequisites
- Go 1.25 or newer.
- Export `CLOUDFLARE_API_TOKEN` with a token that can manage API tokens.

```bash
export CLOUDFLARE_API_TOKEN=your-admin-token
```

## Usage
```bash
go run ./cmd/cftoken -token-prefix my-app -zone example.com
```

Flags of note:
- `-token-prefix string` - required; the CLI appends a UTC timestamp to produce the final token name.
- `-zone-id string` or `-zone string` - supply a zone UUID directly or a friendly zone name defined in defaults or overrides.
- `-permissions string` - comma-separated permission groups; defaults to `Zone:Read` unless config overrides exist.
- `-allow-cidrs string` - comma-separated list of allowed requester CIDR ranges. Required unless `default_allowed_cidrs` is present in config; use `0.0.0.0/32` to disable IP restrictions. The flag always wins.
- `-inspect` - print a summary of token details. When combined with token creation it inspects the newly minted token; otherwise it inspects the management token.
- `-inspect-token string` - print a summary for an arbitrary token value (for example, one you just created) and exit.
- `-dry-run` - preview the resolved token configuration without creating it.
- `-ttl duration` - token lifetime; defaults to `8h`. Use `-ttl 0` for no expiry.
- `-list-permissions` - print available permission groups and exit.
- `-list-zones` - print all configured zones in a table and exit.
- `-timeout duration` - API timeout (default `30s`).
- `-v` - emit verbose request logs.

You can open the compiled binary usage any time:
```bash
go run ./cmd/cftoken -h
```

## Configuration
The CLI reads a single JSON file at `$XDG_CONFIG_HOME/cftoken/config.json` (falls back to `~/.config/cftoken/config.json`). You can provide default permissions, allowed CIDRs, and zone mappings:
```json
{
  "default_permissions": [
    "Zone:Read",
    "Zone:Cache Purge"
  ],
  "default_allowed_cidrs": [
    "10.0.0.1/32",
    "10.0.0.2/32"
  ],
  "zones": {
    "example.com": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "internal.example": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
}
```
These defaults are optional, but when present they replace the CLI fallbacks:
- `default_permissions` seeds the `-permissions` flag when omitted.
- `default_allowed_cidrs` seeds the `-allow-cidrs` flag when omitted.
- `zones` powers `-zone` lookups and the `-list-zones` command; run `go run ./cmd/cftoken -list-zones` to verify entries.

Command-line flags always take precedence over `config.json` values, so pass `-permissions` or `-allow-cidrs` to override the defaults on demand.

Use `0.0.0.0/32` in either the flag or config to disable IP restrictions entirely for the issued token.

The CLI always prints the final allowed CIDR list for the newly created token so you can audit the restriction that Cloudflare enforces.

For debugging, add `-inspect` alongside normal token creation to automatically print the new token's policies, or run `go run ./cmd/cftoken -inspect` on its own (optionally with `-inspect-token <value>`) to review existing tokens.

## Development
- Build: `go build ./...`
- Tests: `go test ./...`
- Isolated cache (for sandboxed environments): `GOCACHE=$(pwd)/.cache go build ./...`

The command is wired to stay thin; reusable logic sits under `internal/cloudflare` and `internal/config`. Keep new shared helpers in those packages, let the CLI layer focus on flag parsing and user interaction. Always run `gofmt`/`goimports` before committing. Avoid checking secrets into the repo.
