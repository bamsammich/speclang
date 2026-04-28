package parser

import (
	"strings"
	"testing"
)

func TestParseBeforeBlock(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
scope api {
  before {
    http.post("/setup", {})
    http.header("X-Test", "true")
  }
  contract SomeContract -> Result {
    x: int
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
scope api {
  before {
    http.post("/setup", {})
  }
  before {
    http.post("/setup2", {})
  }
  contract SomeContract -> Result {
    x: int
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
scope api {
  contract SomeContract -> Result {
    before: string
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	fields := spec.Scopes[0].Contracts[0].Fields
	found := false
	for _, f := range fields {
		if f.Name == "before" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'before' as a field name")
	}
}
