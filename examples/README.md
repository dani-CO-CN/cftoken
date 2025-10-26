# Example Configuration

This directory contains example configurations for using cftoken with zones and policy templates.

## Files

- **`config.json`**: Example configuration with multiple zones (simple.example.com, dev, staging, prod, readonly)
- **`policy.json.tmpl`**: Example policy template that supports account-level and zone-level permissions with variable substitution

## How It Works

### Basic Concept

Zones in the config can be either:
1. **Simple string mapping**: `"simple.example.com": "dddd..."` - just maps a name to a zone ID
2. **Extended configuration**: Full object with zone ID, permissions, CIDRs, TTL, and optional policy templates

The template system allows you to:
- Define Cloudflare API token policies using standard Go template syntax
- Share policy templates across multiple zones with different variables
- Support account-level and zone-level permissions
- Support multiple zones in a single token
- Inject variables via CLI flags (`-var key=value`) or config file
- Automatically inject `ZoneID` from the zone's `zone_id` field (no need to duplicate it in variables)
- Each zone has its own zone ID, CIDRs, and TTL settings

### Example Use Case

You have three zones representing different environments:
- **dev**: Zone ID `bbbb...`, read + DNS permissions, 4h TTL
- **staging**: Zone ID `cccc...`, read + DNS + cache purge permissions, 8h TTL
- **prod**: Zone ID `aaaa...`, read + DNS + cache purge + edit permissions, 8h TTL, stricter IPs

All three use the same policy template (`policy.json.tmpl`) but with different variables:

```json
// policy.json.tmpl
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
      {{- if .IncludeCachePurge }},
      {
        "id": "e17beae8b8cb423a99b1730f21238bed",
        "name": "Cache Purge"
      }
      {{- end }}
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

### Configuration Setup

1. Copy `config.json` to `~/.config/cftoken/config.json`
2. Copy `policy.json.tmpl` to `~/.config/cftoken/templates/policy.json.tmpl`
3. Update zone IDs, CIDRs, and variables to match your setup

### Usage

```bash
# Create token using dev zone configuration (token named "dev-20060102T150405Z")
go run ./cmd/cftoken -zone dev

# Create token using prod zone configuration
go run ./cmd/cftoken -zone prod

# Create token using simple zone (string mapping)
go run ./cmd/cftoken -zone simple.example.com

# Use custom token prefix instead of zone name
go run ./cmd/cftoken -token-prefix myapp -zone prod

# Override template variables with -var flags
go run ./cmd/cftoken -zone dev -var ZoneID=my-zone-id -var IncludeCachePurge=true

# Inject multiple variables for multi-zone token
go run ./cmd/cftoken -zone prod -var ZoneID=zone-abc -var AccountID=acc-123

# List available zones
go run ./cmd/cftoken -list-zones

# Override zone settings with flags (for non-template zones)
go run ./cmd/cftoken -zone readonly -permissions "Zone:Read"
```

### Zones Without Templates

You can also define zones with static permissions (no template):

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

This is useful for simple, one-off configurations that don't need templating.
