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
# Token prefix defaults to zone name
cftoken -zone dev

# Or specify a custom token prefix
cftoken -token-prefix my-app -zone example.com
```

Flags of note:
- `-token-prefix string` - optional; token name prefix. Defaults to zone name if not provided. The CLI appends a UTC timestamp to produce the final token name.
- `-zone-id string` or `-zone string` - supply a zone UUID directly, a friendly zone name (simple string mapping), or a configured zone with extended settings (permissions, CIDRs, TTL, templates).
- `-var key=value` - template variable in key=value format. Can be specified multiple times. Overrides variables from config file.
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
cftoken -h
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
- `zones` powers `-zone` lookups and the `-list-zones` command; run `cftoken -list-zones` to verify entries.

Command-line flags always take precedence over `config.json` values, so pass `-permissions` or `-allow-cidrs` to override the defaults on demand.

Use `0.0.0.0/32` in either the flag or config to disable IP restrictions entirely for the issued token.

The CLI always prints the final allowed CIDR list for the newly created token so you can audit the restriction that Cloudflare enforces.

For debugging, add `-inspect` alongside normal token creation to automatically print the new token's policies, or run `cftoken -inspect` on its own (optionally with `-inspect-token <value>`) to review existing tokens.

## Zones with Extended Configuration

The CLI supports zones with optional extended configuration including template-based policies. Zones can be defined in two ways:

1. **Simple string mapping**: `"example.com": "zone-id-here"` - basic zone name to ID mapping
2. **Extended configuration**: Full object with zone ID, permissions, CIDRs, TTL, and optional policy templates

This allows you to:
- Define zone-specific settings (zone IDs, CIDRs, TTL, permissions)
- Use policy templates to create tokens with account-level and zone-level permissions
- Support multiple zones in a single token
- Inject variables via CLI flags (`-var`) or config file
- Override any setting via CLI flags

### Setting Up Zones

Zones can have different levels of configuration in your config.json:

```json
{
  "zones": {
    "simple.example.com": "dddddddddddddddddddddddddddddddd",
    "dev": {
      "zone_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "allowed_cidrs": ["10.0.1.0/24"],
      "ttl": "4h",
      "template_file": "~/.config/cftoken/templates/policy.json.tmpl",
      "variables": {
        "IncludeCachePurge": false,
        "IncludeEdit": false
      }
    },
    "prod": {
      "zone_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "allowed_cidrs": ["203.0.113.1/32", "203.0.113.2/32"],
      "ttl": "8h",
      "template_file": "~/.config/cftoken/templates/policy.json.tmpl",
      "variables": {
        "IncludeCachePurge": true,
        "IncludeEdit": true
      }
    }
  }
}
```

Note: The `zone_id` is automatically available as `{{ .ZoneID }}` in templates - no need to include it in `variables`.

### Creating Policy Templates

Templates use Go's `text/template` syntax and render to a JSON array of Cloudflare API token policies. Create a template file (e.g., `~/.config/cftoken/templates/policy.json.tmpl`):

```json
[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID }}": "*"
    },
    "permission_groups": [
      {
        "id": "c8fed203ed3043cba015a93ad1616f1f",
        "name": "Zone Read"
      },
      {
        "id": "82e64a83756745bbbb1c9c2701bf816b",
        "name": "DNS Read"
      }
      {{- if .IncludeEdit }},
      {
        "id": "e086da7e2179491d91ee5f35b3ca210a",
        "name": "Zone Settings Write"
      }
      {{- end }}
    ]
  }
]
```

**Account-Level Permissions Example**:

```json
[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.{{ .AccountID }}": "*"
    },
    "permission_groups": [
      {
        "id": "account-permission-id"
      }
    ]
  }
]
```

**Multiple Zones Example**:

```json
[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID1 }}": "*",
      "com.cloudflare.api.account.zone.{{ .ZoneID2 }}": "*"
    },
    "permission_groups": [
      {
        "id": "zone-read-permission-id"
      }
    ]
  }
]
```

Templates allow you to create flexible, reusable token policies with variable substitution for zone IDs, account IDs, and permission groups.

### Using Configured Zones

Create tokens using the `-zone` flag (token prefix defaults to zone name):

```bash
# Use simple zone (string mapping) - token named "simple.example.com-20060102T150405Z"
cftoken -zone simple.example.com

# Use dev zone with extended config - token named "dev-20060102T150405Z"
cftoken -zone dev

# Use prod zone with extended config
cftoken -zone prod

# Specify custom token prefix
cftoken -token-prefix myapp -zone prod

# Override template variables with -var flags (CLI variables override config)
cftoken -zone dev -var ZoneID=abc123 -var IncludeEdit=true

# Inject multiple variables
cftoken -zone prod -var AccountID=my-account -var ZoneID1=zone-a -var ZoneID2=zone-b

# Override zone settings with flags
cftoken -zone prod -permissions "Zone:Read,Zone:Edit"

# List all configured zones
cftoken -list-zones
```

### Template Features

Templates render to a JSON array of Cloudflare API token policy objects using standard Go template syntax. Variables are merged with the following precedence (highest to lowest):
1. **CLI flags** (`-var key=value`) - highest priority, overrides everything
2. **Zone variables** (from `variables` section in config.json)
3. **Auto-injected `ZoneID`** - automatically set from the zone's `zone_id` field

This means you don't need to manually duplicate the zone ID in your variables - it's automatically available as `{{ .ZoneID }}` in templates.

**Inline Templates**:

For simple cases, use `template_inline` instead of `template_file`:

```json
{
  "zones": {
    "quick": {
      "zone_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "allowed_cidrs": ["10.0.0.1/32"],
      "template_inline": "[\"Zone:Read\", \"Zone:{{ .Permission }}\"]",
      "variables": {
        "Permission": "Edit"
      }
    }
  }
}
```

**Zone Configuration Options**:
- `zone_id` - Zone identifier (required). Automatically injected as `ZoneID` variable in templates.
- `allowed_cidrs` - List of allowed CIDR ranges (optional, uses config defaults if not specified)
- `ttl` - Token TTL as duration string (e.g., "8h", "24h")
- `permissions` - Static list of permissions (used if no template specified)
- `template_file` - Path to policy template file (supports `~` for home directory)
- `template_inline` - Inline policy template string (alternative to template_file)
- `variables` - Key-value pairs passed to the template (can override auto-injected `ZoneID`)
- `inherit_defaults` - If true, inherit `default_permissions` and `default_allowed_cidrs` from config (when not specified in zone)

### Zones Without Templates

You can also define zones with static configuration (no templates):

```json
{
  "zones": {
    "readonly": {
      "zone_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "permissions": ["Zone:Read"],
      "allowed_cidrs": ["10.0.0.1/32"],
      "ttl": "1h"
    }
  }
}
```

See the `examples/` directory for complete examples.
## Development
- Build: `go build ./...`
- Tests: `go test ./...`
- Isolated cache (for sandboxed environments): `GOCACHE=$(pwd)/.cache go build ./...`

The command is wired to stay thin; reusable logic sits under `internal/cloudflare` and `internal/config`. Keep new shared helpers in those packages, let the CLI layer focus on flag parsing and user interaction. Always run `gofmt`/`goimports` before committing. Avoid checking secrets into the repo.
