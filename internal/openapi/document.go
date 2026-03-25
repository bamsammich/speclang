package openapi

import (
	"context"
	"fmt"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
)

// loadDocument reads and parses an OpenAPI 3.x file (YAML or JSON)
// using kin-openapi, with full $ref resolution.
func loadDocument(path string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading openapi spec: %w", err)
	}

	// Validate the document structure (catches missing required fields, etc.)
	if err := doc.Validate(
		context.Background(),
		openapi3.DisableSchemaDefaultsValidation(),
	); err != nil {
		return nil, fmt.Errorf("validating openapi spec: %w", err)
	}

	return doc, nil
}

// refName extracts the schema name from a $ref string.
// e.g., "#/components/schemas/Pet" → "Pet"
func refName(ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	frag := u.Fragment
	if frag == "" {
		return ""
	}
	// Fragment is like "/components/schemas/Pet"
	parts := splitPath(frag)
	if len(parts) >= 3 && parts[0] == "components" && parts[1] == "schemas" {
		return parts[2]
	}
	return ""
}

// splitPath splits a URI fragment path, ignoring empty segments.
func splitPath(path string) []string {
	var parts []string
	for _, p := range splitSlash(path) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitSlash(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
