# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`cftoken` is a CLI tool that provisions scoped Cloudflare API tokens. It uses the official `cloudflare-go` SDK to fetch permission groups and create tokens with granular zone-level access controls. We write an internal helper client for the Cloudflare API located at https://developers.cloudflare.com/api/

## Prerequisites

- Go 1.25 or newer
- `CLOUDFLARE_API_TOKEN` environment variable must be set with a token that can manage API tokens

## Build and Development Commands

```bash
# Compile and verify packages
go build ./...

# Run the CLI with help
go run ./cmd/cftoken -h

# Run tests
go test ./...

# Isolated build cache (for sandboxed environments)
GOCACHE=$(pwd)/.cache go build ./...

# Create a token (example)
go run ./cmd/cftoken -token-prefix my-app -zone example.com

# List available permission groups
go run ./cmd/cftoken -list-permissions

# List configured zones
go run ./cmd/cftoken -list-zones

# Dry-run preview
go run ./cmd/cftoken -token-prefix test -zone example.com -dry-run

# Inspect a token
go run ./cmd/cftoken -inspect
go run ./cmd/cftoken -inspect-token <token-value>
```

## Architecture

### Module Organization

The codebase follows a standard Go project structure:

- **`cmd/cftoken/main.go`**: CLI entry point handling flag parsing and user interaction. This layer stays thin and delegates to internal packages.
- **`internal/cloudflare/`**: API client wrapper for Cloudflare interactions. Uses the official `cloudflare-go/v6` SDK with custom helpers for token provisioning, permission group matching, and token inspection.
- **`internal/config/`**: Configuration discovery and parsing. Handles reading from `$XDG_CONFIG_HOME/cftoken/config.json` (or `~/.config/cftoken/config.json`).
  - **`config.go`**: Loads default permissions, allowed CIDRs, and environment configurations from config file.
  - **`zones.go`**: Zone name-to-ID mappings and zone listing functionality.
- **`internal/template/`**: Template rendering engine using Go's `text/template` package. Renders permission templates to JSON arrays for dynamic permission configuration across environments.

### Key Design Patterns

1. **Simplicity**: This project prefers short and simple code - KISS. 

1. **Configuration Priority**: Command-line flags always override config file values. If neither is provided, hard-coded defaults apply.

2. **Permission Resolution**: The `internal/cloudflare/client.go:matchPermissionGroups()` function (line 355) matches user input (permission names, keys, or IDs) against the Cloudflare API's permission groups using case-insensitive normalized matching.

3. **Zone Resolution**: Zone names can be provided via `-zone` flag and are resolved to zone UUIDs using the `internal/config/zones.go:ResolveZoneID()` function (line 86). The CLI also accepts direct zone IDs via `-zone-id`.

4. **IP Restrictions**: The special CIDR `0.0.0.0/32` disables IP restrictions entirely. This is validated in `cmd/cftoken/main.go:normalizeCIDRList()` (line 274).

5. **Token Creation Flow**:
   - Parse flags and validate inputs (main.go:26-243)
   - If `-env` flag is provided, load environment and render template (main.go:135-177)
   - Template values are used as defaults; CLI flags can override them
   - Fetch permission groups from Cloudflare API
   - Match user-provided permission inputs to group IDs
   - Build token policy with zone resource scope
   - Create token via SDK and return result

6. **Zone-Based Configuration**:
   - Zones can be simple string mappings (`"name": "zone-id"`) or extended objects with full configuration
   - Extended zones include: zone IDs, CIDRs, TTL, permissions, and optional permission templates
   - Permission templates render to JSON arrays using Go's `text/template`
   - CLI flags override zone config (precedence: flags > zone config > config defaults)
   - Templates are optional; zones can use static permission lists instead

1. **Clarify Implentation Details**:
   - If implentation details are not clearly defined clarify the questions
   - Propose a implementation plan, the plan needs to be approved before implementation.

## Configuration File

The CLI reads `~/.config/cftoken/config.json`:

```json
{
  "default_permissions": ["Zone:Read", "Zone:Cache Purge"],
  "default_allowed_cidrs": ["10.0.0.1/32"],
  "zones": {
    "example.com": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "dev": {
      "zone_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "allowed_cidrs": ["10.0.1.0/24"],
      "ttl": "4h",
      "template_file": "~/.config/cftoken/templates/permissions.json.tmpl",
      "variables": {
        "CachePermission": "Cache Purge",
        "IncludeEdit": false
      }
    },
    "prod": {
      "zone_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "allowed_cidrs": ["203.0.113.1/32"],
      "permissions": ["Zone:Read", "Zone:Edit", "Zone:Cache Purge"]
    }
  }
}
```

- **`default_permissions`**: Seeds `-permissions` flag when omitted
- **`default_allowed_cidrs`**: Seeds `-allow-cidrs` flag when omitted
- **`zones`**: Maps zone names to either:
  - Simple string: zone UUID (e.g., `"example.com": "zone-id"`)
  - Extended object: full configuration with zone ID, CIDRs, TTL, permissions, and optional templates

## Important Implementation Details

### CIDR Validation
IP restriction logic requires either:
- `-allow-cidrs` flag provided
- `default_allowed_cidrs` in config.json
- Or the sentinel value `0.0.0.0/32` to disable restrictions

The CLI validates all CIDRs and prints the final allowed CIDR list for audit purposes.

### Permission Matching
Permission groups can be specified by:
- Exact ID match (case-insensitive)
- Name match (normalized: lowercased, spaces/underscores/hyphens/colons/dots removed)
- Meta key match (same normalization as name)

See `internal/cloudflare/client.go:normalizeKey()` (line 391) for normalization logic.

### Token Naming
Token names are automatically suffixed with UTC timestamp: `<prefix>-YYYYMMDDTHHMMSSZ`

### TTL Handling
- Default: 8 hours
- Use `-ttl 0` for tokens with no expiration
- API returns RFC3339 formatted expiration timestamps

### Template Rendering
Permission templates are rendered using Go's `text/template` package:

**Template Structure**: Templates must render to a valid JSON array of permission identifiers:
```json
[
  "Zone:Read",
  "Zone:{{ .CachePermission }}",
  "Zone:Edit"
]
```

**Template Flow**:
1. Load zone configuration from config (if `-zone` flag matches extended zone config)
2. If zone has `template_file` or `template_inline`, render with zone variables
3. Otherwise, use static `permissions` from zone config
4. Rendered permissions are used unless overridden by `-permissions` flag

**Variables**: Only variables defined in the zone's `variables` section are available in the template. No built-in variables are automatically provided.

**Precedence Order** (highest to lowest):
1. Explicit CLI flags (`-permissions`, `-allow-cidrs`, `-zone-id`, etc.)
2. Zone configuration values (zone_id, allowed_cidrs, rendered template permissions)
3. Global config defaults (`default_permissions`, `default_allowed_cidrs` if `inherit_defaults: true`)
4. Hard-coded CLI defaults

## Coding Conventions

- Use `gofmt` and `goimports` before committing
- Keep CLI flag parsing in `cmd/cftoken/main.go`
- Put reusable API logic in `internal/cloudflare/`
- Put config logic in `internal/config/`
- Place tests alongside the files they cover (e.g., `client_test.go`)
- Use table-driven tests with Go's built-in `testing` package

## Common Flags

- `-token-prefix` (optional): Token name prefix; defaults to zone name if not provided
- `-zone`: Zone name (simple string mapping or extended config) or `-zone-id` for direct zone UUID
- `-permissions`: Comma-separated permission groups (default: `Zone:Read`)
- `-allow-cidrs`: Comma-separated CIDR ranges (overrides config; use `0.0.0.0/32` to disable)
- `-ttl`: Token lifetime (default: `8h`; use `0` for no expiry)
- `-dry-run`: Preview token configuration without creating it
- `-inspect`: Inspect token details after creation or inspect management token
- `-inspect-token`: Inspect an arbitrary token value
- `-list-permissions`: Show available permission groups
- `-list-zones`: Show configured zones (both simple and extended)
- `-timeout`: API timeout (default: `30s`)
- `-v`: Verbose logging
