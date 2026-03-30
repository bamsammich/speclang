package parser

import (
	"strings"
	"testing"
)

func TestParseBeforeBlock(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope api {
    before {
      http.post("/setup", {})
      http.header("X-Test", "true")
    }
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
	scope := spec.Scopes[0]
	if scope.Before == nil {
		t.Fatal("expected Before block to be set")
	}
	if len(scope.Before.Steps) != 2 {
		t.Fatalf("expected 2 before steps, got %d", len(scope.Before.Steps))
	}
}

func TestParseBeforeBlock_DuplicateRejected(t *testing.T) {
	t.Parallel()
	_, err := Parse(`
spec Test {
  scope api {
    before {
      http.post("/setup", {})
    }
    before {
      http.post("/setup2", {})
    }
    contract {
      input { x: int }
      output { y: int }
    }
  }
}
`)
	if err == nil {
		t.Fatal("expected error for duplicate before blocks")
	}
	if !strings.Contains(err.Error(), "multiple 'before' blocks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBeforeBlock_AsFieldName(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope api {
    contract {
      input { x: int }
      output { before: string }
    }
    scenario smoke {
      given { x: 1 }
      then { before == "ok" }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	out := spec.Scopes[0].Contract.Output
	found := false
	for _, f := range out {
		if f.Name == "before" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'before' as an output field name")
	}
}
