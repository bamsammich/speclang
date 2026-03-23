package parser

import (
	"testing"
)

func TestParseArrayType(t *testing.T) {
	spec, err := Parse(`
spec Test {
  model Item { name: string }
  scope test {
    use http
    contract {
      input {
        tags: []string
        items: []Item
        nested: [][]int
        optional_arr: []int?
      }
      output {
        count: int
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	scope := spec.Scopes[0]
	fields := scope.Contract.Input

	// tags: []string
	if fields[0].Type.Name != "array" {
		t.Errorf("tags type = %q, want 'array'", fields[0].Type.Name)
	}
	if fields[0].Type.ElemType == nil || fields[0].Type.ElemType.Name != "string" {
		t.Error("tags elem type should be string")
	}

	// items: []Item
	if fields[1].Type.Name != "array" || fields[1].Type.ElemType.Name != "Item" {
		t.Errorf("items should be array of Item")
	}

	// nested: [][]int
	if fields[2].Type.Name != "array" {
		t.Errorf("nested type = %q, want 'array'", fields[2].Type.Name)
	}
	inner := fields[2].Type.ElemType
	if inner == nil || inner.Name != "array" || inner.ElemType == nil || inner.ElemType.Name != "int" {
		t.Error("nested should be array of array of int")
	}

	// optional_arr: []int? — ? binds to outermost (optional array)
	if fields[3].Type.Name != "array" {
		t.Errorf("optional_arr type = %q, want 'array'", fields[3].Type.Name)
	}
	if !fields[3].Type.Optional {
		t.Error("optional_arr should be optional")
	}
	if fields[3].Type.ElemType.Optional {
		t.Error("optional_arr element should NOT be optional")
	}
}

func TestParseMapType(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        metadata: map[string, int]
        labels: map[string, string]?
      }
      output {
        ok: bool
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	fields := spec.Scopes[0].Contract.Input

	// metadata: map[string, int]
	if fields[0].Type.Name != "map" {
		t.Errorf("metadata type = %q, want 'map'", fields[0].Type.Name)
	}
	if fields[0].Type.KeyType == nil || fields[0].Type.KeyType.Name != "string" {
		t.Error("metadata key type should be string")
	}
	if fields[0].Type.ValType == nil || fields[0].Type.ValType.Name != "int" {
		t.Error("metadata val type should be int")
	}

	// labels: map[string, string]?
	if fields[1].Type.Name != "map" {
		t.Errorf("labels type = %q, want 'map'", fields[1].Type.Name)
	}
	if !fields[1].Type.Optional {
		t.Error("labels should be optional")
	}
}

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

func TestParseLenExpr(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        items: []int { len(items) > 0 }
      }
      output {
        count: int
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	constraint := spec.Scopes[0].Contract.Input[0].Constraint
	binOp, ok := constraint.(BinaryOp)
	if !ok {
		t.Fatalf("constraint type = %T, want BinaryOp", constraint)
	}
	lenExpr, ok := binOp.Left.(LenExpr)
	if !ok {
		t.Fatalf("left side type = %T, want LenExpr", binOp.Left)
	}
	fieldRef, ok := lenExpr.Arg.(FieldRef)
	if !ok {
		t.Fatalf("len arg type = %T, want FieldRef", lenExpr.Arg)
	}
	if fieldRef.Path != "items" {
		t.Errorf("len arg path = %q, want 'items'", fieldRef.Path)
	}
}
