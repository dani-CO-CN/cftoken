package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ZoneSource indicates where a zone entry originated from.
type ZoneSource string

const (
	// ZoneSourceConfig marks zones read from the user's configuration file.
	ZoneSourceConfig ZoneSource = "config"
)

// ZoneEntry is a normalized zone name and ID paired with its source.
type ZoneEntry struct {
	Name   string
	ID     string
	Source ZoneSource
}

// LoadZoneOverrides reads user-defined zones and extracts zone IDs.
func LoadZoneOverrides() (map[string]string, error) {
	cfg, err := loadSettings()
	if err != nil {
		return nil, err
	}

	out := sanitizeZones(cfg.Zones)
	if len(out) == 0 {
		return nil, errors.New("config zones contains no valid entries")
	}
	return out, nil
}

// ZoneMap returns a map of zone names to zone IDs.
func ZoneMap() (map[string]string, error) {
	overrides, err := LoadZoneOverrides()
	if err != nil {
		return nil, err
	}

	merged := make(map[string]string, len(overrides))
	for name, id := range overrides {
		merged[name] = id
	}
	return merged, nil
}

// ListConfiguredZones returns every zone declared in the configuration file.
func ListConfiguredZones() ([]ZoneEntry, error) {
	entries := make(map[string]ZoneEntry)

	if overrides, err := LoadZoneOverrides(); err == nil {
		for name, id := range overrides {
			n := normalizeZoneName(name)
			if n == "" {
				continue
			}
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				entries[n] = ZoneEntry{Name: n, ID: trimmed, Source: ZoneSourceConfig}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else {
		return nil, err
	}

	out := make([]ZoneEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

// ResolveZoneID returns the zone ID for the supplied zone name using the merged map.
func ResolveZoneID(zoneName string) (string, error) {
	zones, err := ZoneMap()
	if err != nil {
		return "", err
	}

	name := normalizeZoneName(zoneName)
	if name == "" {
		return "", errors.New("zone name is empty")
	}
	if id, ok := zones[name]; ok && id != "" {
		return id, nil
	}
	return "", fmt.Errorf("zone %q not found in default or configured zones", zoneName)
}

func sanitizeZones(values map[string]interface{}) map[string]string {
	if len(values) == 0 {
		return nil
	}

	out := make(map[string]string, len(values))
	for name, value := range values {
		n := normalizeZoneName(name)
		if n == "" {
			continue
		}

		// Handle simple string zone ID
		if zoneID, ok := value.(string); ok {
			if trimmed := strings.TrimSpace(zoneID); trimmed != "" {
				out[n] = trimmed
			}
			continue
		}

		// Handle complex zone object with zone_id field
		if zoneMap, ok := value.(map[string]interface{}); ok {
			if zoneIDValue, exists := zoneMap["zone_id"]; exists {
				if zoneID, ok := zoneIDValue.(string); ok {
					if trimmed := strings.TrimSpace(zoneID); trimmed != "" {
						out[n] = trimmed
					}
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeZoneName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.TrimSuffix(s, ".")
}
