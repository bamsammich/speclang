package parser

import "testing"

func TestParseThenBlock_V3ExprAssertions(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope test {
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario check_ui {
      given { x: 1 }
      then {
        playwright.visible('[data-testid="welcome"]') == true
        playwright.text('[data-testid="msg"]') == "hello"
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	assertions := spec.Scopes[0].Scenarios[0].Then.Assertions

	if len(assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(assertions))
	}

	// First: playwright.visible(...) == true
	a0 := assertions[0]
	binOp, ok := a0.Expr.(BinaryOp)
	if !ok {
		t.Fatalf("a0: expected BinaryOp, got %T", a0.Expr)
	}
	if binOp.Op != "==" {
		t.Errorf("a0: expected ==, got %q", binOp.Op)
	}
	call, ok := binOp.Left.(AdapterCall)
	if !ok {
		t.Fatalf("a0: expected AdapterCall on LHS, got %T", binOp.Left)
	}
	if call.Adapter != "playwright" || call.Method != "visible" {
		t.Errorf("a0: expected playwright.visible, got %s.%s", call.Adapter, call.Method)
	}

	// Second: playwright.text(...) == "hello"
	a1 := assertions[1]
	binOp1, ok := a1.Expr.(BinaryOp)
	if !ok {
		t.Fatalf("a1: expected BinaryOp, got %T", a1.Expr)
	}
	call1, ok := binOp1.Left.(AdapterCall)
	if !ok {
		t.Fatalf("a1: expected AdapterCall on LHS, got %T", binOp1.Left)
	}
	if call1.Method != "text" {
		t.Errorf("a1: expected method text, got %q", call1.Method)
	}
}

func TestParseThenBlock_SimpleFieldAssertion(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope test {
    contract {
      input { x: int }
      output { y: int }
    }
    scenario smoke {
      given { x: 1 }
      then { y == 2 }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Then.Assertions[0]
	binOp, ok := a.Expr.(BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp, got %T", a.Expr)
	}
	if binOp.Op != "==" {
		t.Errorf("expected ==, got %q", binOp.Op)
	}
	lhs, ok := binOp.Left.(FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef on LHS, got %T", binOp.Left)
	}
	if lhs.Path != "y" {
		t.Errorf("expected field path y, got %q", lhs.Path)
	}
}

func TestParseThenBlock_ComparisonOperators(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope test {
    contract {
      input { x: int }
      output { score: int }
    }
    scenario ops {
      given { x: 1 }
      then {
        playwright.count('[data-testid="items"]') >= 1
        playwright.count('[data-testid="items"]') > 0
        playwright.count('[data-testid="items"]') <= 100
        playwright.count('[data-testid="items"]') < 101
        score != 0
        score == 42
      }
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	assertions := spec.Scopes[0].Scenarios[0].Then.Assertions

	if len(assertions) != 6 {
		t.Fatalf("expected 6 assertions, got %d", len(assertions))
	}

	expectedOps := []string{">=", ">", "<=", "<", "!=", "=="}
	for i, expectedOp := range expectedOps {
		a := assertions[i]
		binOp, ok := a.Expr.(BinaryOp)
		if !ok {
			t.Fatalf("assertion[%d]: expected BinaryOp, got %T", i, a.Expr)
		}
		if binOp.Op != expectedOp {
			t.Errorf("assertion[%d]: expected op %q, got %q", i, expectedOp, binOp.Op)
		}
	}
}
