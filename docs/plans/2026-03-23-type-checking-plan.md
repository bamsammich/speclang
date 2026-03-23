# Type Checking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a post-parse validation pass that type-checks literal expressions against contracts, validates model references, checks given completeness, and validates then targets.

**Architecture:** New `pkg/validator` package with a `Validate(*parser.Spec) []error` function called after parsing in all CLI commands. Builds a model registry from `Spec.Models`, then walks all scopes checking contracts, given blocks, and then blocks. Returns all errors (not just the first). Hierarchical error output grouped by scope/scenario.

**Tech Stack:** Go, standard library only

---

### Task 1: Scaffold `pkg/validator` with model resolution

**Files:**
- Create: `pkg/validator/validator.go`
- Create: `pkg/validator/validator_test.go`

**Step 1: Write the failing test**

Create `pkg/validator/validator_test.go`:

```go
package validator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestValidate_UnknownTypeInContract(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown type Widget")
	}
	found := false
	for _, e := range errs {
		if contains(e.Error(), "Widget") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error mentioning Widget, got: %v", errs)
	}
}

func TestValidate_KnownModelPasses(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Widget", Fields: []*parser.Field{
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_UnknownArrayElementType(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "items", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "Widget"},
						}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown array element type Widget")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/validator/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement minimal validator with model resolution**

Create `pkg/validator/validator.go`:

```go
package validator

import (
	"fmt"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

// primitives are type names that don't need model resolution.
var primitives = map[string]bool{
	"int": true, "string": true, "bool": true,
	"float": true, "bytes": true, "array": true, "map": true,
}

// Validate performs post-parse semantic validation on a spec.
// It returns all errors found (not just the first).
func Validate(spec *parser.Spec) []error {
	v := &validator{
		models: buildModelRegistry(spec.Models),
	}

	for _, scope := range spec.Scopes {
		v.scope = scope.Name
		v.validateContract(scope)
	}

	return v.errs
}

type validator struct {
	models map[string]*parser.Model
	errs   []error
	scope  string
}

func buildModelRegistry(models []*parser.Model) map[string]*parser.Model {
	reg := make(map[string]*parser.Model, len(models))
	for _, m := range models {
		reg[m.Name] = m
	}
	return reg
}

func (v *validator) errorf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) validateContract(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	for _, f := range scope.Contract.Input {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract input %q", v.scope, f.Name))
	}
	for _, f := range scope.Contract.Output {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract output %q", v.scope, f.Name))
	}
}

func (v *validator) validateTypeExpr(te parser.TypeExpr, context string) {
	switch {
	case primitives[te.Name]:
		// Recurse into array element type, map key/val types
		if te.ElemType != nil {
			v.validateTypeExpr(*te.ElemType, context)
		}
		if te.KeyType != nil {
			v.validateTypeExpr(*te.KeyType, context)
		}
		if te.ValType != nil {
			v.validateTypeExpr(*te.ValType, context)
		}
	default:
		// Must be a model name
		if _, ok := v.models[te.Name]; !ok {
			v.errorf("%s: unknown type %q", context, te.Name)
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): scaffold validator with model resolution"
```

---

### Task 2: Add literal type checking for given blocks

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing tests**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_GivenLiteralTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralString{Value: "not_an_int"}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for type mismatch")
	}
}

func TestValidate_GivenLiteralCorrectType(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
						{Name: "name", Type: parser.TypeExpr{Name: "string"}},
						{Name: "active", Type: parser.TypeExpr{Name: "bool"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralInt{Value: 42}},
								&parser.Assignment{Path: "name", Value: parser.LiteralString{Value: "foo"}},
								&parser.Assignment{Path: "active", Value: parser.LiteralBool{Value: true}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_NullOnlyForOptional(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "required_field", Type: parser.TypeExpr{Name: "string"}},
						{Name: "optional_field", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "required_field", Value: parser.LiteralNull{}},
								&parser.Assignment{Path: "optional_field", Value: parser.LiteralNull{}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (null on required), got %d: %v", len(errs), errs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/validator/ -run "TestValidate_Given" -v`
Expected: FAIL (no given block validation yet)

**Step 3: Implement given block literal type checking**

Add to `pkg/validator/validator.go`:

```go
func (v *validator) validateScenarios(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	inputFields := buildFieldMap(scope.Contract.Input)

	for _, sc := range scope.Scenarios {
		v.validateGivenBlock(sc, inputFields)
	}
}

func buildFieldMap(fields []*parser.Field) map[string]*parser.Field {
	m := make(map[string]*parser.Field, len(fields))
	for _, f := range fields {
		m[f.Name] = f
	}
	return m
}

func (v *validator) validateGivenBlock(sc *parser.Scenario, inputFields map[string]*parser.Field) {
	if sc.Given == nil {
		return
	}

	for _, step := range sc.Given.Steps {
		assign, ok := step.(*parser.Assignment)
		if !ok {
			continue // skip action calls
		}

		// Resolve the top-level field name from the path
		fieldName := topLevelField(assign.Path)
		field, ok := inputFields[fieldName]
		if !ok {
			continue // unknown field — not this check's concern (could be a nested path)
		}

		v.checkExprType(assign.Value, field.Type,
			fmt.Sprintf("scope %q, scenario %q, field %q", v.scope, sc.Name, assign.Path))
	}
}

func topLevelField(path string) string {
	for i, c := range path {
		if c == '.' {
			return path[:i]
		}
	}
	return path
}

func (v *validator) checkExprType(expr parser.Expr, te parser.TypeExpr, context string) {
	// LiteralNull is valid only for optional types
	if _, isNull := expr.(parser.LiteralNull); isNull {
		if !te.Optional {
			v.errorf("%s: null is not valid for non-optional type %s", context, typeName(te))
		}
		return
	}

	switch te.Name {
	case "int":
		if _, ok := expr.(parser.LiteralInt); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected int, got %s", context, exprTypeName(expr))
			}
		}
	case "float":
		// Accept both float and int literals for float fields
		switch expr.(type) {
		case parser.LiteralFloat, parser.LiteralInt:
			// ok
		default:
			if !isNonLiteral(expr) {
				v.errorf("%s: expected float, got %s", context, exprTypeName(expr))
			}
		}
	case "string":
		if _, ok := expr.(parser.LiteralString); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected string, got %s", context, exprTypeName(expr))
			}
		}
	case "bool":
		if _, ok := expr.(parser.LiteralBool); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected bool, got %s", context, exprTypeName(expr))
			}
		}
	case "array":
		if _, ok := expr.(parser.ArrayLiteral); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected array, got %s", context, exprTypeName(expr))
			}
		}
		// Array element checking handled in Task 3
	default:
		// Model type — expect ObjectLiteral
		if _, ok := expr.(parser.ObjectLiteral); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected %s (object), got %s", context, te.Name, exprTypeName(expr))
			}
		}
		// Object field checking handled in Task 4
	}
}

// isNonLiteral returns true for expressions that can't be statically type-checked
// (field refs, binary ops, env refs, etc.). We skip these silently.
func isNonLiteral(expr parser.Expr) bool {
	switch expr.(type) {
	case parser.FieldRef, parser.BinaryOp, parser.UnaryOp,
		parser.EnvRef, parser.LenExpr, parser.RegexLiteral:
		return true
	}
	return false
}

func typeName(te parser.TypeExpr) string {
	switch te.Name {
	case "array":
		if te.ElemType != nil {
			return "[]" + typeName(*te.ElemType)
		}
		return "[]unknown"
	case "map":
		return "map"
	default:
		name := te.Name
		if te.Optional {
			name += "?"
		}
		return name
	}
}

func exprTypeName(expr parser.Expr) string {
	switch expr.(type) {
	case parser.LiteralInt:
		return "int literal"
	case parser.LiteralFloat:
		return "float literal"
	case parser.LiteralString:
		return "string literal"
	case parser.LiteralBool:
		return "bool literal"
	case parser.LiteralNull:
		return "null"
	case parser.ArrayLiteral:
		return "array literal"
	case parser.ObjectLiteral:
		return "object literal"
	default:
		return fmt.Sprintf("%T", expr)
	}
}
```

Also update `Validate` to call `validateScenarios`:

```go
func Validate(spec *parser.Spec) []error {
	v := &validator{
		models: buildModelRegistry(spec.Models),
	}

	for _, scope := range spec.Scopes {
		v.scope = scope.Name
		v.validateContract(scope)
		v.validateScenarios(scope)
	}

	return v.errs
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add literal type checking for given blocks"
```

---

### Task 3: Add array element and nested literal type checking

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing tests**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_ArrayElementTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "tags", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "int"},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "tags",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.LiteralInt{Value: 1},
											parser.LiteralString{Value: "oops"},
											parser.LiteralInt{Value: 3},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in []int, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ArrayOfObjectsFieldCheck(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Item", Fields: []*parser.Field{
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
				{Name: "price", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "items", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "Item"},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "items",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.ObjectLiteral{Fields: []*parser.ObjField{
												{Key: "name", Value: parser.LiteralString{Value: "widget"}},
												{Key: "price", Value: parser.LiteralInt{Value: 100}},
											}},
											parser.ObjectLiteral{Fields: []*parser.ObjField{
												{Key: "name", Value: parser.LiteralString{Value: "gadget"}},
												{Key: "colour", Value: parser.LiteralString{Value: "red"}},
											}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	// Should find: items[1] has unknown field "colour", and items[1] missing "price" type check
	if len(errs) == 0 {
		t.Fatal("expected errors for unknown field and type mismatch")
	}
	foundColour := false
	for _, e := range errs {
		if contains(e.Error(), "colour") {
			foundColour = true
		}
	}
	if !foundColour {
		t.Errorf("expected error about unknown field 'colour', got: %v", errs)
	}
}

func TestValidate_NestedArrays(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "matrix", Type: parser.TypeExpr{
							Name: "array",
							ElemType: &parser.TypeExpr{
								Name:     "array",
								ElemType: &parser.TypeExpr{Name: "int"},
							},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "matrix",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.ArrayLiteral{Elements: []parser.Expr{
												parser.LiteralInt{Value: 1},
											}},
											parser.ArrayLiteral{Elements: []parser.Expr{
												parser.LiteralString{Value: "bad"},
											}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in nested [][]int, got %d: %v", len(errs), errs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/validator/ -run "TestValidate_Array|TestValidate_Nested" -v`
Expected: FAIL

**Step 3: Add array element recursion to `checkExprType`**

In the `case "array":` branch of `checkExprType`, after checking the top-level type, add element recursion:

```go
	case "array":
		arr, ok := expr.(parser.ArrayLiteral)
		if !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected array, got %s", context, exprTypeName(expr))
			}
			return
		}
		if te.ElemType != nil {
			for i, elem := range arr.Elements {
				v.checkExprType(elem, *te.ElemType,
					fmt.Sprintf("%s[%d]", context, i))
			}
		}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add array element type checking with recursion"
```

---

### Task 4: Add object literal field validation against models

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing tests**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_ObjectLiteralUnknownField(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
										{Key: "email", Value: parser.LiteralString{Value: "alice@test.com"}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown field email, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "email") {
		t.Errorf("expected error about 'email', got: %v", errs[0])
	}
}

func TestValidate_ObjectLiteralFieldTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralString{Value: "not_an_int"}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for balance type mismatch, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ObjectLiteralValidPasses(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/validator/ -run "TestValidate_ObjectLiteral" -v`
Expected: FAIL

**Step 3: Add object literal field validation to `checkExprType`**

In the `default:` (model type) branch of `checkExprType`, after the ObjectLiteral type check, add field validation:

```go
	default:
		// Model type — expect ObjectLiteral
		obj, ok := expr.(parser.ObjectLiteral)
		if !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected %s (object), got %s", context, te.Name, exprTypeName(expr))
			}
			return
		}
		// Validate object fields against model
		model, ok := v.models[te.Name]
		if !ok {
			return // unknown model — already reported by validateContract
		}
		modelFields := make(map[string]*parser.Field, len(model.Fields))
		for _, f := range model.Fields {
			modelFields[f.Name] = f
		}
		for _, of := range obj.Fields {
			mf, ok := modelFields[of.Key]
			if !ok {
				v.errorf("%s: unknown field %q in model %s", context, of.Key, te.Name)
				continue
			}
			v.checkExprType(of.Value, mf.Type,
				fmt.Sprintf("%s.%s", context, of.Key))
		}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add object literal field validation against models"
```

---

### Task 5: Add given completeness check

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing tests**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_GivenMissingRequiredField(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "string"}},
						{Name: "to", Type: parser.TypeExpr{Name: "string"}},
						{Name: "note", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "from", Value: parser.LiteralString{Value: "alice"}},
								// "to" is missing and required
								// "note" is missing but optional — should not error
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing 'to', got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "to") {
		t.Errorf("expected error about 'to', got: %v", errs[0])
	}
}

func TestValidate_GivenWithCallsSkipsCompleteness(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "playwright",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "username", Type: parser.TypeExpr{Name: "string"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "ui_flow",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Call{Namespace: "playwright", Method: "fill"},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (given with calls skips completeness), got: %v", errs)
	}
}

func TestValidate_WhenScenarioSkipsCompleteness(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "generative",
						When: &parser.Block{
							Predicates: []parser.Expr{
								parser.BinaryOp{Left: parser.FieldRef{Path: "amount"}, Op: ">", Right: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (when scenario skips completeness), got: %v", errs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/validator/ -run "TestValidate_Given.*Missing|TestValidate_Given.*Calls|TestValidate_When" -v`
Expected: FAIL

**Step 3: Add completeness check to `validateGivenBlock`**

Add to `validateGivenBlock`, after the type checking loop:

```go
func (v *validator) validateGivenBlock(sc *parser.Scenario, inputFields map[string]*parser.Field) {
	if sc.Given == nil {
		return
	}

	// Check if given block contains any calls — if so, skip completeness
	hasCalls := false
	for _, step := range sc.Given.Steps {
		if _, ok := step.(*parser.Call); ok {
			hasCalls = true
			break
		}
	}

	// Type-check assignments
	for _, step := range sc.Given.Steps {
		assign, ok := step.(*parser.Assignment)
		if !ok {
			continue
		}

		fieldName := topLevelField(assign.Path)
		field, ok := inputFields[fieldName]
		if !ok {
			continue
		}

		v.checkExprType(assign.Value, field.Type,
			fmt.Sprintf("scope %q, scenario %q, field %q", v.scope, sc.Name, assign.Path))
	}

	// Completeness check: all required fields must be assigned
	// Skip for given blocks with action calls (e.g., Playwright flows)
	if !hasCalls {
		assigned := make(map[string]bool)
		for _, step := range sc.Given.Steps {
			if a, ok := step.(*parser.Assignment); ok {
				assigned[topLevelField(a.Path)] = true
			}
		}
		for _, f := range inputFields {
			if !f.Type.Optional && !assigned[f.Name] {
				v.errorf("scope %q, scenario %q: missing required field %q",
					v.scope, sc.Name, f.Name)
			}
		}
	}
}
```

Also ensure `when`-only scenarios skip given validation entirely. In `validateScenarios`, only call `validateGivenBlock` for scenarios that have `Given` (already handled by the nil check), and skip completeness for scenarios that have `When` instead:

The current code already handles this because `validateGivenBlock` returns early if `sc.Given == nil`, and when-scenarios have `Given == nil`.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add given completeness check"
```

---

### Task 6: Add then field validation

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing tests**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_ThenUnknownField(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "x", Type: parser.TypeExpr{Name: "int"}},
					},
					Output: []*parser.Field{
						{Name: "result", Type: parser.TypeExpr{Name: "int"}},
						{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "result", Expected: parser.LiteralInt{Value: 42}},
								{Target: "typo_field", Expected: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown then target, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "typo_field") {
		t.Errorf("expected error about 'typo_field', got: %v", errs[0])
	}
}

func TestValidate_ThenDotPathValid(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
					Output: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
						{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
									}},
								},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "from.balance", Expected: parser.LiteralInt{Value: 70}},
								{Target: "error", Expected: parser.LiteralNull{}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_ThenPluginAssertionSkipped(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "playwright",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "x", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "ui",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "welcome", Plugin: "playwright", Property: "visible", Expected: parser.LiteralBool{Value: true}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (plugin assertions skipped), got: %v", errs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/validator/ -run "TestValidate_Then" -v`
Expected: FAIL

**Step 3: Add then validation**

Add to `pkg/validator/validator.go`:

```go
func (v *validator) validateThenBlock(sc *parser.Scenario, scope *parser.Scope) {
	if sc.Then == nil || scope.Contract == nil {
		return
	}

	outputFields := buildFieldMap(scope.Contract.Output)

	for _, a := range sc.Then.Assertions {
		// Skip plugin assertions (e.g., welcome@playwright.visible)
		if a.Plugin != "" {
			continue
		}
		// Skip expression assertions (invariant-style, no Target)
		if a.Target == "" {
			continue
		}

		fieldName := topLevelField(a.Target)
		if _, ok := outputFields[fieldName]; !ok {
			v.errorf("scope %q, scenario %q: then target %q does not match any output field",
				v.scope, sc.Name, a.Target)
		}
	}
}
```

Update `validateScenarios` to call `validateThenBlock`:

```go
func (v *validator) validateScenarios(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	inputFields := buildFieldMap(scope.Contract.Input)

	for _, sc := range scope.Scenarios {
		v.validateGivenBlock(sc, inputFields)
		v.validateThenBlock(sc, scope)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add then field validation"
```

---

### Task 7: Add multi-error collection test and hierarchical output

**Files:**
- Modify: `pkg/validator/validator.go`
- Modify: `pkg/validator/validator_test.go`

**Step 1: Write the failing test**

Append to `pkg/validator/validator_test.go`:

```go
func TestValidate_MultipleErrors(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
						{Name: "name", Type: parser.TypeExpr{Name: "string"}},
					},
					Output: []*parser.Field{
						{Name: "result", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "bad",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralString{Value: "wrong"}},
								// name is missing
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "typo", Expected: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 errors (type mismatch + missing field + bad then target), got %d: %v", len(errs), errs)
	}
}

func TestFormatErrors(t *testing.T) {
	errs := []error{
		fmt.Errorf("scope %q, contract input %q: unknown type %q", "orders", "items", "Itme"),
		fmt.Errorf("scope %q, scenario %q, field %q: expected int, got string literal", "orders", "smoke", "count"),
		fmt.Errorf("scope %q, scenario %q: missing required field %q", "orders", "smoke", "name"),
		fmt.Errorf("scope %q, scenario %q: then target %q does not match any output field", "transfer", "basic", "balnce"),
	}

	output := FormatErrors(errs)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(output, "orders") || !contains(output, "transfer") {
		t.Error("expected hierarchical output grouped by scope")
	}
}
```

Add `import "fmt"` to the test file imports if not present.

**Step 2: Run tests to verify the format test fails**

Run: `go test ./pkg/validator/ -run "TestFormatErrors" -v`
Expected: FAIL (FormatErrors undefined)

**Step 3: Implement `FormatErrors`**

Add to `pkg/validator/validator.go`:

```go
import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

// FormatErrors formats validation errors in a hierarchical display
// grouped by scope, then by context (contract/scenario).
func FormatErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	// Group errors by scope
	type scopeErrors struct {
		contract []string
		scenarios map[string][]string
	}
	scopes := make(map[string]*scopeErrors)
	var scopeOrder []string
	ungrouped := []string{}

	for _, err := range errs {
		msg := err.Error()
		scopeName, rest, ok := extractScope(msg)
		if !ok {
			ungrouped = append(ungrouped, msg)
			continue
		}
		se, exists := scopes[scopeName]
		if !exists {
			se = &scopeErrors{scenarios: make(map[string][]string)}
			scopes[scopeName] = se
			scopeOrder = append(scopeOrder, scopeName)
		}

		scenarioName, detail, ok := extractScenario(rest)
		if ok {
			se.scenarios[scenarioName] = append(se.scenarios[scenarioName], detail)
		} else {
			se.contract = append(se.contract, rest)
		}
	}

	var b strings.Builder
	b.WriteString("validation errors:\n")

	for _, name := range scopeOrder {
		se := scopes[name]
		fmt.Fprintf(&b, "\n  scope %s:\n", name)
		if len(se.contract) > 0 {
			b.WriteString("    contract:\n")
			for _, msg := range se.contract {
				fmt.Fprintf(&b, "      - %s\n", msg)
			}
		}
		for scName, msgs := range se.scenarios {
			fmt.Fprintf(&b, "    scenario %s:\n", scName)
			for _, msg := range msgs {
				fmt.Fprintf(&b, "      - %s\n", msg)
			}
		}
	}

	for _, msg := range ungrouped {
		fmt.Fprintf(&b, "  - %s\n", msg)
	}

	return b.String()
}

// extractScope parses 'scope "name", rest' from an error message.
func extractScope(msg string) (scope, rest string, ok bool) {
	const prefix = "scope \""
	if !strings.HasPrefix(msg, prefix) {
		return "", "", false
	}
	msg = msg[len(prefix):]
	idx := strings.Index(msg, "\"")
	if idx < 0 {
		return "", "", false
	}
	scope = msg[:idx]
	rest = strings.TrimLeft(msg[idx+1:], ", ")
	return scope, rest, true
}

// extractScenario parses 'scenario "name", rest' from the remainder.
func extractScenario(msg string) (scenario, rest string, ok bool) {
	const prefix = "scenario \""
	if !strings.HasPrefix(msg, prefix) {
		return "", "", false
	}
	msg = msg[len(prefix):]
	idx := strings.Index(msg, "\"")
	if idx < 0 {
		return "", "", false
	}
	scenario = msg[:idx]
	rest = strings.TrimLeft(msg[idx+1:], ", :")
	rest = strings.TrimSpace(rest)
	return scenario, rest, true
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/validator/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/validator/validator.go pkg/validator/validator_test.go
git commit -m "feat(validator): add hierarchical error formatting"
```

---

### Task 8: Integrate validator into CLI commands

**Files:**
- Modify: `cmd/specrun/main.go`

**Step 1: Write no new tests (integration verified by existing self-verification)**

The integration is straightforward: call `Validate()` after `Parse()` in each command.

**Step 2: Add validator import and calls**

In `cmd/specrun/main.go`, add import:

```go
"github.com/bamsammich/speclang/v2/pkg/validator"
```

Add a helper function:

```go
func validateSpec(spec *parser.Spec) int {
	errs := validator.Validate(spec)
	if len(errs) > 0 {
		fmt.Fprint(os.Stderr, validator.FormatErrors(errs))
		return 1
	}
	return 0
}
```

In `runParse`, after parsing succeeds (after line 52), add:

```go
	if code := validateSpec(spec); code != 0 {
		return code
	}
```

In `runGenerate`, after parsing succeeds (after line 106), add the same call.

In `runVerify`, after parsing succeeds (after line 157), add the same call.

**Step 3: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 4: Run self-verification**

Run: `go build -o specrun ./cmd/specrun && SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec`
Expected: PASS (existing specs are valid)

**Step 5: Manual smoke test with invalid spec**

```bash
echo 'spec Test {
  scope test {
    use http
    contract {
      input { count: int }
      output { result: int }
    }
    scenario bad {
      given { count: "wrong" }
      then { typo: 0 }
    }
  }
}' > /tmp/bad.spec && ./specrun parse /tmp/bad.spec
```
Expected: Hierarchical error output to stderr, exit code 1

**Step 6: Commit**

```bash
git add cmd/specrun/main.go
git commit -m "feat(cli): integrate validator into parse, generate, and verify commands"
```

---

### Task 9: Verify existing specs pass validation

**Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 2: Parse all example specs**

```bash
go build -o specrun ./cmd/specrun
./specrun parse examples/transfer.spec
./specrun parse examples/openapi/petstore.spec
./specrun parse examples/proto/user.spec
```
Expected: All parse without validation errors

**Step 3: Run self-verification**

Run: `SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec`
Expected: ALL PASS

**Step 4: Clean up**

```bash
rm -f /tmp/bad.spec ./specrun
```

---

### Task 10: Update skills and syntax reference

**Files:**
- Modify: `skills/author/references/api_reference.md`
- Modify: `skills/verify/SKILL.md`

**Step 1: Update syntax reference**

Add a "Validation" section to the API reference explaining what the validator checks and what error messages look like.

**Step 2: Update verify skill**

Add a note about validation errors to the verify skill, explaining that specs are now validated before verification runs.

**Step 3: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add skills/author/references/api_reference.md skills/verify/SKILL.md
git commit -m "docs: update skills for validation support"
```
