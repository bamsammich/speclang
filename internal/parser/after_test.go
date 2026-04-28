package parser

import (
	"strings"
	"testing"
)

func TestParseAfterBlock(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(`
scope api {
  contract SomeContract -> Result {
    x: int
  }
  after {
    http.post("/teardown", {})
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
scope api {
  contract SomeContract -> Result {
    x: int
  }
  after {
    http.post("/teardown", {})
  }
  after {
    http.post("/teardown2", {})
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
scope api {
  contract SomeContract -> Result {
    after: int
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	fields := parsed.Scopes[0].Contracts[0].Fields
	found := false
	for _, f := range fields {
		if f.Name == "after" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'after' as a field name")
	}
}
