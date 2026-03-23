package plugin

import (
	"fmt"

	"github.com/bamsammich/speclang/v2/pkg/adapter"
)

// Registry maps plugin names to their adapters.
type Registry struct {
	adapters map[string]adapter.Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]adapter.Adapter)}
}

// Register adds a built-in adapter.
func (r *Registry) Register(name string, a adapter.Adapter) {
	r.adapters[name] = a
}

// Get returns the adapter for a plugin name.
func (r *Registry) Get(name string) (adapter.Adapter, error) {
	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not registered — is the adapter binary on PATH?", name)
	}
	return a, nil
}

// TODO: Load external adapter binaries via subprocess + JSON IPC
