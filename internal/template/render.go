package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Variables holds the context for template rendering.
type Variables map[string]interface{}

// Policy represents a full Cloudflare API token policy.
type Policy struct {
	ID               string                 `json:"id,omitempty"`
	Effect           string                 `json:"effect"`
	Resources        map[string]interface{} `json:"resources"`
	PermissionGroups []PermissionGroup      `json:"permission_groups"`
}

// PermissionGroup represents a permission group in a policy.
type PermissionGroup struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// RenderPolicies renders a template and returns Cloudflare API token policies.
// The template must render to a JSON array of policy objects.
func RenderPolicies(templatePath, inlineTemplate string, vars Variables) ([]Policy, error) {
	if templatePath == "" && inlineTemplate == "" {
		return nil, fmt.Errorf("either template_file or template_inline must be specified")
	}

	var templateContent string
	var templateName string

	if inlineTemplate != "" {
		templateContent = inlineTemplate
		templateName = "inline"
	} else {
		expandedPath, err := expandPath(templatePath)
		if err != nil {
			return nil, fmt.Errorf("expand template path: %w", err)
		}

		data, err := os.ReadFile(expandedPath)
		if err != nil {
			return nil, fmt.Errorf("read template file %s: %w", expandedPath, err)
		}
		templateContent = string(data)
		templateName = filepath.Base(expandedPath)
	}

	// Create template with plain Go template syntax
	tmpl, err := template.New(templateName).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	rendered := strings.TrimSpace(buf.String())

	// Parse as policy array
	var policies []Policy
	if err := json.Unmarshal([]byte(rendered), &policies); err != nil {
		return nil, fmt.Errorf("parse rendered template as policies: %w\nRendered content:\n%s", err, rendered)
	}

	return policies, nil
}

// expandPath expands ~ and environment variables in a file path.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	return os.ExpandEnv(path), nil
}
