package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderPolicies_Inline(t *testing.T) {
	inlineTemplate := `[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID }}": "*"
    },
    "permission_groups": [
      {
        "id": "{{ .PermissionID }}",
        "name": "Zone Read"
      }
    ]
  }
]`

	vars := Variables{
		"ZoneID":       "abc123",
		"PermissionID": "perm-id-123",
	}

	policies, err := RenderPolicies("", inlineTemplate, vars)
	if err != nil {
		t.Fatalf("RenderPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	policy := policies[0]
	if policy.Effect != "allow" {
		t.Errorf("expected effect 'allow', got %s", policy.Effect)
	}
	if len(policy.PermissionGroups) != 1 {
		t.Fatalf("expected 1 permission group, got %d", len(policy.PermissionGroups))
	}
	if policy.PermissionGroups[0].ID != "perm-id-123" {
		t.Errorf("expected permission ID 'perm-id-123', got %s", policy.PermissionGroups[0].ID)
	}
}

func TestRenderPolicies_File(t *testing.T) {
	// Create temp template file
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "policy.json.tmpl")

	templateContent := `[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID }}": "*"
    },
    "permission_groups": [
      {
        "id": "zone-read-id"
      },
      {
        "id": "zone-edit-id"
      }
    ]
  }
]`

	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	vars := Variables{
		"ZoneID": "test-zone-123",
	}

	policies, err := RenderPolicies(templatePath, "", vars)
	if err != nil {
		t.Fatalf("RenderPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	policy := policies[0]
	if len(policy.PermissionGroups) != 2 {
		t.Errorf("expected 2 permission groups, got %d", len(policy.PermissionGroups))
	}
}

func TestRenderPolicies_MultipleZones(t *testing.T) {
	inlineTemplate := `[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID1 }}": "*",
      "com.cloudflare.api.account.zone.{{ .ZoneID2 }}": "*"
    },
    "permission_groups": [
      {
        "id": "zone-read-id"
      }
    ]
  }
]`

	vars := Variables{
		"ZoneID1": "zone-abc",
		"ZoneID2": "zone-xyz",
	}

	policies, err := RenderPolicies("", inlineTemplate, vars)
	if err != nil {
		t.Fatalf("RenderPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	policy := policies[0]
	if len(policy.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(policy.Resources))
	}
}

func TestRenderPolicies_NoTemplate(t *testing.T) {
	vars := Variables{}
	_, err := RenderPolicies("", "", vars)
	if err == nil {
		t.Error("expected error when no template provided, got nil")
	}
}

func TestRenderPolicies_InvalidJSON(t *testing.T) {
	inlineTemplate := `[{ "effect": "allow" invalid json }]`

	vars := Variables{}
	_, err := RenderPolicies("", inlineTemplate, vars)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestRenderPolicies_AccountLevel(t *testing.T) {
	inlineTemplate := `[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.{{ .AccountID }}": "*"
    },
    "permission_groups": [
      {
        "id": "account-read-id"
      }
    ]
  }
]`

	vars := Variables{
		"AccountID": "acc-123",
	}

	policies, err := RenderPolicies("", inlineTemplate, vars)
	if err != nil {
		t.Fatalf("RenderPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	policy := policies[0]
	resourceKey := "com.cloudflare.api.account.acc-123"
	if _, exists := policy.Resources[resourceKey]; !exists {
		t.Errorf("expected resource key %s not found", resourceKey)
	}
}

func TestRenderPolicies_AutoInjectedZoneID(t *testing.T) {
	// This test simulates what happens when ZoneID is auto-injected from zone config
	inlineTemplate := `[
  {
    "effect": "allow",
    "resources": {
      "com.cloudflare.api.account.zone.{{ .ZoneID }}": "*"
    },
    "permission_groups": [
      {
        "id": "zone-read-id"
      }
    ]
  }
]`

	// ZoneID would be auto-injected by main.go from zoneConfig.ZoneID
	vars := Variables{
		"ZoneID": "auto-injected-zone-123",
	}

	policies, err := RenderPolicies("", inlineTemplate, vars)
	if err != nil {
		t.Fatalf("RenderPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	policy := policies[0]
	expectedKey := "com.cloudflare.api.account.zone.auto-injected-zone-123"
	if _, exists := policy.Resources[expectedKey]; !exists {
		t.Errorf("expected resource key %s not found in resources: %v", expectedKey, policy.Resources)
	}
}
