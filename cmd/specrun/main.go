package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bamsammich/speclang/pkg/adapter"
	"github.com/bamsammich/speclang/pkg/generator"
	"github.com/bamsammich/speclang/pkg/openapi"
	"github.com/bamsammich/speclang/pkg/parser"
	"github.com/bamsammich/speclang/pkg/runner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: specrun verify <spec-file> [flags]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "parse":
		os.Exit(runParse(os.Args[2:]))
	case "generate":
		os.Exit(runGenerate(os.Args[2:]))
	case "verify":
		os.Exit(runVerify(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runParse(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: specrun parse <spec-file>")
		return 1
	}
	specFile := args[0]

	spec, err := parser.ParseFileWithImports(specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
		return 1
	}
	return 0
}

// splitFlagsAndPositional separates flag arguments from positional arguments.
// Flags (args starting with "-") and their values are collected into flagArgs;
// the first non-flag arg is returned as the positional arg. This allows flags
// to appear before or after the positional argument.
func splitFlagsAndPositional(args []string) (flagArgs []string, positional string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if positional == "" {
				positional = a
			}
			continue
		}
		flagArgs = append(flagArgs, a)
		if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, positional
}

func runGenerate(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	scope := fs.String("scope", "", "scope name to generate input for")
	seed := fs.Uint64("seed", 42, "random seed")

	flagArgs, specFile := splitFlagsAndPositional(args)

	if err := fs.Parse(flagArgs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if specFile == "" || *scope == "" {
		fmt.Fprintln(os.Stderr, "usage: specrun generate <spec-file> --scope <name> [--seed N]")
		return 1
	}

	spec, err := parser.ParseFileWithImports(specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	var scopeDef *parser.Scope
	for _, s := range spec.Scopes {
		if s.Name == *scope {
			scopeDef = s
			break
		}
	}
	if scopeDef == nil {
		fmt.Fprintf(os.Stderr, "scope %q not found\n", *scope)
		return 1
	}

	gen := generator.New(scopeDef.Contract, spec.Models, *seed)
	input, err := gen.GenerateInput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generation error: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(input); err != nil {
		fmt.Fprintf(os.Stderr, "encoding error: %v\n", err)
		return 1
	}
	return 0
}

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	seed := fs.Uint64("seed", 42, "random seed for input generation")
	iterations := fs.Int("iterations", 100, "inputs per when-scenario and invariant")
	jsonOutput := fs.Bool("json", false, "output results as JSON")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr,
			"usage: specrun verify <spec-file> [--seed N] [--iterations N] [--json]")
		return 1
	}
	specFile := fs.Arg(0)

	spec, err := parser.ParseFileWithImports(specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	config := resolveTargetConfig(spec.Target)

	adp, err := createAdapter(spec, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adapter init error: %v\n", err)
		return 1
	}
	defer adp.Close()

	r := runner.New(spec, adp, *seed)
	r.SetN(*iterations)

	if !*jsonOutput {
		fmt.Printf("verifying %s (%s) with seed=%d, iterations=%d\n\n",
			specFile, spec.Name, *seed, *iterations)
	}

	res, err := r.Verify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "encoding error: %v\n", err)
			return 1
		}
	} else {
		printResults(res)
	}

	if len(res.Failures) > 0 {
		return 1
	}
	return 0
}

func printResults(res *runner.Result) {
	for _, scope := range res.Scopes {
		fmt.Printf("  scope %s:\n", scope.Name)
		for _, check := range scope.Checks {
			if check.Passed {
				printPassedCheck(check)
			} else {
				printFailedCheck(check)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Scenarios:  %d/%d passed\n", res.ScenariosPassed, res.ScenariosRun)
	fmt.Printf("Invariants: %d/%d passed\n", res.InvariantsPassed, res.InvariantsChecked)
}

func printPassedCheck(check runner.CheckResult) {
	if check.InputsRun <= 1 {
		fmt.Printf("    ✓ %s %s\n", check.Kind, check.Name)
	} else {
		fmt.Printf("    ✓ %s %s (%d inputs)\n", check.Kind, check.Name, check.InputsRun)
	}
}

func printFailedCheck(check runner.CheckResult) {
	suffix := ""
	if check.Failure != nil && check.Failure.Shrunk {
		suffix = ", shrunk"
	}
	if check.InputsRun <= 1 {
		fmt.Printf("    ✗ %s %s (failed%s)\n", check.Kind, check.Name, suffix)
	} else {
		fmt.Printf("    ✗ %s %s (failed on input %d/%d%s)\n",
			check.Kind, check.Name, check.FailedAt, check.InputsRun, suffix)
	}

	if check.Failure == nil {
		return
	}

	f := check.Failure
	if f.Input != nil {
		if inputJSON, err := json.MarshalIndent(f.Input, "          ", "  "); err == nil {
			fmt.Printf("        input:\n          %s\n", inputJSON)
		}
	}
	if f.Expected != nil {
		fmt.Printf("        expected: %v\n", f.Expected)
	}
	if f.Actual != nil {
		fmt.Printf("        actual:   %v\n", f.Actual)
	}
}

func createAdapter(spec *parser.Spec, targetConfig map[string]string) (adapter.Adapter, error) {
	if len(spec.Uses) == 0 {
		return nil, errors.New("spec has no 'use' directive")
	}

	pluginName := spec.Uses[0]
	switch pluginName {
	case "http":
		adp := adapter.NewHTTPAdapter()
		if err := adp.Init(targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	case "process":
		adp := adapter.NewProcessAdapter()
		if err := adp.Init(targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	default:
		return nil, fmt.Errorf("unknown plugin %q", pluginName)
	}
}

func resolveTargetConfig(target *parser.Target) map[string]string {
	config := make(map[string]string)
	if target == nil {
		return config
	}
	for key, expr := range target.Fields {
		config[key] = resolveExprToString(expr)
	}
	return config
}

func resolveExprToString(expr parser.Expr) string {
	switch e := expr.(type) {
	case parser.LiteralString:
		return e.Value
	case parser.EnvRef:
		if val := os.Getenv(e.Var); val != "" {
			return val
		}
		return e.Default
	default:
		return fmt.Sprintf("%v", e)
	}
}

// defaultImports returns the built-in import registry with all supported adapters.
func defaultImports() parser.ImportRegistry {
	return parser.ImportRegistry{
		"openapi": &openapi.Resolver{},
	}
}
