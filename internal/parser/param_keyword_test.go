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

// TestParseAction_MoreKeywordsAsParamNames covers the "same is likely true for
// other scope-block keywords" list from issue #113: contract, invariant,
// scenario, when, after — in addition to before which Task 1 covers.
func TestParseAction_MoreKeywordsAsParamNames(t *testing.T) {
	t.Parallel()
	cases := []string{"after", "contract", "invariant", "scenario", "when"}
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
