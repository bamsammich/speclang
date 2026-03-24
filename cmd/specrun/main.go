package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	playwright "github.com/playwright-community/playwright-go"

	"github.com/bamsammich/speclang/v2/pkg/adapter"
	"github.com/bamsammich/speclang/v2/pkg/generator"
	"github.com/bamsammich/speclang/v2/pkg/infra"
	"github.com/bamsammich/speclang/v2/pkg/openapi"
	"github.com/bamsammich/speclang/v2/pkg/parser"
	protoresolver "github.com/bamsammich/speclang/v2/pkg/proto"
	"github.com/bamsammich/speclang/v2/pkg/runner"
	"github.com/bamsammich/speclang/v2/pkg/validator"
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
	case "install":
		os.Exit(runInstall(os.Args[2:]))
	default:
		//nolint:gosec // CLI writing to stderr, not a web response
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func validateSpec(spec *parser.Spec) int {
	errs := validator.Validate(spec)
	if len(errs) > 0 {
		//nolint:gosec // CLI writing to stderr, not a web response
		fmt.Fprint(os.Stderr, validator.FormatErrors(errs))
		return 1
	}
	return 0
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

	if code := validateSpec(spec); code != 0 {
		return code
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

	if code := validateSpec(spec); code != 0 {
		return code
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

// verifyOpts holds parsed flags for the verify command.
type verifyOpts struct {
	specFile     string
	seed         uint64
	iterations   int
	jsonOutput   bool
	keepServices bool
	noServices   bool
}

func parseVerifyOpts(args []string) (*verifyOpts, error) {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	opts := &verifyOpts{}
	fs.Uint64Var(&opts.seed, "seed", 42, "random seed for input generation")
	fs.IntVar(&opts.iterations, "iterations", 100, "inputs per when-scenario and invariant")
	fs.BoolVar(&opts.jsonOutput, "json", false, "output results as JSON")
	fs.BoolVar(
		&opts.keepServices, "keep-services", false, "keep containers running after verification",
	)
	fs.BoolVar(&opts.noServices, "no-services", false, "skip service lifecycle management")

	flagArgs, specFile := splitVerifyArgs(fs, args)
	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}
	if specFile == "" {
		return nil, errors.New(
			"usage: specrun verify <spec-file> [--seed N] [--iterations N] [--json] [--keep-services] [--no-services]",
		)
	}
	opts.specFile = specFile
	return opts, nil
}

// splitVerifyArgs separates flags from the positional spec file argument.
// Unlike splitFlagsAndPositional, this uses the FlagSet definition to correctly
// identify boolean flags that don't consume a following argument.
func splitVerifyArgs(fs *flag.FlagSet, args []string) (flagArgs []string, positional string) {
	boolFlags := make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) {
		if _, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			boolFlags[f.Name] = true
		}
	})

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if positional == "" {
				positional = a
			}
			continue
		}
		flagArgs = append(flagArgs, a)
		// If flag has no "=" value and is NOT a bool flag, consume the next arg as its value.
		name := strings.TrimLeft(a, "-")
		if eqIdx := strings.Index(name, "="); eqIdx >= 0 {
			continue // value is inline
		}
		if !boolFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, positional
}

func runVerify(args []string) int {
	opts, err := parseVerifyOpts(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	spec, err := parser.ParseFileWithImports(opts.specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	if code := validateSpec(spec); code != 0 {
		return code
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runningServices, cleanup, err := startServices(ctx, spec, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if cleanup != nil {
		defer cleanup()
	}

	config := resolveTargetConfig(spec.Target, runningServices)

	adapters, err := createAdapters(spec, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adapter init error: %v\n", err)
		return 1
	}
	defer closeAdapters(adapters)

	r := runner.New(spec, adapters, opts.seed)
	r.SetN(opts.iterations)

	if !opts.jsonOutput {
		fmt.Printf("verifying %s (%s) with seed=%d, iterations=%d\n\n",
			opts.specFile, spec.Name, opts.seed, opts.iterations)
	}

	return runAndReport(r, opts.jsonOutput)
}

func runAndReport(r *runner.Runner, jsonOutput bool) int {
	res, err := r.Verify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		return 1
	}

	if jsonOutput {
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

// startServices builds infra config, starts services if declared, and returns
// running services and a cleanup function. The cleanup function is nil when
// no services are running or --keep-services is set.
func startServices(
	ctx context.Context,
	spec *parser.Spec,
	opts *verifyOpts,
) ([]infra.RunningService, func(), error) {
	if opts.noServices {
		return nil, nil, nil
	}
	cfg := buildInfraConfig(spec, opts.specFile)
	manager, err := infra.NewManager(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("service manager init error: %w", err)
	}
	if manager == nil {
		return nil, nil, nil
	}

	// Pre-flight orphan removal.
	if cleanupErr := manager.Cleanup(ctx); cleanupErr != nil {
		fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", cleanupErr)
	}

	services, err := manager.Start(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("service start error: %w", err)
	}

	// Register signal handler for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\ninterrupted, cleaning up services...")
		manager.Stop(context.Background()) //nolint:errcheck // best-effort on signal
		os.Exit(1)                         //nolint:revive // intentional exit on interrupt
	}()

	var cleanup func()
	if !opts.keepServices {
		cleanup = func() {
			if stopErr := manager.Stop(ctx); stopErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to stop services: %v\n", stopErr)
			}
		}
	}

	return services, cleanup, nil
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

func runInstall(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: specrun install <plugin>")
		fmt.Fprintln(os.Stderr, "  supported: playwright")
		return 1
	}

	switch args[0] {
	case "playwright":
		fmt.Println("Installing Playwright browsers (chromium)...")
		err := playwright.Install(&playwright.RunOptions{
			Browsers: []string{"chromium"},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
			return 1
		}
		fmt.Println("Playwright browsers installed successfully.")
		return 0
	default:
		//nolint:gosec // CLI writing to stderr, not a web response
		fmt.Fprintf(os.Stderr, "unknown plugin %q (supported: playwright)\n", args[0])
		return 1
	}
}

// collectPlugins returns the unique set of plugin names from all scopes.
func collectPlugins(spec *parser.Spec) []string {
	seen := make(map[string]bool)
	var plugins []string
	for _, scope := range spec.Scopes {
		if scope.Use != "" && !seen[scope.Use] {
			seen[scope.Use] = true
			plugins = append(plugins, scope.Use)
		}
	}
	return plugins
}

func createAdapters(
	spec *parser.Spec,
	targetConfig map[string]string,
) (map[string]adapter.Adapter, error) {
	plugins := collectPlugins(spec)
	if len(plugins) == 0 {
		return nil, errors.New("no scopes declare a 'use' directive")
	}

	adapters := make(map[string]adapter.Adapter, len(plugins))
	for _, name := range plugins {
		adp, err := createSingleAdapter(name, targetConfig)
		if err != nil {
			closeAdapters(adapters)
			return nil, fmt.Errorf("initializing %q adapter: %w", name, err)
		}
		adapters[name] = adp
	}
	return adapters, nil
}

func createSingleAdapter(
	pluginName string,
	targetConfig map[string]string,
) (adapter.Adapter, error) {
	switch pluginName {
	case "http":
		adp, err := adapter.NewHTTPAdapter()
		if err != nil {
			return nil, fmt.Errorf("creating http adapter: %w", err)
		}
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
	case "playwright":
		adp := adapter.NewPlaywrightAdapter()
		if err := adp.Init(targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	default:
		return nil, fmt.Errorf("unknown plugin %q", pluginName)
	}
}

func closeAdapters(adapters map[string]adapter.Adapter) {
	for _, adp := range adapters {
		adp.Close() //nolint:errcheck // best-effort cleanup at program exit
	}
}

func resolveTargetConfig(
	target *parser.Target,
	services []infra.RunningService,
) map[string]string {
	config := make(map[string]string)
	if target == nil {
		return config
	}
	for key, expr := range target.Fields {
		config[key] = resolveExprToString(expr, target, services)
	}
	return config
}

func resolveExprToString(
	expr parser.Expr,
	target *parser.Target,
	services []infra.RunningService,
) string {
	switch e := expr.(type) {
	case parser.LiteralString:
		return e.Value
	case parser.EnvRef:
		if val := os.Getenv(e.Var); val != "" {
			return val
		}
		return e.Default
	case parser.ServiceRef:
		return resolveServiceURL(e.Name, target, services)
	default:
		return fmt.Sprintf("%v", e)
	}
}

// resolveServiceURL finds the URL for a named service. It checks running
// services first, then falls back to the declared port (for --no-services mode).
func resolveServiceURL(
	name string,
	target *parser.Target,
	services []infra.RunningService,
) string {
	for _, svc := range services {
		if svc.Name == name {
			return svc.URL
		}
	}
	if target != nil {
		for _, svc := range target.Services {
			if svc.Name == name && svc.Port > 0 {
				return fmt.Sprintf("http://localhost:%d", svc.Port)
			}
		}
	}
	return ""
}

// buildInfraConfig constructs an infra.Config from the spec's target block.
func buildInfraConfig(spec *parser.Spec, specFile string) infra.Config {
	specDir := filepath.Dir(specFile)
	cfg := infra.Config{
		SpecName: spec.Name,
		SpecDir:  specDir,
	}
	if spec.Target == nil {
		return cfg
	}
	cfg.ComposePath = resolveRelPath(specDir, spec.Target.Compose)
	for _, svc := range spec.Target.Services {
		cfg.Services = append(cfg.Services, convertServiceDef(specDir, svc))
	}
	return cfg
}

// convertServiceDef converts a parsed Service into an infra.ServiceDef,
// resolving relative paths and copying maps to avoid aliasing the AST.
func convertServiceDef(specDir string, svc *parser.Service) infra.ServiceDef {
	def := infra.ServiceDef{
		Name:   svc.Name,
		Build:  resolveRelPath(specDir, svc.Build),
		Image:  svc.Image,
		Port:   svc.Port,
		Health: svc.Health,
		Env:    copyMap(svc.Env),
	}
	if len(svc.Volumes) > 0 {
		def.Volumes = make(map[string]string, len(svc.Volumes))
		for host, container := range svc.Volumes {
			def.Volumes[resolveRelPath(specDir, host)] = container
		}
	}
	return def
}

// resolveRelPath resolves p relative to base if p is non-empty and not absolute.
func resolveRelPath(base, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// copyMap returns a shallow copy of m, or nil if m is empty.
func copyMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// defaultImports returns the built-in import registry with all supported adapters.
func defaultImports() parser.ImportRegistry {
	return parser.ImportRegistry{
		"openapi": &openapi.Resolver{},
		"proto":   &protoresolver.Resolver{},
	}
}
