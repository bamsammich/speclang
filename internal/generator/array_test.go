package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v3/internal/parser"
)

func TestGenerateArray(t *testing.T) {
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{
					Name: "tags",
					Type: parser.TypeExpr{
						Name:     "array",
						ElemType: &parser.TypeExpr{Name: "string"},
					},
				},
			},
		},
		nil, 42,
	)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := input["tags"].([]any)
	if !ok {
		t.Fatalf("expected []any for tags, got %T", input["tags"])
	}
	// Every element should be a string
	for i, elem := range arr {
		if _, ok := elem.(string); !ok {
			t.Errorf("tags[%d] type = %T, want string", i, elem)
		}
	}
}

func TestGenerateMap(t *testing.T) {
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{Name: "metadata", Type: parser.TypeExpr{
					Name:    "map",
					KeyType: &parser.TypeExpr{Name: "string"},
					ValType: &parser.TypeExpr{Name: "int"},
				}},
			},
		},
		nil, 42,
	)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := input["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for metadata, got %T", input["metadata"])
	}
	for k, v := range m {
		if _, ok := v.(int); !ok {
			t.Errorf("metadata[%q] type = %T, want int", k, v)
		}
	}
}

func TestLenConstraint(t *testing.T) {
	// items: []int { len(items) >= 1 }
	constraint := parser.BinaryOp{
		Left:  parser.LenExpr{Arg: parser.FieldRef{Path: "items"}},
		Op:    ">=",
		Right: parser.LiteralInt{Value: 1},
	}
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{
					Name: "items",
					Type: parser.TypeExpr{
						Name:     "array",
						ElemType: &parser.TypeExpr{Name: "int"},
					},
					Constraint: constraint,
				},
			},
		},
		nil, 42,
	)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := input["items"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", input["items"])
	}
	if len(arr) < 1 {
		t.Errorf("len constraint >= 1 violated: got length %d", len(arr))
	}
}

func TestEvalLen(t *testing.T) {
	ctx := &evalCtx{input: map[string]any{
		"items": []any{1, 2, 3},
		"name":  "hello",
		"meta":  map[string]any{"a": 1, "b": 2},
	}}

	// len(items) = 3
	val, ok := ctx.eval(parser.LenExpr{Arg: parser.FieldRef{Path: "items"}})
	if !ok || val != 3 {
		t.Errorf("len(items) = %v, want 3", val)
	}

	// len(name) = 5
	val, ok = ctx.eval(parser.LenExpr{Arg: parser.FieldRef{Path: "name"}})
	if !ok || val != 5 {
		t.Errorf("len(name) = %v, want 5", val)
	}

	// len(meta) = 2
	val, ok = ctx.eval(parser.LenExpr{Arg: parser.FieldRef{Path: "meta"}})
	if !ok || val != 2 {
		t.Errorf("len(meta) = %v, want 2", val)
	}
}

func TestEvalContains(t *testing.T) {
	ctx := &evalCtx{input: map[string]any{
		"msg":   "parse error: unexpected token",
		"items": []any{1, 2, 3},
		"empty": "",
		"arr":   []any{},
	}}

	// String substring match
	val, ok := ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "msg"},
		Needle:   parser.LiteralString{Value: "error"},
	})
	if !ok || val != true {
		t.Errorf("contains(msg, 'error') = %v, want true", val)
	}

	// String substring miss
	val, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "msg"},
		Needle:   parser.LiteralString{Value: "success"},
	})
	if !ok || val != false {
		t.Errorf("contains(msg, 'success') = %v, want false", val)
	}

	// Array element membership
	val, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "items"},
		Needle:   parser.LiteralInt{Value: 2},
	})
	if !ok || val != true {
		t.Errorf("contains(items, 2) = %v, want true", val)
	}

	// Array element miss
	val, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "items"},
		Needle:   parser.LiteralInt{Value: 99},
	})
	if !ok || val != false {
		t.Errorf("contains(items, 99) = %v, want false", val)
	}

	// Empty string contains empty string
	val, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "empty"},
		Needle:   parser.LiteralString{Value: ""},
	})
	if !ok || val != true {
		t.Errorf("contains('', '') = %v, want true", val)
	}

	// Empty array does not contain anything
	val, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "arr"},
		Needle:   parser.LiteralInt{Value: 1},
	})
	if !ok || val != false {
		t.Errorf("contains([], 1) = %v, want false", val)
	}

	// Type mismatch: string haystack with int needle
	_, ok = ctx.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "msg"},
		Needle:   parser.LiteralInt{Value: 42},
	})
	if ok {
		t.Error("contains(string, int) should return not ok")
	}

	// Type mismatch: int haystack
	ctx2 := &evalCtx{input: map[string]any{"n": 42}}
	_, ok = ctx2.eval(parser.ContainsExpr{
		Haystack: parser.FieldRef{Path: "n"},
		Needle:   parser.LiteralInt{Value: 4},
	})
	if ok {
		t.Error("contains(int, int) should return not ok")
	}
}

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
