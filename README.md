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
- `-allow-cidrs string` - comma-separated list of allowed requester CIDR ranges. Defaults to `10.0.0.1/32,10.0.0.2/32`; override with your own network.
- `-inspect` - print a summary of token details. When combined with token creation it inspects the newly minted token; otherwise it inspects the management token.
- `-inspect-token string` - print a summary for an arbitrary token value (for example, one you just created) and exit.
- `-ttl duration` - token lifetime; defaults to `8h`. Use `-ttl 0` for no expiry.
- `-list-permissions` - print available permission groups and exit.
- `-list-zones` - print all configured zones in a table and exit.
- `-timeout duration` - API timeout (default `30s`).
- `-v` - emit verbose request logs.

You can open the compiled binary usage any time:
```bash
go run ./cmd/cftoken -h
```

## Zone Configuration
Zones are loaded exclusively from `~/.config/cftoken/zones.json` (or `$XDG_CONFIG_HOME/cftoken/zones.json`). Example:
```json
{
  "example.com": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "internal.example": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
}
```
Run `go run ./cmd/cftoken -list-zones` to verify merged entries and their sources.

## Permissions Defaults
Optional defaults belong in `$XDG_CONFIG_HOME/cftoken/config.json`:
```json
{
  "default_permissions": [
    "Zone:Read",
    "Zone:Cache Purge"
  ]
}
```
If present, these replace the CLI fallback when `-permissions` is omitted.

The CLI always prints the final allowed CIDR list for the newly created token so you can audit the restriction that Cloudflare enforces.

For debugging, add `-inspect` alongside normal token creation to automatically print the new token's policies, or run `go run ./cmd/cftoken -inspect` on its own (optionally with `-inspect-token <value>`) to review existing tokens.

## Development
- Build: `go build ./...`
- Tests: `go test ./...`
- Isolated cache (for sandboxed environments): `GOCACHE=$(pwd)/.cache go build ./...`

The command is wired to stay thin; reusable logic sits under `internal/cloudflare` and `internal/config`. Keep new shared helpers in those packages, let the CLI layer focus on flag parsing and user interaction. Always run `gofmt`/`goimports` before committing. Avoid checking secrets into the repo.
