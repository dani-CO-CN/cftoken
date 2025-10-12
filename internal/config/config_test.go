package config

import (
	"errors"
	"io/fs"
	"testing"
)

func TestLoadDefaultPermissions(t *testing.T) {
	tmp := t.TempDir()
	stubConfigDir(t, tmp)

	writeJSON(t, configFilePath(t, tmp, "config.json"), map[string]any{
		"default_permissions": []string{" Zone:Read ", "", "Account:Members"},
	})

	perms, err := LoadDefaultPermissions()
	if err != nil {
		t.Fatalf("LoadDefaultPermissions() error = %v", err)
	}

	if len(perms) != 2 {
		t.Fatalf("LoadDefaultPermissions() returned %d entries, want 2", len(perms))
	}
	want := []string{"Zone:Read", "Account:Members"}
	for i, got := range perms {
		if got != want[i] {
			t.Fatalf("LoadDefaultPermissions()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestLoadDefaultPermissionsEmpty(t *testing.T) {
	tmp := t.TempDir()
	stubConfigDir(t, tmp)

	writeJSON(t, configFilePath(t, tmp, "config.json"), map[string]any{
		"default_permissions": []string{},
	})

	if _, err := LoadDefaultPermissions(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadDefaultPermissions() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadDefaultAllowedCIDRs(t *testing.T) {
	tmp := t.TempDir()
	stubConfigDir(t, tmp)

	writeJSON(t, configFilePath(t, tmp, "config.json"), map[string]any{
		"default_allowed_cidrs": []string{" 10.0.0.1/32 ", "", "10.0.0.2/32"},
	})

	cidrs, err := LoadDefaultAllowedCIDRs()
	if err != nil {
		t.Fatalf("LoadDefaultAllowedCIDRs() error = %v", err)
	}

	if len(cidrs) != 2 {
		t.Fatalf("LoadDefaultAllowedCIDRs() returned %d entries, want 2", len(cidrs))
	}
	want := []string{"10.0.0.1/32", "10.0.0.2/32"}
	for i, got := range cidrs {
		if got != want[i] {
			t.Fatalf("LoadDefaultAllowedCIDRs()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestLoadDefaultAllowedCIDRsEmpty(t *testing.T) {
	tmp := t.TempDir()
	stubConfigDir(t, tmp)

	writeJSON(t, configFilePath(t, tmp, "config.json"), map[string]any{
		"default_allowed_cidrs": []string{},
	})

	if _, err := LoadDefaultAllowedCIDRs(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadDefaultAllowedCIDRs() error = %v, want fs.ErrNotExist", err)
	}
}
