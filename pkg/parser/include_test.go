package parser

import (
	"path/filepath"
	"testing"
)

func TestResolveIncludes_Basic(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "include", "basic", "root.spec")
	tokens, err := lexFile(root)
	if err != nil {
		t.Fatalf("lexing root: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveIncludes(tokens, filepath.Dir(absRoot), absRoot, nil)
	if err != nil {
		t.Fatalf("resolveIncludes: %v", err)
	}

	// Should have no TokenInclude remaining
	for _, tok := range resolved {
		if tok.Type == TokenInclude {
			t.Fatal("resolved tokens still contain TokenInclude")
		}
	}

	// Should end with exactly one EOF
	if resolved[len(resolved)-1].Type != TokenEOF {
		t.Fatal("expected EOF as last token")
	}

	// Count EOFs — should be exactly one
	eofCount := 0
	for _, tok := range resolved {
		if tok.Type == TokenEOF {
			eofCount++
		}
	}
	if eofCount != 1 {
		t.Fatalf("expected exactly 1 EOF, got %d", eofCount)
	}

	// Should contain tokens from included files
	hasModel := false
	hasScope := false
	for _, tok := range resolved {
		if tok.Type == TokenModel {
			hasModel = true
		}
		if tok.Type == TokenScope {
			hasScope = true
		}
	}
	if !hasModel {
		t.Error("expected model token from included models.spec")
	}
	if !hasScope {
		t.Error("expected scope token from included scopes.spec")
	}
}
