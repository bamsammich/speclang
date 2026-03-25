package parser

import (
	"testing"
)

func TestParseEnumType(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        status: enum("active", "inactive", "pending")
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

	field := spec.Scopes[0].Contract.Input[0]
	if field.Type.Name != "enum" {
		t.Errorf("type name = %q, want 'enum'", field.Type.Name)
	}
	if len(field.Type.Variants) != 3 {
		t.Fatalf("variants count = %d, want 3", len(field.Type.Variants))
	}
	want := []string{"active", "inactive", "pending"}
	for i, v := range want {
		if field.Type.Variants[i] != v {
			t.Errorf("variant[%d] = %q, want %q", i, field.Type.Variants[i], v)
		}
	}
}

func TestParseEnumType_SingleVariant(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        mode: enum("default")
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

	field := spec.Scopes[0].Contract.Input[0]
	if field.Type.Name != "enum" {
		t.Errorf("type name = %q, want 'enum'", field.Type.Name)
	}
	if len(field.Type.Variants) != 1 {
		t.Fatalf("variants count = %d, want 1", len(field.Type.Variants))
	}
	if field.Type.Variants[0] != "default" {
		t.Errorf("variant[0] = %q, want 'default'", field.Type.Variants[0])
	}
}

func TestParseEnumType_Optional(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        role: enum("admin", "user")?
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

	field := spec.Scopes[0].Contract.Input[0]
	if field.Type.Name != "enum" {
		t.Errorf("type name = %q, want 'enum'", field.Type.Name)
	}
	if !field.Type.Optional {
		t.Error("expected optional enum")
	}
	if len(field.Type.Variants) != 2 {
		t.Fatalf("variants count = %d, want 2", len(field.Type.Variants))
	}
}

func TestParseEnumType_TrailingComma(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        color: enum("red", "green", "blue",)
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

	field := spec.Scopes[0].Contract.Input[0]
	if len(field.Type.Variants) != 3 {
		t.Fatalf("variants count = %d, want 3 (trailing comma)", len(field.Type.Variants))
	}
}

func TestParseEnumType_Empty(t *testing.T) {
	_, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        status: enum()
      }
      output {
        ok: bool
      }
    }
  }
}
`)
	if err == nil {
		t.Fatal("expected parse error for empty enum")
	}
}

func TestParseEnumType_NonStringVariant(t *testing.T) {
	_, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        status: enum(1, 2, 3)
      }
      output {
        ok: bool
      }
    }
  }
}
`)
	if err == nil {
		t.Fatal("expected parse error for non-string enum variant")
	}
}

func TestParseEnumType_InGiven(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        status: enum("active", "inactive")
      }
      output {
        ok: bool
      }
    }
    scenario smoke {
      given {
        status: "active"
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
	lit, ok := a.Value.(LiteralString)
	if !ok {
		t.Fatalf("expected LiteralString, got %T", a.Value)
	}
	if lit.Value != "active" {
		t.Errorf("value = %q, want 'active'", lit.Value)
	}
}
