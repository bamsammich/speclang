package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fatih/color"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/urfave/cli/v3"

	"github.com/bamsammich/speclang/v2/pkg/adapter"
	"github.com/bamsammich/speclang/v2/pkg/generator"
	"github.com/bamsammich/speclang/v2/pkg/infra"
	"github.com/bamsammich/speclang/v2/pkg/openapi"
	"github.com/bamsammich/speclang/v2/pkg/parser"
	protoresolver "github.com/bamsammich/speclang/v2/pkg/proto"
	"github.com/bamsammich/speclang/v2/pkg/runner"
	"github.com/bamsammich/speclang/v2/pkg/validator"
)

var (
	colorGreen = color.New(color.FgGreen)
	colorRed   = color.New(color.FgRed)
	colorBold  = color.New(color.Bold)
	colorDim   = color.New(color.FgHiBlack)
)

func main() {
	app := &cli.Command{
		Name:  "specrun",
		Usage: "specification verification runtime",
		Commands: []*cli.Command{
			parseCmd(),
			generateCmd(),
			verifyCmd(),
			installCmd(),
		},
		CommandNotFound: func(_ context.Context, _ *cli.Command, name string) {
			//nolint:gosec // CLI writing to stderr, not a web response
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", name)
			os.Exit(1) //nolint:revive // intentional exit for unknown command
		},
		ExitErrHandler: func(_ context.Context, _ *cli.Command, err error) {
			if err == nil {
				return
			}
			if exitErr, ok := err.(cli.ExitCoder); ok {
				if msg := exitErr.Error(); msg != "" {
					fmt.Fprintln(os.Stderr, msg)
				}
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}

func parseCmd() *cli.Command {
	return &cli.Command{
		Name:            "parse",
		Usage:           "parse a spec file and output AST as JSON",
		ArgsUsage:       "<spec-file>",
		HideHelpCommand: true,
		Action: func(_ context.Context, cmd *cli.Command) error {
			specFile := cmd.Args().First()
			if specFile == "" {
				return cli.Exit("usage: specrun parse <spec-file>", 1)
			}
			code := runParse(specFile)
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}

func generateCmd() *cli.Command {
	return &cli.Command{
		Name:            "generate",
		Usage:           "generate test input for a scope",
		ArgsUsage:       "<spec-file>",
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "scope",
				Usage:    "scope name to generate input for",
				Required: true,
			},
			&cli.Uint64Flag{
				Name:  "seed",
				Usage: "random seed",
				Value: 42,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			specFile := cmd.Args().First()
			if specFile == "" {
				return cli.Exit("usage: specrun generate <spec-file> --scope <name> [--seed N]", 1)
			}
			code := runGenerate(specFile, cmd.String("scope"), cmd.Uint64("seed"))
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}

func verifyCmd() *cli.Command {
	return &cli.Command{
		Name:            "verify",
		Usage:           "verify a spec against a target",
		ArgsUsage:       "<spec-file>",
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:  "seed",
				Usage: "random seed for input generation",
				Value: 42,
			},
			&cli.IntFlag{
				Name:  "iterations",
				Usage: "inputs per when-scenario and invariant",
				Value: 100,
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "output results as JSON",
			},
			&cli.BoolFlag{
				Name:  "keep-services",
				Usage: "keep containers running after verification",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			specFile := cmd.Args().First()
			if specFile == "" {
				return cli.Exit(
					"usage: specrun verify <spec-file> [--seed N] [--iterations N] [--json] [--keep-services]",
					1,
				)
			}
			opts := &verifyOpts{
				specFile:     specFile,
				seed:         cmd.Uint64("seed"),
				iterations:   cmd.Int("iterations"),
				jsonOutput:   cmd.Bool("json"),
				keepServices: cmd.Bool("keep-services"),
			}
			code := runVerify(opts)
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}

func installCmd() *cli.Command {
	return &cli.Command{
		Name:            "install",
		Usage:           "install plugin dependencies",
		ArgsUsage:       "<plugin>",
		HideHelpCommand: true,
		Action: func(_ context.Context, cmd *cli.Command) error {
			plugin := cmd.Args().First()
			if plugin == "" {
				return cli.Exit("usage: specrun install <plugin>\n  supported: playwright", 1)
			}
			code := runInstall(plugin)
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
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

func runParse(specFile string) int {
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

func runGenerate(specFile, scope string, seed uint64) int {
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
		if s.Name == scope {
			scopeDef = s
			break
		}
	}
	if scopeDef == nil {
		fmt.Fprintf(os.Stderr, "scope %q not found\n", scope)
		return 1
	}

	gen := generator.New(scopeDef.Contract, spec.Models, seed)
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
}

func runVerify(opts *verifyOpts) int {
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
		colorBold.Printf("Verifying %s", spec.Name)
		colorDim.Printf(" (seed=%d, iterations=%d)\n\n", opts.seed, opts.iterations)
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

// logProgress writes a dim progress message to stderr when not in JSON mode.
func logProgress(jsonOutput bool, format string, args ...any) {
	if !jsonOutput {
		colorDim.Fprintf(os.Stderr, format, args...)
	}
}

// startServices builds infra config, starts services if declared, and returns
// running services and a cleanup function. The cleanup function is nil when
// no services are running or --keep-services is set.
func startServices(
	ctx context.Context,
	spec *parser.Spec,
	opts *verifyOpts,
) ([]infra.RunningService, func(), error) {
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

	logProgress(opts.jsonOutput, "Starting services...\n")

	services, err := manager.Start(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("service start error: %w", err)
	}

	for _, svc := range services {
		logProgress(opts.jsonOutput, "  %s ready on port %d\n", svc.Name, svc.Port)
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

	cleanup := makeCleanup(ctx, manager, opts)

	return services, cleanup, nil
}

// makeCleanup returns a function that stops services with progress messages,
// or nil if --keep-services is set.
func makeCleanup(ctx context.Context, manager infra.ServiceManager, opts *verifyOpts) func() {
	if opts.keepServices {
		return nil
	}
	return func() {
		logProgress(opts.jsonOutput, "\nStopping services... ")
		if stopErr := manager.Stop(ctx); stopErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to stop services: %v\n", stopErr)
		} else {
			logProgress(opts.jsonOutput, "done\n")
		}
	}
}

func printResults(res *runner.Result) {
	for _, scope := range res.Scopes {
		colorBold.Printf("  scope %s:\n", scope.Name)
		for _, check := range scope.Checks {
			if check.Passed {
				printPassedCheck(check)
			} else {
				printFailedCheck(check)
			}
		}
		fmt.Println()
	}

	allPass := len(res.Failures) == 0
	summaryColor := colorGreen
	if !allPass {
		summaryColor = colorRed
	}
	summaryColor.Printf("Scenarios:  %d/%d passed\n", res.ScenariosPassed, res.ScenariosRun)
	summaryColor.Printf("Invariants: %d/%d passed\n", res.InvariantsPassed, res.InvariantsChecked)
}

func printPassedCheck(check runner.CheckResult) {
	colorGreen.Printf("    \u2713 %s %s", check.Kind, check.Name)
	if check.InputsRun > 1 {
		colorDim.Printf(" (%d inputs)", check.InputsRun)
	}
	fmt.Println()
}

func printFailedCheck(check runner.CheckResult) {
	suffix := ""
	if check.Failure != nil && check.Failure.Shrunk {
		suffix = ", shrunk"
	}
	if check.InputsRun <= 1 {
		colorRed.Printf("    \u2717 %s %s", check.Kind, check.Name)
		fmt.Printf(" (failed%s)\n", suffix)
	} else {
		colorRed.Printf("    \u2717 %s %s", check.Kind, check.Name)
		fmt.Printf(" (failed on input %d/%d%s)\n",
			check.FailedAt, check.InputsRun, suffix)
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

func runInstall(plugin string) int {
	switch plugin {
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
		fmt.Fprintf(os.Stderr, "unknown plugin %q (supported: playwright)\n", plugin)
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

// resolveServiceURL finds the URL for a named service from running containers.
func resolveServiceURL(
	name string,
	_ *parser.Target,
	services []infra.RunningService,
) string {
	for _, svc := range services {
		if svc.Name == name {
			return svc.URL
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
