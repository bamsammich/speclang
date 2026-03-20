package generator

import (
	"github.com/bamsammich/speclang/pkg/parser"
)

// Shrink attempts to find a minimal counterexample by shrinking each field
// in the input. The stillFails predicate re-executes against the real system
// to confirm the shrunk input still triggers the failure.
func Shrink(
	input map[string]any,
	fields []*parser.Field,
	models map[string]*parser.Model,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	for _, f := range fields {
		current = shrinkField(current, f, models, stillFails)
	}
	return current
}

func shrinkField(
	input map[string]any,
	field *parser.Field,
	models map[string]*parser.Model,
	stillFails func(map[string]any) bool,
) map[string]any {
	val, ok := input[field.Name]
	if !ok || val == nil {
		return input
	}

	if m, ok := models[field.Type.Name]; ok {
		return shrinkModel(input, field.Name, m, models, stillFails)
	}

	switch val.(type) {
	case int:
		return shrinkInt(input, field.Name, stillFails)
	case string:
		return shrinkString(input, field.Name, stillFails)
	default:
		return input
	}
}

// shrinkInt binary searches toward 0.
func shrinkInt(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	val, ok := current[name].(int)
	if !ok {
		return current
	}

	lo := 0
	hi := val
	if val < 0 {
		lo = val
		hi = 0
	}

	for lo < hi {
		mid := lo + (hi-lo)/2
		candidate := copyMap(current)
		candidate[name] = mid
		if stillFails(candidate) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}

	candidate := copyMap(current)
	candidate[name] = lo
	if stillFails(candidate) {
		current[name] = lo
	}
	return current
}

// shrinkString binary searches on length (shorter prefixes).
func shrinkString(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	val, ok := current[name].(string)
	if !ok {
		return current
	}

	lo := 0
	hi := len(val)

	for lo < hi {
		mid := lo + (hi-lo)/2
		candidate := copyMap(current)
		candidate[name] = val[:mid]
		if stillFails(candidate) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}

	candidate := copyMap(current)
	candidate[name] = val[:lo]
	if stillFails(candidate) {
		current[name] = val[:lo]
	}
	return current
}

// shrinkModel recurses into each field of a nested model.
func shrinkModel(
	input map[string]any,
	name string,
	model *parser.Model,
	models map[string]*parser.Model,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	nested, ok := current[name].(map[string]any)
	if !ok {
		return current
	}

	nested = copyMap(nested)
	current[name] = nested

	for _, f := range model.Fields {
		// Wrap stillFails to operate on the nested map but check via the full input.
		nested = Shrink(nested, []*parser.Field{f}, models, func(candidate map[string]any) bool {
			outer := copyMap(current)
			outer[name] = candidate
			return stillFails(outer)
		})
	}

	current[name] = nested
	return current
}

func copyMap(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
