package parser

import (
	"strings"
	"testing"
)

func TestParseAfterBlock(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(`
spec Test {
  scope api {
    contract {
      input { x: int }
      output { y: int }
    }
    after {
      http.post("/teardown", {})
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
	scope := parsed.Scopes[0]
	if scope.After == nil {
		t.Fatal("expected After block to be set")
	}
	if len(scope.After.Steps) != 1 {
		t.Fatalf("expected 1 after step, got %d", len(scope.After.Steps))
	}
	if _, ok := scope.After.Steps[0].(*AdapterCall); !ok {
		t.Fatalf("expected step to be *AdapterCall, got %T", scope.After.Steps[0])
	}
}

func TestParseAfterBlock_DuplicateRejected(t *testing.T) {
	t.Parallel()
	_, err := Parse(`
spec Test {
  scope api {
    contract {
      input { x: int }
      output { y: int }
    }
    after {
      http.post("/teardown", {})
    }
    after {
      http.post("/teardown2", {})
    }
  }
}
`)
	if err == nil {
		t.Fatal("expected error for duplicate after blocks")
	}
	if !strings.Contains(err.Error(), "multiple 'after' blocks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAfterBlock_AsFieldName(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(`
spec Test {
  scope api {
    contract {
      input { x: int }
      output { after: int }
    }
    scenario smoke {
      given { x: 1 }
      then { after == 42 }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	out := parsed.Scopes[0].Contract.Output
	found := false
	for _, f := range out {
		if f.Name == "after" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'after' as an output field name")
	}
}
