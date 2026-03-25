package specrun

import (
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()

	plugins := reg.Plugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d: %v", len(plugins), plugins)
	}

	expected := []string{"http", "playwright", "process"}
	for i, name := range expected {
		if plugins[i] != name {
			t.Errorf("plugin[%d] = %q, want %q", i, plugins[i], name)
		}
	}

	// Verify each plugin has expected actions and assertions.
	tests := []struct {
		name       string
		actions    []string
		assertions []string
	}{
		{
			name:       "http",
			actions:    []string{"get", "post", "put", "delete", "header"},
			assertions: []string{"status", "body", "header"},
		},
		{
			name:       "process",
			actions:    []string{"exec"},
			assertions: []string{"exit_code", "stdout", "stderr"},
		},
		{
			name: "playwright",
			actions: []string{
				"goto",
				"click",
				"fill",
				"type",
				"select",
				"check",
				"uncheck",
				"wait",
				"resize",
				"new_page",
				"close_page",
				"clear_state",
			},
			assertions: []string{"visible", "text", "value", "checked", "disabled", "count"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, ok := reg.Plugin(tt.name)
			if !ok {
				t.Fatalf("plugin %q not registered", tt.name)
			}

			for _, action := range tt.actions {
				if _, ok := def.Actions[action]; !ok {
					t.Errorf("missing action %q in %s", action, tt.name)
				}
			}
			if len(def.Actions) != len(tt.actions) {
				t.Errorf("%s: expected %d actions, got %d",
					tt.name, len(tt.actions), len(def.Actions))
			}

			for _, assertion := range tt.assertions {
				if _, ok := def.Assertions[assertion]; !ok {
					t.Errorf("missing assertion %q in %s", assertion, tt.name)
				}
			}
			if len(def.Assertions) != len(tt.assertions) {
				t.Errorf("%s: expected %d assertions, got %d",
					tt.name, len(tt.assertions), len(def.Assertions))
			}

			if def.Adapter == nil {
				t.Errorf("%s: adapter is nil", tt.name)
			}
		})
	}
}
