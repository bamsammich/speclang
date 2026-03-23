package validator

import (
	"fmt"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

// primitives are type names that don't need model resolution.
var primitives = map[string]bool{
	"int": true, "string": true, "bool": true,
	"float": true, "bytes": true, "array": true, "map": true,
}

// Validate performs post-parse semantic validation on a spec.
// It returns all errors found (not just the first).
func Validate(spec *parser.Spec) []error {
	v := &validator{
		models: buildModelRegistry(spec.Models),
	}

	for _, scope := range spec.Scopes {
		v.scope = scope.Name
		v.validateContract(scope)
	}

	return v.errs
}

type validator struct {
	models map[string]*parser.Model
	errs   []error
	scope  string
}

func buildModelRegistry(models []*parser.Model) map[string]*parser.Model {
	reg := make(map[string]*parser.Model, len(models))
	for _, m := range models {
		reg[m.Name] = m
	}
	return reg
}

func (v *validator) errorf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) validateContract(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	for _, f := range scope.Contract.Input {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract input %q", v.scope, f.Name))
	}
	for _, f := range scope.Contract.Output {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract output %q", v.scope, f.Name))
	}
}

func (v *validator) validateTypeExpr(te parser.TypeExpr, context string) {
	switch {
	case primitives[te.Name]:
		if te.ElemType != nil {
			v.validateTypeExpr(*te.ElemType, context)
		}
		if te.KeyType != nil {
			v.validateTypeExpr(*te.KeyType, context)
		}
		if te.ValType != nil {
			v.validateTypeExpr(*te.ValType, context)
		}
	default:
		if _, ok := v.models[te.Name]; !ok {
			v.errorf("%s: unknown type %q", context, te.Name)
		}
	}
}
