package spec

import (
	"fmt"
	"sort"
)

// PluginDef describes a registered plugin's schema and adapter.
type PluginDef struct {
	Actions    map[string]ActionDef
	Assertions map[string]AssertionDef
	Adapter    Adapter
}

// ActionDef describes a plugin action's parameter schema.
type ActionDef struct {
	Params []Param
}

// AssertionDef describes a plugin assertion's expected type.
type AssertionDef struct {
	Type string
}

// Registry holds registered plugins and their adapters.
type Registry struct {
	plugins map[string]PluginDef
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]PluginDef)}
}

// Register adds or replaces a plugin definition.
func (r *Registry) Register(name string, def PluginDef) {
	r.plugins[name] = def
}

// Adapter returns the adapter for a registered plugin.
//
//nolint:ireturn // Adapter is the canonical interface for plugin adapters.
func (r *Registry) Adapter(name string) (Adapter, error) {
	def, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not registered", name)
	}
	return def.Adapter, nil
}

// Plugin returns the definition for a registered plugin.
func (r *Registry) Plugin(name string) (PluginDef, bool) {
	def, ok := r.plugins[name]
	return def, ok
}

// Plugins returns the sorted names of all registered plugins.
func (r *Registry) Plugins() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
