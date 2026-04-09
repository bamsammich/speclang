package parser

import (
	"testing"
)

// TestParseAction_BeforeAsParamName is the exact reproducer for issue #113.
// `before` is a natural cursor-pagination parameter name but was rejected by
// parseParam's strict TokenIdent check.
func TestParseAction_BeforeAsParamName(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope session_history {
    action session_history(limit: int?, before: string?) {
      let result = http.get("/api/v1/sessions/history")
      return result
    }

    contract {
      input {
        limit: int?
        before: string?
      }
      output {
        sessions: string?
        error: string?
      }
      action: session_history
    }
  }
}
`)
	if err != nil {
		t.Fatalf("expected parse to succeed, got: %v", err)
	}
	if len(spec.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
	}
	if len(spec.Scopes[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(spec.Scopes[0].Actions))
	}
	action := spec.Scopes[0].Actions[0]
	if len(action.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(action.Params))
	}
	if action.Params[0].Name != "limit" {
		t.Errorf("expected first param 'limit', got %q", action.Params[0].Name)
	}
	if action.Params[1].Name != "before" {
		t.Errorf("expected second param 'before', got %q", action.Params[1].Name)
	}
}

// TestParseAction_MoreKeywordsAsParamNames covers keywords that are valid as
// param names via isIdentLike. `after` is a control case — it was already
// accepted before issue #113. `when` was added by #113 and is safe because it
// does not declare scope-level blocks.
//
// Note: "contract", "invariant", and "scenario" are intentionally NOT tested
// here. Those keywords declare scope-level blocks; including them in
// isIdentLike caused a regression where `scenario nested {}` inside a then
// block silently parsed as a field reference instead of a syntax error.
func TestParseAction_MoreKeywordsAsParamNames(t *testing.T) {
	t.Parallel()
	// `after` is a control: accepted before #113. `when` was added by #113.
	cases := []string{"after", "when"}
	for _, kw := range cases {
		kw := kw
		t.Run(kw, func(t *testing.T) {
			t.Parallel()
			src := `
spec Test {
  scope s {
    action foo(` + kw + `: string) {
      return ` + kw + `
    }
  }
}
`
			spec, err := Parse(src)
			if err != nil {
				t.Fatalf("expected parse to succeed for param name %q, got: %v", kw, err)
			}
			got := spec.Scopes[0].Actions[0].Params[0].Name
			if got != kw {
				t.Errorf("expected param name %q, got %q", kw, got)
			}
		})
	}
}

// TestKeywordsInNamePositions confirms block keywords can appear in every
// identifier-like name position in a spec, not just action parameters.
func TestKeywordsInNamePositions(t *testing.T) {
	t.Parallel()

	t.Run("model name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`spec T { model before { x: int } }`)
		if err != nil {
			t.Fatalf("model named 'before' should parse: %v", err)
		}
	})

	t.Run("scenario name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    contract { input { x: int } output { y: int } }
    scenario before {
      given { x: 1 }
      then { y == 1 }
    }
  }
}
`)
		if err != nil {
			t.Fatalf("scenario named 'before' should parse: %v", err)
		}
	})

	t.Run("invariant name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    contract { input { x: int } output { y: int } action: foo }
    action foo(x: int) { return x }
    invariant before {
      y == x
    }
  }
}
`)
		if err != nil {
			t.Fatalf("invariant named 'before' should parse: %v", err)
		}
	})

	t.Run("object literal key", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    scenario smoke {
      given {
        http.post("/x", { before: "2026-01-01", after: "2026-12-31" })
      }
      then { true == true }
    }
  }
}
`)
		if err != nil {
			t.Fatalf("object literal key 'before' should parse: %v", err)
		}
	})
}
