package parser

import "testing"

func TestParseGivenBlock_Assignments(t *testing.T) {
	spec, err := Parse(`
use http
spec Test {
  scope test {
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
