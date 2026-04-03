package v2parser

import "testing"

func TestParseGivenBlock_Assignments(t *testing.T) {
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
      given {
        x: 42
      }
      then {
        y: 84
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	sc := spec.Scopes[0].Scenarios[0]
	if len(sc.Given.Steps) != 1 {
		t.Fatalf("expected 1 given step, got %d", len(sc.Given.Steps))
	}
	a, ok := sc.Given.Steps[0].(*Assignment)
	if !ok {
		t.Fatalf("expected *Assignment, got %T", sc.Given.Steps[0])
	}
	if a.Path != "x" {
		t.Errorf("path = %q, want 'x'", a.Path)
	}
}

func TestParseGivenBlock_ActionCalls(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope test {
    use playwright
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario ui_flow {
      given {
        playwright.fill(username, "alice")
        x: 42
        playwright.click(submit)
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
	if len(sc.Given.Steps) != 3 {
		t.Fatalf("expected 3 given steps, got %d", len(sc.Given.Steps))
	}

	// Step 0: playwright.fill(username, "alice")
	c0, ok := sc.Given.Steps[0].(*Call)
	if !ok {
		t.Fatalf("step 0: expected *Call, got %T", sc.Given.Steps[0])
	}
	if c0.Namespace != "playwright" || c0.Method != "fill" {
		t.Errorf("step 0: got %s.%s, want playwright.fill", c0.Namespace, c0.Method)
	}

	// Step 1: x: 42 (assignment)
	_, ok = sc.Given.Steps[1].(*Assignment)
	if !ok {
		t.Fatalf("step 1: expected *Assignment, got %T", sc.Given.Steps[1])
	}

	// Step 2: playwright.click(submit)
	c2, ok := sc.Given.Steps[2].(*Call)
	if !ok {
		t.Fatalf("step 2: expected *Call, got %T", sc.Given.Steps[2])
	}
	if c2.Method != "click" {
		t.Errorf("step 2: method = %q, want 'click'", c2.Method)
	}
}
