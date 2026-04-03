package v2parser

import (
	"testing"
)

func TestParseExistsExpr(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { name: string }
      output { status: string }
    }
    invariant field_exists {
      exists(output.status)
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	inv := spec.Scopes[0].Invariants[0]
	if inv.Name != "field_exists" {
		t.Fatalf("name = %q, want field_exists", inv.Name)
	}
	expr, ok := inv.Assertions[0].Expr.(ExistsExpr)
	if !ok {
		t.Fatalf("expected ExistsExpr, got %T", inv.Assertions[0].Expr)
	}
	ref, ok := expr.Arg.(FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef arg, got %T", expr.Arg)
	}
	if ref.Path != "output.status" {
		t.Fatalf("path = %q, want output.status", ref.Path)
	}
}

func TestParseHasKeyExpr(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { name: string }
      output { status: string }
    }
    invariant key_check {
      has_key(output, "status")
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	inv := spec.Scopes[0].Invariants[0]
	expr, ok := inv.Assertions[0].Expr.(HasKeyExpr)
	if !ok {
		t.Fatalf("expected HasKeyExpr, got %T", inv.Assertions[0].Expr)
	}
	ref, ok := expr.Arg.(FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef arg, got %T", expr.Arg)
	}
	if ref.Path != "output" {
		t.Fatalf("path = %q, want output", ref.Path)
	}
	key, ok := expr.Key.(LiteralString)
	if !ok {
		t.Fatalf("expected LiteralString key, got %T", expr.Key)
	}
	if key.Value != "status" {
		t.Fatalf("key = %q, want status", key.Value)
	}
}

func TestParseExistsInThen(t *testing.T) {
	// exists() can be used as an expected value in then blocks
	spec, err := Parse(`
spec Test {
  scope test {
    use process
    contract {
      input { file: string }
      output { exit_code: int }
    }
    scenario check_field {
      given {
        file: "test.spec"
      }
      then {
        exit_code: 0
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = spec
}
