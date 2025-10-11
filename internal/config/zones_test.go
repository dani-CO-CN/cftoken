package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeZoneName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{" example.com ", "example.com"},
		{"", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeZoneName(tc.input); got != tc.want {
				t.Fatalf("normalizeZoneName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestZoneMapIncludesOverrides(t *testing.T) {
	tmp := t.TempDir()
	overridePath := configFilePath(t, tmp, "zones.json")
	raw := map[string]string{
		"example.com":  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"override.com": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	writeJSON(t, overridePath, raw)

	stubConfigDir(t, tmp)

	got, err := ZoneMap()
	if err != nil {
		t.Fatalf("ZoneMap() error = %v", err)
	}

	if len(got) != len(raw) {
		t.Fatalf("ZoneMap() returned %d entries, want %d", len(got), len(raw))
	}

	for name, wantID := range raw {
		normalized := normalizeZoneName(name)
		if gotID, ok := got[normalized]; !ok {
			t.Fatalf("ZoneMap() missing entry for %q", name)
		} else if gotID != wantID {
			t.Fatalf("ZoneMap()[%q] = %q, want %q", name, gotID, wantID)
		}
	}
}

func TestListConfiguredZonesSources(t *testing.T) {
	tmp := t.TempDir()
	overridePath := configFilePath(t, tmp, "zones.json")
	overrideData := map[string]string{
		"default.com": "override-default",
		"config.com":  "config-zone",
	}
	writeJSON(t, overridePath, overrideData)

	stubConfigDir(t, tmp)

	zones, err := ListConfiguredZones()
	if err != nil {
		t.Fatalf("ListConfiguredZones() error = %v", err)
	}

	if len(zones) == 0 {
		t.Fatalf("ListConfiguredZones() returned no zones")
	}

	found := make(map[string]ZoneEntry, len(zones))
	for _, entry := range zones {
		found[entry.Name] = entry
	}

	if entry, ok := found["default.com"]; !ok {
		t.Fatalf("expected merged entry default.com")
	} else {
		if entry.ID != overrideData["default.com"] {
			t.Fatalf("expected default.com ID %q, got %q", overrideData["default.com"], entry.ID)
		}
		if entry.Source != ZoneSourceConfig {
			t.Fatalf("expected default.com source %q, got %q", ZoneSourceConfig, entry.Source)
		}
	}

	if entry, ok := found["config.com"]; !ok {
		t.Fatalf("expected config-only entry config.com")
	} else if entry.Source != ZoneSourceConfig {
		t.Fatalf("expected config.com source %q, got %q", ZoneSourceConfig, entry.Source)
	}

	if len(found) != len(overrideData) {
		t.Fatalf("expected %d zones, got %d", len(overrideData), len(found))
	}
}

func TestLoadZoneOverridesEmpty(t *testing.T) {
	tmp := t.TempDir()
	writeJSON(t, configFilePath(t, tmp, "zones.json"), map[string]string{})
	stubConfigDir(t, tmp)

	overrides, err := LoadZoneOverrides()
	if err == nil || !strings.Contains(err.Error(), "zones.json contains no valid entries") {
		t.Fatalf("LoadZoneOverrides() error = %v, want empty-entry error", err)
	}
	if overrides != nil {
		t.Fatalf("LoadZoneOverrides() = %v, want nil", overrides)
	}
}

func TestZoneMapErrorPropagation(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, configFilePath(t, tmp, "zones.json"), "{invalid json")
	stubConfigDir(t, tmp)

	if _, err := ZoneMap(); err == nil {
		t.Fatalf("ZoneMap() expected error but got nil")
	}
}

func TestZoneMapMissingConfig(t *testing.T) {
	stubConfigDir(t, t.TempDir())
	if _, err := ZoneMap(); err == nil {
		t.Fatalf("ZoneMap() expected error for missing config")
	}
}

func TestListConfiguredZonesMissingConfig(t *testing.T) {
	stubConfigDir(t, t.TempDir())
	if _, err := ListConfiguredZones(); err == nil {
		t.Fatalf("ListConfiguredZones() expected error for missing config")
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	writeFile(t, path, string(data))
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func stubConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Ensure ~/.config path isn't used.
	t.Setenv("HOME", dir)
}

func configFilePath(t *testing.T, root, name string) string {
	t.Helper()
	return filepath.Join(root, "cftoken", name)
}
