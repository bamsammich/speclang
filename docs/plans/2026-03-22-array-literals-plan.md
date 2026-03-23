# Array Literals Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support array literal expressions (`[1, 2, 3]`) everywhere expressions are valid — `given`, `then`, `when`, invariants, constraints.

**Architecture:** Add `ArrayLiteral` AST node mirroring `ObjectLiteral`, a `parseArrayLiteral` function in the parser, and eval cases in both the generator and runner. No lexer, adapter, or shrinker changes needed.

**Tech Stack:** Go, standard library only

---

### Task 1: Add `ArrayLiteral` AST node

**Files:**
- Modify: `pkg/parser/ast.go:170-198`

**Step 1: Write the failing test**

Add to `pkg/parser/array_test.go`:

```go
func TestParseArrayLiteral(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { items: []int }
      output { ok: bool }
    }
    scenario smoke {
      given {
        items: [1, 2, 3]
      }
      then {
        ok: true
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	sc := spec.Scopes[0].Scenarios[0]
	a, ok := sc.Given.Steps[0].(*Assignment)
	if !ok {
		t.Fatalf("expected *Assignment, got %T", sc.Given.Steps[0])
	}
	arr, ok := a.Value.(ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral, got %T", a.Value)
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr.Elements))
	}
	for i, want := range []int{1, 2, 3} {
		lit, ok := arr.Elements[i].(LiteralInt)
		if !ok {
			t.Fatalf("elements[%d]: expected LiteralInt, got %T", i, arr.Elements[i])
		}
		if lit.Value != want {
			t.Errorf("elements[%d] = %d, want %d", i, lit.Value, want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/parser/ -run TestParseArrayLiteral -v`
Expected: FAIL (compilation error — `ArrayLiteral` undefined)

**Step 3: Add `ArrayLiteral` type to AST**

In `pkg/parser/ast.go`, after `ObjectLiteral` (line 177), add:

```go
type ArrayLiteral struct {
	Elements []Expr `json:"elements,omitempty"`
}
```

After `func (ObjectLiteral) exprNode() {}` (line 195), add:

```go
func (ArrayLiteral) exprNode() {}
```

**Step 4: Run test to verify it still fails (correctly)**

Run: `go test ./pkg/parser/ -run TestParseArrayLiteral -v`
Expected: FAIL with parse error `unexpected token LBracket` (AST exists but parser doesn't handle it yet)

**Step 5: Commit**

```bash
git add pkg/parser/ast.go pkg/parser/array_test.go
git commit -m "feat(parser): add ArrayLiteral AST node and failing test"
```

---

### Task 2: Implement `parseArrayLiteral` in parser

**Files:**
- Modify: `pkg/parser/parser.go:1058-1118` (parseAtom) and append `parseArrayLiteral`

**Step 1: Add `case TokenLBracket:` to `parseAtom`**

In `pkg/parser/parser.go`, inside `parseAtom`, add a case before the `TokenLParen` case (after line 1095):

```go
	case TokenLBracket:
		return p.parseArrayLiteral()
```

**Step 2: Add `parseArrayLiteral` function**

After `parseObjectLiteral` (line 1209), add:

```go
// parseArrayLiteral parses: [ expr, expr, ... ]
func (p *parser) parseArrayLiteral() (Expr, error) {
	p.advance() // consume [
	arr := ArrayLiteral{}

	for p.peek().Type != TokenRBracket {
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		arr.Elements = append(arr.Elements, elem)
		if p.peek().Type == TokenComma {
			p.advance()
		}
	}

	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return arr, nil
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./pkg/parser/ -run TestParseArrayLiteral -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/parser/parser.go
git commit -m "feat(parser): implement parseArrayLiteral"
```

---

### Task 3: Add parser tests for edge cases

**Files:**
- Modify: `pkg/parser/array_test.go`

**Step 1: Add edge case tests**

Append to `pkg/parser/array_test.go`:

```go
func TestParseArrayLiteral_Empty(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { items: []int }
      output { ok: bool }
    }
    scenario smoke {
      given { items: [] }
      then { ok: true }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Given.Steps[0].(*Assignment)
	arr, ok := a.Value.(ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral, got %T", a.Value)
	}
	if len(arr.Elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arr.Elements))
	}
}

func TestParseArrayLiteral_Nested(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { matrix: [][]int }
      output { ok: bool }
    }
    scenario smoke {
      given { matrix: [[1, 2], [3, 4]] }
      then { ok: true }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Given.Steps[0].(*Assignment)
	arr, ok := a.Value.(ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral, got %T", a.Value)
	}
	if len(arr.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr.Elements))
	}
	inner, ok := arr.Elements[0].(ArrayLiteral)
	if !ok {
		t.Fatalf("expected nested ArrayLiteral, got %T", arr.Elements[0])
	}
	if len(inner.Elements) != 2 {
		t.Errorf("expected 2 inner elements, got %d", len(inner.Elements))
	}
}

func TestParseArrayLiteral_Objects(t *testing.T) {
	spec, err := Parse(`
spec Test {
  model Item { name: string; price: int }
  scope test {
    use http
    contract {
      input { items: []Item }
      output { total: int }
    }
    scenario smoke {
      given {
        items: [
          { name: "widget", price: 100 },
          { name: "gadget", price: 200 }
        ]
      }
      then { total: 300 }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Given.Steps[0].(*Assignment)
	arr, ok := a.Value.(ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral, got %T", a.Value)
	}
	if len(arr.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr.Elements))
	}
	obj, ok := arr.Elements[0].(ObjectLiteral)
	if !ok {
		t.Fatalf("expected ObjectLiteral, got %T", arr.Elements[0])
	}
	if len(obj.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(obj.Fields))
	}
}

func TestParseArrayLiteral_TrailingComma(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { items: []int }
      output { ok: bool }
    }
    scenario smoke {
      given { items: [1, 2, 3,] }
      then { ok: true }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Given.Steps[0].(*Assignment)
	arr := a.Value.(ArrayLiteral)
	if len(arr.Elements) != 3 {
		t.Errorf("expected 3 elements (trailing comma), got %d", len(arr.Elements))
	}
}
```

**Step 2: Run tests**

Run: `go test ./pkg/parser/ -run TestParseArrayLiteral -v`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add pkg/parser/array_test.go
git commit -m "test(parser): add array literal edge case tests"
```

---

### Task 4: Add `ArrayLiteral` eval to generator

**Files:**
- Modify: `pkg/generator/generator.go:256-302` (evalCtx.eval)
- Modify: `pkg/generator/array_test.go`

**Step 1: Write the failing test**

Append to `pkg/generator/array_test.go`:

```go
func TestEvalArrayLiteral(t *testing.T) {
	ctx := &evalCtx{input: map[string]any{}}

	// Simple array
	val, ok := ctx.eval(parser.ArrayLiteral{
		Elements: []parser.Expr{
			parser.LiteralInt{Value: 1},
			parser.LiteralInt{Value: 2},
			parser.LiteralInt{Value: 3},
		},
	})
	if !ok {
		t.Fatal("eval returned not ok")
	}
	arr, ok := val.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", val)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	for i, want := range []int{1, 2, 3} {
		if arr[i] != want {
			t.Errorf("arr[%d] = %v, want %d", i, arr[i], want)
		}
	}

	// Empty array
	val, ok = ctx.eval(parser.ArrayLiteral{})
	if !ok {
		t.Fatal("eval empty array returned not ok")
	}
	arr = val.([]any)
	if len(arr) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arr))
	}

	// Nested array of objects
	val, ok = ctx.eval(parser.ArrayLiteral{
		Elements: []parser.Expr{
			parser.ObjectLiteral{Fields: []*parser.ObjField{
				{Key: "name", Value: parser.LiteralString{Value: "a"}},
			}},
		},
	})
	if !ok {
		t.Fatal("eval nested returned not ok")
	}
	arr = val.([]any)
	inner, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", arr[0])
	}
	if inner["name"] != "a" {
		t.Errorf("inner[name] = %v, want 'a'", inner["name"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/generator/ -run TestEvalArrayLiteral -v`
Expected: FAIL (eval returns `nil, false` for `ArrayLiteral` — hits default case)

**Step 3: Add `ArrayLiteral` case to `evalCtx.eval`**

In `pkg/generator/generator.go`, inside `evalCtx.eval`, after the `parser.ObjectLiteral` case (line 281), add:

```go
	case parser.ArrayLiteral:
		result := make([]any, len(e.Elements))
		for i, elem := range e.Elements {
			v, ok := c.eval(elem)
			if !ok {
				return nil, false
			}
			result[i] = v
		}
		return result, true
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/generator/ -run TestEvalArrayLiteral -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/generator/generator.go pkg/generator/array_test.go
git commit -m "feat(generator): add ArrayLiteral eval support"
```

---

### Task 5: Add `ArrayLiteral` to runner's `exprToValue`

**Files:**
- Modify: `pkg/runner/runner.go:728-750` (exprToValue)

**Step 1: Add `ArrayLiteral` case**

In `pkg/runner/runner.go`, inside `exprToValue`, after the `parser.ObjectLiteral` case (line 746), add:

```go
	case parser.ArrayLiteral:
		result := make([]any, len(e.Elements))
		for i, elem := range e.Elements {
			result[i] = exprToValue(elem)
		}
		return result
```

**Step 2: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add pkg/runner/runner.go
git commit -m "feat(runner): add ArrayLiteral to exprToValue"
```

---

### Task 6: Update skills and syntax reference

**Files:**
- Modify: `skills/author/references/api_reference.md`

**Step 1: Check current reference for expression docs**

Read `skills/author/references/api_reference.md` and find the expressions section.

**Step 2: Add array literal to the expression reference**

Add `[expr, expr, ...]` to the list of expression forms, with a brief description and example.

**Step 3: Run all tests to confirm nothing broke**

Run: `go test ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add skills/author/references/api_reference.md
git commit -m "docs: add array literal to syntax reference"
```

---

### Task 7: File follow-up issue for type checking

**Step 1: Create GitHub issue**

```bash
gh issue create \
  --title "Add compile-time type checking for literal expressions" \
  --body "Array literals and object literals in \`given\`/\`then\` blocks are not validated against contract types at parse time. For example, \`items: [\"not_an_int\"]\` would be accepted by the parser for an \`items: []int\` contract and only fail at runtime.

Add a type-checking pass that validates:
- Array literal elements match the declared element type
- Object literal fields match model field names and types
- Nested literals are checked recursively

This was identified during #29 (array literal support) as a separate concern."
```

**Step 2: Commit** (nothing to commit — issue only)

---

### Task 8: Verify end-to-end

**Step 1: Run the full test suite**

Run: `go test ./...`
Expected: ALL PASS

**Step 2: Manual smoke test with parse command**

Create a temp spec and run `specrun parse` on it to verify the AST JSON output includes `ArrayLiteral` nodes correctly.

Run: `go run ./cmd/specrun parse <(cat <<'EOF'
spec Test {
  scope test {
    use http
    contract {
      input { items: []int }
      output { ok: bool }
    }
    scenario smoke {
      given { items: [1, 2, 3] }
      then { ok: true }
    }
  }
}
EOF
)`

Expected: JSON AST with array literal elements visible

**Step 3: Run self-verification**

Run: `go run ./cmd/specrun verify specs/speclang.spec`
Expected: PASS (existing self-verification still works)
