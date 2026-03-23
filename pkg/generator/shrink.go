package generator

import (
	"encoding/base64"

	"github.com/bamsammich/speclang/v2/pkg/parser"
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

	// Dispatch on TypeExpr.Name first to disambiguate types that share
	// the same Go runtime type (e.g., bytes and string are both string).
	switch field.Type.Name {
	case "float":
		return shrinkFloat(input, field.Name, stillFails)
	case "bytes":
		return shrinkBytes(input, field.Name, stillFails)
	case "array":
		return shrinkArray(input, field.Name, stillFails)
	case "map":
		return shrinkMap(input, field.Name, stillFails)
	}

	// Model lookup for named model types.
	if m, ok := models[field.Type.Name]; ok {
		return shrinkModel(input, field.Name, m, models, stillFails)
	}

	// Fall back to runtime type dispatch for legacy types.
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

// shrinkFloat binary searches toward 0.0.
func shrinkFloat(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	val, ok := current[name].(float64)
	if !ok {
		return current
	}

	lo := 0.0
	hi := val
	if val < 0 {
		lo = val
		hi = 0.0
	}

	const epsilon = 0.001
	for hi-lo > epsilon {
		mid := lo + (hi-lo)/2.0
		candidate := copyMap(current)
		candidate[name] = mid
		if stillFails(candidate) {
			hi = mid
		} else {
			lo = mid + epsilon
		}
	}

	candidate := copyMap(current)
	candidate[name] = lo
	if stillFails(candidate) {
		current[name] = lo
	}
	return current
}

// shrinkBytes decodes base64, binary searches on byte length, re-encodes.
func shrinkBytes(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	encoded, ok := current[name].(string)
	if !ok {
		return current
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return current
	}

	lo := 0
	hi := len(raw)

	for lo < hi {
		mid := lo + (hi-lo)/2
		candidate := copyMap(current)
		candidate[name] = base64.StdEncoding.EncodeToString(raw[:mid])
		if stillFails(candidate) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}

	candidate := copyMap(current)
	candidate[name] = base64.StdEncoding.EncodeToString(raw[:lo])
	if stillFails(candidate) {
		current[name] = base64.StdEncoding.EncodeToString(raw[:lo])
	}
	return current
}

// shrinkArray binary searches on length, then shrinks remaining elements.
func shrinkArray(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	arr, ok := current[name].([]any)
	if !ok || len(arr) == 0 {
		return current
	}

	// Binary search on length (remove from end).
	lo := 0
	hi := len(arr)
	for lo < hi {
		mid := lo + (hi-lo)/2
		candidate := copyMap(current)
		candidate[name] = copySlice(arr[:mid])
		if stillFails(candidate) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}

	candidate := copyMap(current)
	candidate[name] = copySlice(arr[:lo])
	if stillFails(candidate) {
		current[name] = copySlice(arr[:lo])
	}
	return current
}

// shrinkMap tries removing each key, keeping removals that preserve failure.
func shrinkMap(
	input map[string]any,
	name string,
	stillFails func(map[string]any) bool,
) map[string]any {
	current := copyMap(input)
	m, ok := current[name].(map[string]any)
	if !ok || len(m) == 0 {
		return current
	}

	// Try removing each key.
	for key := range m {
		candidate := copyMap(current)
		reduced := copyMap(m)
		delete(reduced, key)
		candidate[name] = reduced
		if stillFails(candidate) {
			current = candidate
			m = reduced
		}
	}
	return current
}

func copySlice(s []any) []any {
	c := make([]any, len(s))
	copy(c, s)
	return c
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
