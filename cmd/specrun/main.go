package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bamsammich/speclang/pkg/adapter"
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

	spec, err := parser.ParseFile(specFile)
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

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	seed := fs.Uint64("seed", 42, "random seed for input generation")
	iterations := fs.Int("iterations", 100, "inputs per when-scenario and invariant")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: specrun verify <spec-file> [--seed N] [--iterations N]")
		return 1
	}
	specFile := fs.Arg(0)

	spec, err := parser.ParseFile(specFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	config := resolveTargetConfig(spec.Target)

	adp := adapter.NewHTTPAdapter()
	if err := adp.Init(config); err != nil {
		fmt.Fprintf(os.Stderr, "adapter init error: %v\n", err)
		return 1
	}
	defer adp.Close()

	r := runner.New(spec, adp, *seed)
	r.SetN(*iterations)

	fmt.Printf("verifying %s (%s) with seed=%d, iterations=%d\n\n",
		specFile, spec.Name, *seed, *iterations)

	res, err := r.Verify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		return 1
	}

	printResults(res)

	if len(res.Failures) > 0 {
		return 1
	}
	return 0
}

func printResults(res *runner.Result) {
	fmt.Printf("Scenarios:  %d/%d passed\n", res.ScenariosPassed, res.ScenariosRun)
	fmt.Printf("Invariants: %d/%d passed\n", res.InvariantsPassed, res.InvariantsChecked)

	if len(res.Failures) == 0 {
		fmt.Println("\nAll checks passed.")
		return
	}

	fmt.Println("\nFailures:")
	for _, f := range res.Failures {
		fmt.Printf("\n  [%s] %s: %s\n", f.Scope, f.Name, f.Description)
		if f.Shrunk {
			fmt.Println("    (shrunk to minimal counterexample)")
		}
		if f.Input != nil {
			if inputJSON, err := json.MarshalIndent(f.Input, "    ", "  "); err == nil {
				fmt.Printf("    input: %s\n", inputJSON)
			}
		}
		if f.Expected != nil {
			fmt.Printf("    expected: %v\n", f.Expected)
		}
		if f.Actual != nil {
			fmt.Printf("    actual:   %v\n", f.Actual)
		}
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
