package parser

import (
	"testing"
)

func TestParseFieldRefExpr_ArrayIndex(t *testing.T) {
	t.Parallel()

	spec, err := Parse(`
model Result {
  items: []string
  error: string?
}
scope test_scope {
  contract Order -> Result {
    invariant order {
      when error == null and len(output.items) > 1:
        output.items.0 >= output.items.1
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	inv := spec.Scopes[0].Contracts[0].Invariants[0]
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
model Item { name: string }
model Result {
  results: []Item
  error: string?
}
scope test_scope {
  contract GetItems -> Result {
    invariant first_name {
      when error == null and len(output.results) > 0:
        output.results.0.name != ""
    }
  }
}
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	inv := spec.Scopes[0].Contracts[0].Invariants[0]
	if len(inv.Assertions) == 0 {
		t.Fatal("expected at least one assertion")
	}
}

func TestParseField_KeywordAsFieldName(t *testing.T) {
	t.Parallel()

	// In v4 contract bodies, keywords that are not structural (action, invariant,
	// scenario, constrain) can be used as field names.
	spec, err := Parse(`
scope test_scope {
  contract KeywordFields -> bool {
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
`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fields := spec.Scopes[0].Contracts[0].Fields
	expected := []string{"model", "input", "output", "target", "scope", "config", "given", "then"}
	if len(fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(fields))
	}
	for i, name := range expected {
		if fields[i].Name != name {
			t.Errorf("field %d: name = %q, want %q", i, fields[i].Name, name)
		}
	}
}
