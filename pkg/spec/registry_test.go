package spec_test

import (
	"encoding/json"
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// stubAdapter is a minimal Adapter implementation for testing.
type stubAdapter struct{}

func (stubAdapter) Init(map[string]string) error { return nil }

func (stubAdapter) Action(string, json.RawMessage) (*spec.Response, error) {
	return nil, nil
}

func (stubAdapter) Assert(string, string, json.RawMessage) (*spec.Response, error) {
	return nil, nil
}

func (stubAdapter) Reset() error { return nil }
func (stubAdapter) Close() error { return nil }

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := spec.NewRegistry()
	adp := stubAdapter{}

	r.Register("http", spec.PluginDef{
		Adapter: adp,
		Actions: map[string]spec.ActionDef{
			"get": {Params: []spec.Param{{Name: "url", Type: spec.TypeExpr{Name: "string"}}}},
		},
	})

	got, ok := r.Plugin("http")
	if !ok {
		t.Fatal("expected plugin 'http' to be registered")
	}
	if got.Adapter == nil {
		t.Fatal("expected adapter to be non-nil")
	}
	if _, ok := got.Actions["get"]; !ok {
		t.Fatal("expected 'get' action to be registered")
	}
}

func TestRegistry_AdapterLookup(t *testing.T) {
	r := spec.NewRegistry()
	adp := stubAdapter{}
	r.Register("http", spec.PluginDef{Adapter: adp})

	got, err := r.Adapter("http")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected adapter to be non-nil")
	}
}

func TestRegistry_AdapterUnknownPlugin(t *testing.T) {
	r := spec.NewRegistry()

	_, err := r.Adapter("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}

	want := `plugin "nonexistent" not registered`
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestRegistry_PluginsSorted(t *testing.T) {
	r := spec.NewRegistry()
	r.Register("playwright", spec.PluginDef{})
	r.Register("http", spec.PluginDef{})
	r.Register("process", spec.PluginDef{})

	names := r.Plugins()
	if len(names) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(names))
	}

	expected := []string{"http", "playwright", "process"}
	for i, name := range names {
		if name != expected[i] {
			t.Fatalf("plugins[%d] = %q, want %q", i, name, expected[i])
		}
	}
}
