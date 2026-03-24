package parser_test

import (
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestParseAllExpr(t *testing.T) {
	t.Parallel()

	src := `spec T {
  scope s {
    use http
    config { path: "/x" method: "GET" }
    contract {
      input { items: []int }
      output { ok: bool }
    }
    invariant all_positive {
      all(input.items, x => x > 0)
    }
  }
}`
	spec, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	if inv.Name != "all_positive" {
		t.Fatalf("expected invariant name 'all_positive', got %q", inv.Name)
	}

	a := inv.Assertions[0]
	allExpr, ok := a.Expr.(parser.AllExpr)
	if !ok {
		t.Fatalf("expected AllExpr, got %T", a.Expr)
	}
	if allExpr.BoundVar != "x" {
		t.Errorf("expected bound var 'x', got %q", allExpr.BoundVar)
	}

	ref, ok := allExpr.Array.(parser.FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef for array, got %T", allExpr.Array)
	}
	if ref.Path != "input.items" {
		t.Errorf("expected array path 'input.items', got %q", ref.Path)
	}

	binOp, ok := allExpr.Predicate.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp for predicate, got %T", allExpr.Predicate)
	}
	if binOp.Op != ">" {
		t.Errorf("expected op '>', got %q", binOp.Op)
	}
}

func TestParseAnyExpr(t *testing.T) {
	t.Parallel()

	src := `spec T {
  scope s {
    use http
    config { path: "/x" method: "GET" }
    contract {
      input { items: []string }
      output { ok: bool }
    }
    invariant has_nonempty {
      any(input.items, s => len(s) > 0)
    }
  }
}`
	spec, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	anyExpr, ok := inv.Assertions[0].Expr.(parser.AnyExpr)
	if !ok {
		t.Fatalf("expected AnyExpr, got %T", inv.Assertions[0].Expr)
	}
	if anyExpr.BoundVar != "s" {
		t.Errorf("expected bound var 's', got %q", anyExpr.BoundVar)
	}
}

func TestParseNestedQuantifiers(t *testing.T) {
	t.Parallel()

	src := `spec T {
  scope s {
    use http
    config { path: "/x" method: "GET" }
    contract {
      input { matrix: []any }
      output { ok: bool }
    }
    invariant all_rows_have_positive {
      all(input.matrix, row => any(row, x => x > 0))
    }
  }
}`
	spec, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	allExpr, ok := inv.Assertions[0].Expr.(parser.AllExpr)
	if !ok {
		t.Fatalf("expected AllExpr, got %T", inv.Assertions[0].Expr)
	}
	if allExpr.BoundVar != "row" {
		t.Errorf("expected bound var 'row', got %q", allExpr.BoundVar)
	}

	// Predicate should be AnyExpr
	anyExpr, ok := allExpr.Predicate.(parser.AnyExpr)
	if !ok {
		t.Fatalf("expected nested AnyExpr, got %T", allExpr.Predicate)
	}
	if anyExpr.BoundVar != "x" {
		t.Errorf("expected inner bound var 'x', got %q", anyExpr.BoundVar)
	}
}

func TestParseQuantifierWithComplexPredicate(t *testing.T) {
	t.Parallel()

	src := `spec T {
  scope s {
    use http
    config { path: "/x" method: "GET" }
    contract {
      input { nums: []int }
      output { ok: bool }
    }
    invariant bounded {
      all(input.nums, n => n >= 0 && n <= 100)
    }
  }
}`
	spec, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	allExpr, ok := inv.Assertions[0].Expr.(parser.AllExpr)
	if !ok {
		t.Fatalf("expected AllExpr, got %T", inv.Assertions[0].Expr)
	}

	// Predicate should be a && BinaryOp
	binOp, ok := allExpr.Predicate.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp for predicate, got %T", allExpr.Predicate)
	}
	if binOp.Op != "&&" {
		t.Errorf("expected op '&&', got %q", binOp.Op)
	}
}
