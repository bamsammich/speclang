// Package openapi implements an ImportResolver that converts OpenAPI 3.x
// schemas into speclang AST nodes (models and scopes).
//
// It uses kin-openapi for spec parsing and $ref resolution, so the converter
// stays current with OpenAPI spec evolution without custom maintenance.
package openapi

import (
	"github.com/bamsammich/speclang/pkg/parser"
)

// Resolver implements parser.ImportResolver for OpenAPI 3.x specs.
type Resolver struct{}

// Resolve reads an OpenAPI file and returns speclang models and scopes.
func (r *Resolver) Resolve(absPath string) ([]*parser.Model, []*parser.Scope, error) {
	doc, err := loadDocument(absPath)
	if err != nil {
		return nil, nil, err
	}

	models := convertSchemas(doc)
	scopes := convertPaths(doc)

	return models, scopes, nil
}
