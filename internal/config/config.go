package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// settings mirrors the JSON structure stored in the config file.
type settings struct {
	DefaultPermissions []string `json:"default_permissions"`
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

	out := make([]string, 0, len(cfg.DefaultPermissions))
	for _, entry := range cfg.DefaultPermissions {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("config default_permissions is empty")
	}
	return out, nil
}
