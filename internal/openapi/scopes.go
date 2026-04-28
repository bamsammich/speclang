package openapi

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bamsammich/speclang/v4/internal/parser"
)

// convertPaths converts OpenAPI paths to speclang scopes.
// Each path+method combination becomes a scope containing one contract.
// The contract's action block contains the HTTP call, and its ReturnType
// references the response model (or is empty if there is no response schema).
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
// The scope contains one contract whose action block calls the HTTP endpoint.
func operationToScope(path, method string, op *openapi3.Operation) *parser.Scope {
	name := op.OperationID
	if name == "" {
		name = sanitizeScopeName(method, path)
	}

	scope := &parser.Scope{Name: name}

	contract := &parser.Contract{Name: name}

	// Request body → contract input fields
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if sch := jsonSchemaRef(op.RequestBody.Value.Content); sch != nil {
			contract.Fields = schemaRefToFields(sch)
		}
	}

	// Success response → contract return type
	if resp := successResponse(op.Responses); resp != nil {
		if sch := jsonSchemaRef(resp.Content); sch != nil {
			retType := responseReturnType(sch)
			contract.ReturnType = retType
		}
	}

	// Action block: HTTP call
	contract.Action = buildActionBlock(method, path, contract.Fields)

	scope.Contracts = append(scope.Contracts, contract)
	return scope
}

// responseReturnType derives a TypeExpr from a response schema.
// If the schema references a named model, use that name; otherwise use "any".
func responseReturnType(ref *openapi3.SchemaRef) parser.TypeExpr {
	if ref.Ref != "" {
		if name := refName(ref.Ref); name != "" {
			return parser.TypeExpr{Name: name}
		}
	}
	return parser.TypeExpr{Name: "any"}
}

// buildActionBlock constructs an ActionBlock that calls the HTTP endpoint.
func buildActionBlock(method, path string, fields []*parser.Field) *parser.ActionBlock {
	lowerMethod := strings.ToLower(method)

	// Build the argument list: path string + optional body object
	var args []parser.Expr
	args = append(args, parser.LiteralString{Value: path})

	if (lowerMethod == "post" || lowerMethod == "put" || lowerMethod == "patch") && len(fields) > 0 {
		var objFields []*parser.ObjField
		for _, f := range fields {
			objFields = append(objFields, &parser.ObjField{
				Key:   f.Name,
				Value: parser.FieldRef{Path: f.Name},
			})
		}
		args = append(args, parser.ObjectLiteral{Fields: objFields})
	}

	ret := &parser.ReturnStmt{
		Value: parser.AdapterCall{
			Adapter: "http",
			Method:  lowerMethod,
			Args:    args,
		},
	}

	return &parser.ActionBlock{Body: []parser.GivenStep{ret}}
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
