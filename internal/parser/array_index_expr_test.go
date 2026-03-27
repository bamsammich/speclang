package parser

import (
	"testing"
)

func TestParseFieldRefExpr_ArrayIndex(t *testing.T) {
	t.Parallel()

	spec, err := Parse(`
spec Test {
  scope test_scope {
    use http
    config {
      path: "/test"
      method: "GET"
    }
    contract {
      input {}
      output {
        items: []string
        error: string?
      }
    }
    invariant order {
      when error == null && len(output.items) > 1:
        output.items.0 >= output.items.1
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	if inv.Name != "order" {
		t.Errorf("invariant name = %q, want %q", inv.Name, "order")
	}

	if len(inv.Assertions) == 0 {
		t.Fatal("expected at least one assertion in invariant")
	}
}

func TestParseFieldRefExpr_NestedArrayIndex(t *testing.T) {
	t.Parallel()

	spec, err := Parse(`
spec Test {
  model Item { name: string }
  scope test_scope {
    use http
    config {
      path: "/test"
      method: "GET"
    }
    contract {
      input {}
      output {
        results: []Item
        error: string?
      }
    }
    invariant first_name {
      when error == null && len(output.results) > 0:
        output.results.0.name != ""
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	if len(inv.Assertions) == 0 {
		t.Fatal("expected at least one assertion")
	}
}

func TestParseField_KeywordAsFieldName(t *testing.T) {
	t.Parallel()

	spec, err := Parse(`
spec Test {
  scope test_scope {
    use http
    config {
      path: "/test"
      method: "GET"
    }
    contract {
      input {}
      output {
        action: string
        model: string
        input: int
        output: int
        target: string
        scope: string
        config: string
        given: bool
        then: bool
      }
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fields := spec.Scopes[0].Contract.Output
	expected := []string{"action", "model", "input", "output", "target", "scope", "config", "given", "then"}
	if len(fields) != len(expected) {
		t.Fatalf("expected %d output fields, got %d", len(expected), len(fields))
	}
	for i, name := range expected {
		if fields[i].Name != name {
			t.Errorf("field %d: name = %q, want %q", i, fields[i].Name, name)
		}
	}
}
