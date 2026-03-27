package parser

import "testing"

func TestParseThenBlock_AtSyntax(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  locators {
    welcome: [data-testid=welcome]
  }
  scope test {
    use playwright
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario check_ui {
      given { x: 1 }
      then {
        welcome@playwright.visible: true
        welcome@playwright.text: "hello"
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

	a0 := assertions[0]
	if a0.Target != "welcome" {
		t.Errorf("a0.Target = %q, want 'welcome'", a0.Target)
	}
	if a0.Plugin != "playwright" {
		t.Errorf("a0.Plugin = %q, want 'playwright'", a0.Plugin)
	}
	if a0.Property != "visible" {
		t.Errorf("a0.Property = %q, want 'visible'", a0.Property)
	}
	if _, ok := a0.Expected.(LiteralBool); !ok {
		t.Errorf("a0.Expected type = %T, want LiteralBool", a0.Expected)
	}

	a1 := assertions[1]
	if a1.Plugin != "playwright" || a1.Property != "text" {
		t.Errorf("a1: Plugin=%q Property=%q, want playwright.text", a1.Plugin, a1.Property)
	}
}

func TestParseThenBlock_PathAssertion_Unchanged(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input { x: int }
      output { y: int }
    }
    scenario smoke {
      given { x: 1 }
      then { y: 2 }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	a := spec.Scopes[0].Scenarios[0].Then.Assertions[0]
	if a.Target != "y" || a.Plugin != "" || a.Property != "" {
		t.Errorf(
			"path assertion should have empty Plugin/Property, got Plugin=%q Property=%q",
			a.Plugin,
			a.Property,
		)
	}
	if a.Operator != "==" {
		t.Errorf("colon assertion Operator = %q, want '=='", a.Operator)
	}
}

func TestParseThenBlock_ComparisonOperators(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  locators {
    items: [data-testid=items]
  }
  scope test {
    use playwright
    contract {
      input { x: int }
      output { score: int }
    }
    scenario ops {
      given { x: 1 }
      then {
        items@playwright.count >= 1
        items@playwright.count > 0
        items@playwright.count <= 100
        items@playwright.count < 101
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

	tests := []struct {
		target   string
		plugin   string
		property string
		operator string
	}{
		{"items", "playwright", "count", ">="},
		{"items", "playwright", "count", ">"},
		{"items", "playwright", "count", "<="},
		{"items", "playwright", "count", "<"},
		{"score", "", "", "!="},
		{"score", "", "", "=="},
	}

	for i, tt := range tests {
		a := assertions[i]
		if a.Target != tt.target {
			t.Errorf("assertion[%d] Target = %q, want %q", i, a.Target, tt.target)
		}
		if a.Plugin != tt.plugin {
			t.Errorf("assertion[%d] Plugin = %q, want %q", i, a.Plugin, tt.plugin)
		}
		if a.Property != tt.property {
			t.Errorf("assertion[%d] Property = %q, want %q", i, a.Property, tt.property)
		}
		if a.Operator != tt.operator {
			t.Errorf("assertion[%d] Operator = %q, want %q", i, a.Operator, tt.operator)
		}
	}
}
