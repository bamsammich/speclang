package openapi

import (
	"fmt"
	"os"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bamsammich/speclang/pkg/parser"
)

// convertSchemas converts OpenAPI component schemas to speclang models.
// Only schemas with type: object and properties are converted.
// Unsupported types (array, number/float, enum) emit warnings to stderr.
func convertSchemas(doc *openapi3.T) []*parser.Model {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil
	}

	var models []*parser.Model
	for name, ref := range doc.Components.Schemas {
		sch := ref.Value
		if sch == nil || !sch.Type.Is("object") || len(sch.Properties) == 0 {
			continue
		}
		m := schemaToModel(name, sch)
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models
}

// schemaToModel converts a single OpenAPI object schema to a speclang Model.
func schemaToModel(name string, sch *openapi3.Schema) *parser.Model {
	requiredSet := toSet(sch.Required)
	m := &parser.Model{Name: name}

	fieldNames := sortedSchemaKeys(sch.Properties)

	for _, fieldName := range fieldNames {
		fieldRef := sch.Properties[fieldName]
		field := schemaRefToField(fieldName, fieldRef, !requiredSet[fieldName])
		if field == nil {
			continue
		}
		m.Fields = append(m.Fields, field)
	}
	return m
}

// schemaRefToField converts an OpenAPI SchemaRef to a speclang Field.
// Returns nil if the type is unsupported.
func schemaRefToField(name string, ref *openapi3.SchemaRef, optional bool) *parser.Field {
	// Handle $ref — reference to another model
	if ref.Ref != "" {
		modelName := refName(ref.Ref)
		if modelName == "" {
			fmt.Fprintf(os.Stderr, "warning: unsupported $ref %q for field %q, skipping\n", ref.Ref, name)
			return nil
		}
		return &parser.Field{
			Name: name,
			Type: parser.TypeExpr{Name: modelName, Optional: optional},
		}
	}

	sch := ref.Value
	if sch == nil {
		return nil
	}

	typeExpr, ok := mapType(sch)
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: unsupported type %q for field %q, skipping\n", sch.Type.Slice(), name)
		return nil
	}
	typeExpr.Optional = optional

	field := &parser.Field{
		Name: name,
		Type: typeExpr,
	}

	field.Constraint = buildConstraint(name, sch)
	return field
}

// mapType maps an OpenAPI schema type to a speclang TypeExpr.
func mapType(sch *openapi3.Schema) (parser.TypeExpr, bool) {
	switch {
	case sch.Type.Is("integer"):
		return parser.TypeExpr{Name: "int"}, true
	case sch.Type.Is("string"):
		return parser.TypeExpr{Name: "string"}, true
	case sch.Type.Is("boolean"):
		return parser.TypeExpr{Name: "bool"}, true
	case sch.Type.Is("number"):
		fmt.Fprintf(os.Stderr, "warning: mapping OpenAPI 'number' to 'int' (no float support)\n")
		return parser.TypeExpr{Name: "int"}, true
	case sch.Type.Is("array"):
		return parser.TypeExpr{}, false
	default:
		return parser.TypeExpr{}, false
	}
}

// buildConstraint creates a speclang constraint expression from OpenAPI
// minimum/maximum/exclusiveMinimum/exclusiveMaximum.
func buildConstraint(fieldName string, sch *openapi3.Schema) parser.Expr {
	ref := parser.FieldRef{Path: fieldName}
	var lower, upper parser.Expr

	if sch.ExclusiveMin {
		if sch.Min != nil {
			lower = parser.BinaryOp{
				Left:  parser.LiteralInt{Value: int(*sch.Min)},
				Op:    "<",
				Right: ref,
			}
		}
	} else if sch.Min != nil {
		lower = parser.BinaryOp{
			Left:  parser.LiteralInt{Value: int(*sch.Min)},
			Op:    "<=",
			Right: ref,
		}
	}

	if sch.ExclusiveMax {
		if sch.Max != nil {
			upper = parser.BinaryOp{
				Left:  ref,
				Op:    "<",
				Right: parser.LiteralInt{Value: int(*sch.Max)},
			}
		}
	} else if sch.Max != nil {
		upper = parser.BinaryOp{
			Left:  ref,
			Op:    "<=",
			Right: parser.LiteralInt{Value: int(*sch.Max)},
		}
	}

	if lower != nil && upper != nil {
		return parser.BinaryOp{
			Left:  lower,
			Op:    "&&",
			Right: upper,
		}
	}
	if lower != nil {
		return lower
	}
	return upper
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func sortedSchemaKeys(m openapi3.Schemas) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
