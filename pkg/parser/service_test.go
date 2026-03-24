package parser

import "testing"

func TestParseServiceRef(t *testing.T) {
	t.Parallel()
	src := `spec Test {
  target {
    base_url: service(app)
  }
}`
	spec, err := Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ref, ok := spec.Target.Fields["base_url"].(ServiceRef)
	if !ok {
		t.Fatalf("expected ServiceRef, got %T", spec.Target.Fields["base_url"])
	}
	if ref.Name != "app" {
		t.Fatalf("expected name=app, got %q", ref.Name)
	}
}
