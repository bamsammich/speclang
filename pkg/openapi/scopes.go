package openapi

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bamsammich/speclang/pkg/parser"
)

// convertPaths converts OpenAPI paths to speclang scopes.
// Each path+method combination becomes a scope with config (path, method)
// and a contract derived from the request body and response schemas.
func convertPaths(doc *openapi3.T) []*parser.Scope {
	if doc.Paths == nil {
		return nil
	}

	var scopes []*parser.Scope
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			scope := operationToScope(path, method, op)
			scopes = append(scopes, scope)
		}
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })
	return scopes
}

// operationToScope converts a single OpenAPI operation to a speclang Scope.
func operationToScope(path, method string, op *openapi3.Operation) *parser.Scope {
	name := op.OperationID
	if name == "" {
		name = sanitizeScopeName(method, path)
	}

	scope := &parser.Scope{
		Name: name,
		Config: map[string]parser.Expr{
			"path":   parser.LiteralString{Value: path},
			"method": parser.LiteralString{Value: strings.ToUpper(method)},
		},
	}

	contract := &parser.Contract{}

	// Request body → contract input
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if sch := jsonSchemaRef(op.RequestBody.Value.Content); sch != nil {
			contract.Input = schemaRefToFields(sch)
		}
	}

	// Success response → contract output
	if resp := successResponse(op.Responses); resp != nil {
		if sch := jsonSchemaRef(resp.Content); sch != nil {
			contract.Output = schemaRefToFields(sch)
		}
	}

	if contract.Input != nil || contract.Output != nil {
		scope.Contract = contract
	}

	return scope
}

// schemaRefToFields converts an object schema's properties to speclang Fields.
func schemaRefToFields(ref *openapi3.SchemaRef) []*parser.Field {
	sch := ref.Value
	if sch == nil || len(sch.Properties) == 0 {
		return nil
	}

	requiredSet := toSet(sch.Required)
	fieldNames := sortedSchemaKeys(sch.Properties)

	var fields []*parser.Field
	for _, name := range fieldNames {
		f := schemaRefToField(name, sch.Properties[name], !requiredSet[name])
		if f != nil {
			fields = append(fields, f)
		}
	}
	return fields
}

// jsonSchemaRef extracts the schema ref from the application/json media type.
func jsonSchemaRef(content openapi3.Content) *openapi3.SchemaRef {
	if mt := content.Get("application/json"); mt != nil && mt.Schema != nil {
		return mt.Schema
	}
	return nil
}

// successResponse finds the first success response (200 or 201).
func successResponse(responses *openapi3.Responses) *openapi3.Response {
	if responses == nil {
		return nil
	}
	if r := responses.Value("200"); r != nil && r.Value != nil {
		return r.Value
	}
	if r := responses.Value("201"); r != nil && r.Value != nil {
		return r.Value
	}
	return nil
}

// sanitizeScopeName generates a scope name from method and path.
func sanitizeScopeName(method, path string) string {
	name := strings.ToLower(method) + "_" + path
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = strings.ReplaceAll(name, "-", "_")
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_")
	return name
}
