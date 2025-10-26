package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// settings mirrors the JSON structure stored in the config file.
type settings struct {
	DefaultPermissions  []string               `json:"default_permissions"`
	DefaultAllowedCIDRs []string               `json:"default_allowed_cidrs"`
	Zones               map[string]interface{} `json:"zones"`
}

// ZoneConfig defines extended configuration for a zone with optional template for permissions.
type ZoneConfig struct {
	ZoneID          string                 `json:"zone_id"`
	Permissions     []string               `json:"permissions"`
	AllowedCIDRs    []string               `json:"allowed_cidrs"`
	TTL             string                 `json:"ttl"`
	TemplateFile    string                 `json:"template_file"`
	TemplateInline  string                 `json:"template_inline"`
	Variables       map[string]interface{} `json:"variables"`
	InheritDefaults bool                   `json:"inherit_defaults"`
}

// DefaultPath resolves the config file path according to XDG conventions.
func DefaultPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func configDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "cftoken"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cftoken"), nil
}

// LoadDefaultPermissions reads the configuration file (if present) and returns
// the default permission keys defined within.
func LoadDefaultPermissions() ([]string, error) {
	cfg, err := loadSettings()
	if err != nil {
		return nil, err
	}

	perms := sanitizeStringList(cfg.DefaultPermissions)
	if len(perms) == 0 {
		return nil, fs.ErrNotExist
	}
	return perms, nil
}

// LoadDefaultAllowedCIDRs reads the configuration file (if present) and returns
// the default allowed CIDR ranges defined within.
func LoadDefaultAllowedCIDRs() ([]string, error) {
	cfg, err := loadSettings()
	if err != nil {
		return nil, err
	}

	cidrs := sanitizeStringList(cfg.DefaultAllowedCIDRs)
	if len(cidrs) == 0 {
		return nil, fs.ErrNotExist
	}
	return cidrs, nil
}

func loadSettings() (*settings, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg settings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return &cfg, nil
}

func sanitizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, entry := range values {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// LoadZoneConfig loads zone configuration by name. Returns the zone ID and optional extended config.
func LoadZoneConfig(zoneName string) (string, *ZoneConfig, error) {
	cfg, err := loadSettings()
	if err != nil {
		return "", nil, err
	}

	if cfg.Zones == nil {
		return "", nil, fmt.Errorf("no zones configured")
	}

	zoneValue, ok := cfg.Zones[zoneName]
	if !ok {
		return "", nil, fmt.Errorf("zone %q not found", zoneName)
	}

	// Handle simple string zone ID
	if zoneID, ok := zoneValue.(string); ok {
		return zoneID, nil, nil
	}

	// Handle complex zone configuration
	zoneMap, ok := zoneValue.(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("zone %q has invalid configuration format", zoneName)
	}

	// Marshal back to JSON and unmarshal into ZoneConfig
	data, err := json.Marshal(zoneMap)
	if err != nil {
		return "", nil, fmt.Errorf("parse zone config: %w", err)
	}

	var zoneConfig ZoneConfig
	if err := json.Unmarshal(data, &zoneConfig); err != nil {
		return "", nil, fmt.Errorf("parse zone config: %w", err)
	}

	// Apply defaults if requested
	if zoneConfig.InheritDefaults {
		if len(zoneConfig.Permissions) == 0 && len(cfg.DefaultPermissions) > 0 {
			zoneConfig.Permissions = append([]string(nil), cfg.DefaultPermissions...)
		}
		if len(zoneConfig.AllowedCIDRs) == 0 && len(cfg.DefaultAllowedCIDRs) > 0 {
			zoneConfig.AllowedCIDRs = append([]string(nil), cfg.DefaultAllowedCIDRs...)
		}
	}

	return zoneConfig.ZoneID, &zoneConfig, nil
}
