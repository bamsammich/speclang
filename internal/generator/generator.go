package generator

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/rand/v2" //nolint:gosec // intentional use of math/rand for reproducible test generation
	"os"
	"reflect"
	"strings"

	"github.com/bamsammich/speclang/v2/internal/parser"
)

// Generator produces random valid inputs from a contract and model definitions.
type Generator struct {
	contract *parser.Contract
	models   map[string]*parser.Model
	seed     uint64
	seqN     uint64
}

// New creates a generator for the given contract and models with a reproducible seed.
func New(contract *parser.Contract, models []*parser.Model, seed uint64) *Generator {
	modelMap := make(map[string]*parser.Model, len(models))
	for _, m := range models {
		modelMap[m.Name] = m
	}
	return &Generator{contract: contract, models: modelMap, seed: seed}
}

// GenerateInput produces one random valid input satisfying the contract's constraints.
// It uses rejection sampling: generate unconstrained values, then check constraints,
// retrying until valid. For the typical transfer spec this converges quickly.
func (g *Generator) GenerateInput() (map[string]any, error) {
	if g.contract == nil {
		return nil, errors.New("no contract")
	}

	rng := rand.New(rand.NewPCG(g.seed, g.seqN)) //nolint:gosec // reproducible seeds
	g.seqN++

	const maxAttempts = 1000
	for range maxAttempts {
		input := g.generateFields(rng, g.contract.Input)
		if checkConstraints(input, g.contract.Input) {
			return input, nil
		}
	}

	return nil, fmt.Errorf("failed to generate valid input after %d attempts", maxAttempts)
}

// GenerateN produces n random valid inputs.
func (g *Generator) GenerateN(n int) ([]map[string]any, error) {
	results := make([]map[string]any, 0, n)
	for range n {
		input, err := g.GenerateInput()
		if err != nil {
			return nil, err
		}
		results = append(results, input)
	}
	return results, nil
}

// GenerateMatching generates an input satisfying both contract constraints
// and the given predicate. Uses rejection sampling.
func (g *Generator) GenerateMatching(match func(map[string]any) bool) (map[string]any, error) {
	if g.contract == nil {
		return nil, errors.New("no contract")
	}

	rng := rand.New(rand.NewPCG(g.seed, g.seqN)) //nolint:gosec // reproducible seeds
	g.seqN++

	const maxAttempts = 10000
	for range maxAttempts {
		input := g.generateFields(rng, g.contract.Input)
		if match(input) {
			return input, nil
		}
	}

	return nil, fmt.Errorf("failed to generate matching input after %d attempts", maxAttempts)
}

// ContractInput returns the contract's input fields for use by the shrinking engine.
func (g *Generator) ContractInput() []*parser.Field {
	if g.contract == nil {
		return nil
	}
	return g.contract.Input
}

// Eval evaluates an expression against the given variable context.
func Eval(expr parser.Expr, vars map[string]any) (any, bool) {
	ctx := &evalCtx{input: vars}
	return ctx.eval(expr)
}

func (g *Generator) generateFields(rng *rand.Rand, fields []*parser.Field) map[string]any {
	result := make(map[string]any, len(fields))
	for _, f := range fields {
		result[f.Name] = g.generateValue(rng, f.Type)
	}
	return result
}

func (g *Generator) generateValue(rng *rand.Rand, t parser.TypeExpr) any {
	if t.Optional && rng.IntN(4) == 0 {
		return nil
	}

	if m, ok := g.models[t.Name]; ok {
		return g.generateFields(rng, m.Fields)
	}

	return g.generatePrimitive(rng, t)
}

func (g *Generator) generatePrimitive(rng *rand.Rand, t parser.TypeExpr) any {
	switch t.Name {
	case "int":
		return generateInt(rng)
	case "float":
		return generateFloat(rng)
	case "bytes":
		return generateBytes(rng)
	case "string":
		return generateString(rng)
	case "bool":
		return rng.IntN(2) == 1
	case "array":
		if t.ElemType != nil {
			return g.generateArray(rng, *t.ElemType)
		}
		return nil
	case "map":
		if t.KeyType != nil && t.ValType != nil {
			return g.generateMap(rng, *t.ValType)
		}
		return nil
	case "enum":
		if len(t.Variants) > 0 {
			return t.Variants[rng.IntN(len(t.Variants))]
		}
		return nil
	default:
		return nil
	}
}

// generateInt produces ints biased toward boundaries: [0, 1000] with extra
// weight on 0, 1, and the upper range.
func generateInt(rng *rand.Rand) int {
	// 20% chance of boundary values
	if rng.IntN(5) == 0 {
		boundaries := []int{0, 1, 2, 100, 500, 1000}
		return boundaries[rng.IntN(len(boundaries))]
	}
	return rng.IntN(1001)
}

// generateFloat produces float64 values with multi-strategy distribution:
// 10% boundary, 10% small, 30% medium, 50% wide.
func generateFloat(rng *rand.Rand) float64 {
	r := rng.IntN(10)
	switch {
	case r == 0: // 10% boundary values
		boundaries := []float64{0.0, 1.0, -1.0, 0.5, -0.5, math.SmallestNonzeroFloat64}
		return boundaries[rng.IntN(len(boundaries))]
	case r == 1: // 10% small range [-1, 1]
		return rng.Float64()*2.0 - 1.0
	case r < 5: // 30% medium [-1e6, 1e6]
		return rng.Float64()*2e6 - 1e6
	default: // 50% wide with exponential scaling
		exp := rng.Float64() * 20.0 // exponent [0, 20]
		val := math.Pow(10, exp) * (rng.Float64()*2.0 - 1.0)
		return val
	}
}

// generateBytes produces base64-encoded random byte strings.
func generateBytes(rng *rand.Rand) string {
	var length int
	if rng.IntN(5) == 0 {
		boundaries := []int{0, 1, 16, 32}
		length = boundaries[rng.IntN(len(boundaries))]
	} else {
		length = rng.IntN(33)
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(rng.IntN(256)) //nolint:gosec // IntN(256) returns 0-255, always fits in byte
	}
	return base64.StdEncoding.EncodeToString(b)
}

func generateString(rng *rand.Rand) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	length := rng.IntN(8) + 1
	var b strings.Builder
	b.Grow(length)
	for range length {
		b.WriteByte(charset[rng.IntN(len(charset))])
	}
	return b.String()
}

// generateArray produces a random-length array with elements of the given type.
func (g *Generator) generateArray(rng *rand.Rand, elemType parser.TypeExpr) []any {
	var length int
	r := rng.IntN(20)
	switch {
	case r < 16: // 80% small [0, 5]
		length = rng.IntN(6)
	case r < 19: // 15% medium [5, 20]
		length = 5 + rng.IntN(16)
	default: // 5% large [20, 100]
		length = 20 + rng.IntN(81)
	}
	result := make([]any, length)
	for i := range result {
		result[i] = g.generateValue(rng, elemType)
	}
	return result
}

// generateMap produces a random-size map with string keys and typed values.
func (g *Generator) generateMap(rng *rand.Rand, valType parser.TypeExpr) map[string]any {
	size := rng.IntN(5) // [0, 4]
	result := make(map[string]any, size)
	for range size {
		key := generateString(rng)
		result[key] = g.generateValue(rng, valType)
	}
	return result
}

// checkConstraints returns true if all field constraints are satisfied.
func checkConstraints(input map[string]any, fields []*parser.Field) bool {
	for _, f := range fields {
		if f.Constraint == nil {
			continue
		}
		ctx := &evalCtx{input: input, fieldName: f.Name}
		val, ok := ctx.eval(f.Constraint)
		if !ok {
			return false
		}
		b, isBool := val.(bool)
		if !isBool || !b {
			return false
		}
	}
	return true
}

// evalCtx holds context for evaluating constraint expressions.
type evalCtx struct {
	input     map[string]any
	fieldName string
}

func (c *evalCtx) eval(expr parser.Expr) (any, bool) {
	if v, ok := evalLiteral(expr); ok {
		return v, true
	}

	switch e := expr.(type) {
	case parser.FieldRef:
		return c.resolveRef(e.Path)
	case parser.BinaryOp:
		return c.evalBinary(e)
	case parser.ObjectLiteral:
		return c.evalObject(e)
	case parser.ArrayLiteral:
		return c.evalArray(e)
	case parser.LenExpr:
		return c.evalLen(e)
	case parser.AllExpr:
		return c.evalAll(e)
	case parser.AnyExpr:
		return c.evalAny(e)
	case parser.ContainsExpr:
		return c.evalContains(e)
	case parser.ExistsExpr:
		return c.evalExists(e)
	case parser.HasKeyExpr:
		return c.evalHasKey(e)
	case parser.UnaryOp:
		return c.evalUnary(e)
	case parser.IfExpr:
		return c.evalIf(e)
	default:
		return nil, false
	}
}

// evalLiteral returns the Go value for literal expression types, or (nil, false)
// if the expression is not a literal.
func evalLiteral(expr parser.Expr) (any, bool) {
	switch e := expr.(type) {
	case parser.LiteralInt:
		return e.Value, true
	case parser.LiteralFloat:
		return e.Value, true
	case parser.LiteralString:
		return e.Value, true
	case parser.LiteralBool:
		return e.Value, true
	case parser.LiteralNull:
		return nil, true
	case parser.EnvRef:
		// Intentionally treat "" the same as unset — in a spec language there is
		// no meaningful distinction between "variable not set" and "set to empty."
		if val := os.Getenv(e.Var); val != "" {
			return val, true
		}
		return e.Default, true
	default:
		return nil, false
	}
}

func (c *evalCtx) evalObject(e parser.ObjectLiteral) (any, bool) {
	m := make(map[string]any, len(e.Fields))
	for _, f := range e.Fields {
		v, ok := c.eval(f.Value)
		if !ok {
			return nil, false
		}
		m[f.Key] = v
	}
	return m, true
}

func (c *evalCtx) evalArray(e parser.ArrayLiteral) (any, bool) {
	result := make([]any, len(e.Elements))
	for i, elem := range e.Elements {
		v, ok := c.eval(elem)
		if !ok {
			return nil, false
		}
		result[i] = v
	}
	return result, true
}

func (c *evalCtx) evalLen(e parser.LenExpr) (any, bool) {
	val, ok := c.eval(e.Arg)
	if !ok {
		return nil, false
	}
	switch v := val.(type) {
	case []any:
		return len(v), true
	case map[string]any:
		return len(v), true
	case string:
		return len(v), true
	default:
		return nil, false
	}
}

func (c *evalCtx) evalContains(e parser.ContainsExpr) (any, bool) {
	haystack, ok := c.eval(e.Haystack)
	if !ok {
		return nil, false
	}
	needle, ok := c.eval(e.Needle)
	if !ok {
		return nil, false
	}
	switch h := haystack.(type) {
	case string:
		n, ok := needle.(string)
		if !ok {
			return nil, false
		}
		return strings.Contains(h, n), true
	case []any:
		for _, elem := range h {
			if reflect.DeepEqual(elem, needle) {
				return true, true
			}
		}
		return false, true
	default:
		return nil, false
	}
}

func (c *evalCtx) evalExists(e parser.ExistsExpr) (any, bool) {
	_, ok := c.eval(e.Arg)
	if !ok {
		return false, true
	}
	return true, true
}

func (c *evalCtx) evalHasKey(e parser.HasKeyExpr) (any, bool) {
	obj, ok := c.eval(e.Arg)
	if !ok {
		return false, true
	}
	key, kok := c.eval(e.Key)
	if !kok {
		return nil, false
	}
	keyStr, isStr := key.(string)
	if !isStr {
		return nil, false
	}
	m, isMap := obj.(map[string]any)
	if !isMap {
		return false, true
	}
	_, exists := m[keyStr]
	return exists, true
}

func (c *evalCtx) evalIf(e parser.IfExpr) (any, bool) {
	cond, ok := c.eval(e.Condition)
	if !ok {
		return nil, false
	}
	b, isBool := cond.(bool)
	if !isBool {
		return nil, false
	}
	if b {
		return c.eval(e.Then)
	}
	return c.eval(e.Else)
}

func (c *evalCtx) evalUnary(e parser.UnaryOp) (any, bool) {
	val, ok := c.eval(e.Operand)
	if !ok {
		return nil, false
	}
	switch e.Op {
	case "!":
		b, isBool := val.(bool)
		if !isBool {
			return nil, false
		}
		return !b, true
	case "-":
		if n, ok := toInt(val); ok {
			return -n, true
		}
		if f, ok := toFloat(val); ok {
			return -f, true
		}
		return nil, false
	default:
		return nil, false
	}
}

// evalAll evaluates all(array, elem => predicate). Short-circuits on first false.
func (c *evalCtx) evalAll(e parser.AllExpr) (any, bool) {
	arrVal, ok := c.eval(e.Array)
	if !ok {
		return nil, false
	}
	arr, ok := arrVal.([]any)
	if !ok {
		return nil, false
	}
	for _, elem := range arr {
		inner := c.withBinding(e.BoundVar, elem)
		val, ok := inner.eval(e.Predicate)
		if !ok {
			return nil, false
		}
		b, isBool := val.(bool)
		if !isBool {
			return nil, false
		}
		if !b {
			return false, true
		}
	}
	return true, true
}

// evalAny evaluates any(array, elem => predicate). Short-circuits on first true.
func (c *evalCtx) evalAny(e parser.AnyExpr) (any, bool) {
	arrVal, ok := c.eval(e.Array)
	if !ok {
		return nil, false
	}
	arr, ok := arrVal.([]any)
	if !ok {
		return nil, false
	}
	for _, elem := range arr {
		inner := c.withBinding(e.BoundVar, elem)
		val, ok := inner.eval(e.Predicate)
		if !ok {
			return nil, false
		}
		b, isBool := val.(bool)
		if !isBool {
			return nil, false
		}
		if b {
			return true, true
		}
	}
	return false, true
}

// withBinding returns a new evalCtx with the bound variable added to the input scope.
// The original input map is not mutated.
func (c *evalCtx) withBinding(name string, value any) *evalCtx {
	merged := make(map[string]any, len(c.input)+1)
	for k, v := range c.input {
		merged[k] = v
	}
	merged[name] = value
	return &evalCtx{input: merged, fieldName: c.fieldName}
}

func (c *evalCtx) resolveRef(path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = c.input

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		val, exists := m[part]
		if !exists {
			return nil, false
		}
		current = val
	}

	return current, true
}

func isComparisonOp(op string) bool {
	switch op {
	case "<", "<=", ">", ">=", "==", "!=":
		return true
	}
	return false
}

func (c *evalCtx) evalBinary(op parser.BinaryOp) (any, bool) {
	// Handle chained comparisons: "0 < amount <= from.balance" is parsed as
	// BinaryOp{BinaryOp{0, "<", amount}, "<=", from.balance}. We expand this
	// to (0 < amount) AND (amount <= from.balance).
	if isComparisonOp(op.Op) {
		if inner, ok := op.Left.(parser.BinaryOp); ok && isComparisonOp(inner.Op) {
			return c.evalChainedComparison(inner, op.Op, op.Right)
		}
	}

	left, lok := c.eval(op.Left)
	if !lok {
		return nil, false
	}
	right, rok := c.eval(op.Right)
	if !rok {
		return nil, false
	}

	return evalBinaryValues(op.Op, left, right)
}

func (c *evalCtx) evalChainedComparison(
	inner parser.BinaryOp,
	outerOp string,
	outerRight parser.Expr,
) (any, bool) {
	leftResult, lok := c.evalBinary(inner)
	if !lok {
		return nil, false
	}
	lb, isBool := leftResult.(bool)
	if !isBool || !lb {
		return false, true
	}
	pivotOp := parser.BinaryOp{Left: inner.Right, Op: outerOp, Right: outerRight}
	return c.evalBinary(pivotOp)
}

func evalBinaryValues(op string, left, right any) (any, bool) {
	switch op {
	case "&&", "||":
		return evalBoolOp(op, left, right)
	case "==", "!=":
		return evalEqualityOp(op, left, right)
	case "<", "<=", ">", ">=", "+", "-", "*", "/", "%":
		// If both sides are native int, use int arithmetic (avoids float precision issues).
		ln, lok := left.(int)
		rn, rok := right.(int)
		if lok && rok {
			return evalIntOp(op, ln, rn)
		}
		// Modulo is only defined for integers.
		if op == "%" {
			return nil, false
		}
		// Otherwise try float arithmetic (handles float64 from JSON and float type).
		lf, lfok := toFloat(left)
		rf, rfok := toFloat(right)
		if lfok && rfok {
			return evalFloatOp(op, lf, rf)
		}
		return nil, false
	default:
		return nil, false
	}
}

func evalBoolOp(op string, left, right any) (any, bool) {
	lb, lok := left.(bool)
	rb, rok := right.(bool)
	if !lok || !rok {
		return nil, false
	}
	if op == "&&" {
		return lb && rb, true
	}
	return lb || rb, true
}

func evalEqualityOp(op string, left, right any) (any, bool) {
	eq := left == right
	// Fall back to numeric comparison for int/float64 mismatch.
	if !eq {
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if lok && rok {
			eq = lf == rf
		}
	}
	if op == "!=" {
		return !eq, true
	}
	return eq, true
}

func evalIntOp(op string, l, r int) (any, bool) {
	switch op {
	case "<":
		return l < r, true
	case "<=":
		return l <= r, true
	case ">":
		return l > r, true
	case ">=":
		return l >= r, true
	case "+":
		return l + r, true
	case "-":
		return l - r, true
	case "*":
		return l * r, true
	case "/":
		if r == 0 {
			return nil, false
		}
		return l / r, true
	case "%":
		if r == 0 {
			return nil, false
		}
		return l % r, true
	default:
		return nil, false
	}
}

func evalFloatOp(op string, l, r float64) (any, bool) {
	switch op {
	case "<":
		return l < r, true
	case "<=":
		return l <= r, true
	case ">":
		return l > r, true
	case ">=":
		return l >= r, true
	case "+":
		return l + r, true
	case "-":
		return l - r, true
	case "*":
		return l * r, true
	case "/":
		if r == 0 {
			return nil, false
		}
		return l / r, true
	default:
		return nil, false
	}
}

// toInt converts a value to int. Only succeeds for native int or float64
// values that are exact integers (no truncation of 3.7 to 3).
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		if math.Floor(n) == n && !math.IsInf(n, 0) && !math.IsNaN(n) {
			return int(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// toFloat converts a value to float64. Succeeds for int and float64.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
