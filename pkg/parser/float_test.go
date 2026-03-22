package parser

import (
	"testing"
)

func TestLexFloat(t *testing.T) {
	tests := []struct {
		input  string
		tokens []struct {
			typ TokenType
			val string
		}
	}{
		{
			"3.14",
			[]struct {
				typ TokenType
				val string
			}{
				{TokenFloat, "3.14"},
			},
		},
		{
			"3.",
			[]struct {
				typ TokenType
				val string
			}{
				{TokenInt, "3"},
				{TokenDot, "."},
			},
		},
		{
			"3.field",
			[]struct {
				typ TokenType
				val string
			}{
				{TokenInt, "3"},
				{TokenDot, "."},
				{TokenIdent, "field"},
			},
		},
		{
			"0.5",
			[]struct {
				typ TokenType
				val string
			}{
				{TokenFloat, "0.5"},
			},
		},
		{
			"100.001",
			[]struct {
				typ TokenType
				val string
			}{
				{TokenFloat, "100.001"},
			},
		},
	}

	for _, tt := range tests {
		tokens, err := Lex(tt.input)
		if err != nil {
			t.Fatalf("Lex(%q) error: %v", tt.input, err)
		}
		// Remove EOF
		tokens = tokens[:len(tokens)-1]
		if len(tokens) != len(tt.tokens) {
			t.Errorf("Lex(%q): got %d tokens, want %d", tt.input, len(tokens), len(tt.tokens))
			continue
		}
		for i, want := range tt.tokens {
			if tokens[i].Type != want.typ || tokens[i].Value != want.val {
				t.Errorf("Lex(%q) token %d: got %s(%q), want %s(%q)",
					tt.input, i, tokens[i].Type, tokens[i].Value, want.typ, want.val)
			}
		}
	}
}

func TestParseFloatLiteral(t *testing.T) {
	spec, err := Parse(`
spec Test {
  scope test {
    use http
    contract {
      input {
        price: float { price > 0.5 }
      }
      output {
        total: float
      }
    }
    scenario smoke {
      given {
        price: 9.99
      }
      then {
        total: 19.98
      }
    }
  }
}
`)
	if err != nil {
		t.Fatal(err)
	}

	if len(spec.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
	}

	scope := spec.Scopes[0]

	// Check input field type
	if scope.Contract.Input[0].Type.Name != "float" {
		t.Errorf("input type = %q, want 'float'", scope.Contract.Input[0].Type.Name)
	}

	// Check constraint uses LiteralFloat
	constraint, ok := scope.Contract.Input[0].Constraint.(BinaryOp)
	if !ok {
		t.Fatalf("constraint type = %T, want BinaryOp", scope.Contract.Input[0].Constraint)
	}
	litFloat, ok := constraint.Right.(LiteralFloat)
	if !ok {
		t.Fatalf("constraint right = %T, want LiteralFloat", constraint.Right)
	}
	if litFloat.Value != 0.5 {
		t.Errorf("constraint right value = %v, want 0.5", litFloat.Value)
	}

	// Check given value is LiteralFloat
	given := scope.Scenarios[0].Given.Steps[0].(*Assignment)
	gv, ok := given.Value.(LiteralFloat)
	if !ok {
		t.Fatalf("given value type = %T, want LiteralFloat", given.Value)
	}
	if gv.Value != 9.99 {
		t.Errorf("given value = %v, want 9.99", gv.Value)
	}
}
