package specrun

import (
	"github.com/bamsammich/speclang/v2/internal/adapter"
	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// DefaultRegistry returns a fresh Registry with the three built-in plugins
// (http, process, playwright) registered with their schemas and adapters.
func DefaultRegistry() *spec.Registry {
	r := spec.NewRegistry()
	registerHTTP(r)
	registerProcess(r)
	registerPlaywright(r)
	return r
}

func registerHTTP(r *spec.Registry) {
	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		// NewHTTPAdapter only fails on cookiejar creation which is infallible
		// with nil options. If it somehow fails, skip registration.
		return
	}
	r.Register("http", spec.PluginDef{
		Actions: map[string]spec.PluginActionDef{
			"get": {Params: []spec.Param{{Name: "path", Type: spec.TypeExpr{Name: "string"}}}},
			"post": {
				Params: []spec.Param{
					{Name: "path", Type: spec.TypeExpr{Name: "string"}},
					{Name: "body", Type: spec.TypeExpr{Name: "any"}},
				},
			},
			"put": {
				Params: []spec.Param{
					{Name: "path", Type: spec.TypeExpr{Name: "string"}},
					{Name: "body", Type: spec.TypeExpr{Name: "any"}},
				},
			},
			"delete": {Params: []spec.Param{{Name: "path", Type: spec.TypeExpr{Name: "string"}}}},
			"header": {
				Params: []spec.Param{
					{Name: "name", Type: spec.TypeExpr{Name: "string"}},
					{Name: "value", Type: spec.TypeExpr{Name: "string"}},
				},
			},
		},
		Assertions: map[string]spec.AssertionDef{
			"status": {Type: "int"},
			"body":   {Type: "any"},
			"header": {Type: "string"},
		},
		Adapter: adp,
	})
}

func registerProcess(r *spec.Registry) {
	adp := adapter.NewProcessAdapter()
	r.Register("process", spec.PluginDef{
		Actions: map[string]spec.PluginActionDef{
			"exec": {Params: []spec.Param{{Name: "args", Type: spec.TypeExpr{Name: "string"}}}},
		},
		Assertions: map[string]spec.AssertionDef{
			"exit_code": {Type: "int"},
			"stdout":    {Type: "any"},
			"stderr":    {Type: "string"},
		},
		Adapter: adp,
	})
}

func registerPlaywright(r *spec.Registry) {
	adp := adapter.NewPlaywrightAdapter()
	r.Register("playwright", spec.PluginDef{
		Actions: map[string]spec.PluginActionDef{
			"goto": {
				Params: []spec.Param{{Name: "url", Type: spec.TypeExpr{Name: "string"}}},
			},
			"click": {
				Params: []spec.Param{{Name: "selector", Type: spec.TypeExpr{Name: "string"}}},
			},
			"fill": {
				Params: []spec.Param{
					{Name: "selector", Type: spec.TypeExpr{Name: "string"}},
					{Name: "value", Type: spec.TypeExpr{Name: "string"}},
				},
			},
			"type": {
				Params: []spec.Param{
					{Name: "selector", Type: spec.TypeExpr{Name: "string"}},
					{Name: "value", Type: spec.TypeExpr{Name: "string"}},
				},
			},
			"select": {
				Params: []spec.Param{
					{Name: "selector", Type: spec.TypeExpr{Name: "string"}},
					{Name: "value", Type: spec.TypeExpr{Name: "string"}},
				},
			},
			"check": {
				Params: []spec.Param{{Name: "selector", Type: spec.TypeExpr{Name: "string"}}},
			},
			"uncheck": {
				Params: []spec.Param{{Name: "selector", Type: spec.TypeExpr{Name: "string"}}},
			},
			"wait": {
				Params: []spec.Param{{Name: "selector", Type: spec.TypeExpr{Name: "string"}}},
			},
			"resize": {
				Params: []spec.Param{
					{Name: "width", Type: spec.TypeExpr{Name: "int"}},
					{Name: "height", Type: spec.TypeExpr{Name: "int"}},
				},
			},
			"new_page":    {Params: nil},
			"close_page":  {Params: nil},
			"clear_state": {Params: nil},
		},
		Assertions: map[string]spec.AssertionDef{
			"visible":  {Type: "bool"},
			"text":     {Type: "string"},
			"value":    {Type: "string"},
			"checked":  {Type: "bool"},
			"disabled": {Type: "bool"},
			"count":    {Type: "int"},
		},
		Adapter: adp,
	})
}
