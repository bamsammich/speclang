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

	"github.com/bamsammich/speclang/v2/internal/adapter"
	"github.com/bamsammich/speclang/v2/internal/infra"
	"github.com/bamsammich/speclang/v2/internal/openapi"
	protoresolver "github.com/bamsammich/speclang/v2/internal/proto"
	"github.com/bamsammich/speclang/v2/internal/runner"
	"github.com/bamsammich/speclang/v2/pkg/spec"
	"github.com/bamsammich/speclang/v2/pkg/specrun"
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

func validateSpec(s *spec.Spec) int {
	errs := specrun.Validate(s)
	if len(errs) > 0 {
		//nolint:gosec // CLI writing to stderr, not a web response
		fmt.Fprint(os.Stderr, specrun.FormatErrors(errs))
		return 1
	}
	return 0
}

func runParse(specFile string) int {
	s, err := specrun.ParseFile(specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	if code := validateSpec(s); code != 0 {
		return code
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
		return 1
	}
	return 0
}

func runGenerate(specFile, scopeName string, seed uint64) int {
	s, err := specrun.ParseFile(specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	if code := validateSpec(s); code != 0 {
		return code
	}

	input, err := specrun.Generate(s, scopeName, seed)
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
	s, err := specrun.ParseFile(opts.specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	if code := validateSpec(s); code != 0 {
		return code
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runningServices, cleanup, err := startServices(ctx, s, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if cleanup != nil {
		defer cleanup()
	}

	config := resolveTargetConfig(s.Target, runningServices)

	adapters, err := createAdapters(s, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adapter init error: %v\n", err)
		return 1
	}
	defer closeAdapters(adapters)

	r := runner.New(s, adapters, opts.seed)
	r.SetN(opts.iterations)

	if !opts.jsonOutput {
		colorBold.Printf("Verifying %s", s.Name)
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

// svcPortEnvKey returns the env var name used to propagate a running service's
// URL by port (e.g., "SPECRUN_SVC_PORT_8080" for port 8080).
func svcPortEnvKey(port int) string {
	return fmt.Sprintf("SPECRUN_SVC_PORT_%d", port)
}

// inheritedServiceURL returns the URL of a service already started by a parent
// specrun process, identified by declared port. Returns "" if not inherited.
func inheritedServiceURL(port int) string {
	return os.Getenv(svcPortEnvKey(port))
}

// resolveInheritedServices checks if ALL declared services have URLs inherited
// from a parent specrun process (via SPECRUN_SVC_PORT_* env vars). Returns
// RunningService entries if all are inherited, nil otherwise.
func resolveInheritedServices(
	defs []infra.ServiceDef,
) []infra.RunningService {
	if len(defs) == 0 {
		return nil
	}
	var services []infra.RunningService
	for _, def := range defs {
		url := inheritedServiceURL(def.Port)
		if url == "" {
			return nil
		}
		services = append(services, infra.RunningService{
			Name: def.Name,
			URL:  url,
			Port: def.Port,
		})
	}
	return services
}

// startServices builds infra config, starts services if declared, and returns
// running services and a cleanup function. The cleanup function is nil when
// no services are running or --keep-services is set.
// Services already started by a parent specrun process (identified by
// SPECRUN_SVC_PORT_* env vars) are reused without starting new containers.
func startServices(
	ctx context.Context,
	s *spec.Spec,
	opts *verifyOpts,
) ([]infra.RunningService, func(), error) {
	cfg := buildInfraConfig(s, opts.specFile)

	// Check if all declared services are already provided by a parent process.
	inherited := resolveInheritedServices(cfg.Services)
	if inherited != nil {
		return inherited, nil, nil
	}

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

	// Propagate service URLs by port to child processes so nested specrun
	// invocations reuse the already-running containers.
	for _, svc := range services {
		//nolint:errcheck // env propagation is best-effort
		os.Setenv(svcPortEnvKey(svc.Port), svc.URL)
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
func collectPlugins(s *spec.Spec) []string {
	seen := make(map[string]bool)
	var plugins []string
	for _, scope := range s.Scopes {
		if scope.Use != "" && !seen[scope.Use] {
			seen[scope.Use] = true
			plugins = append(plugins, scope.Use)
		}
	}
	return plugins
}

func createAdapters(
	s *spec.Spec,
	targetConfig map[string]string,
) (map[string]adapter.Adapter, error) {
	plugins := collectPlugins(s)
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
	target *spec.Target,
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
	expr spec.Expr,
	target *spec.Target,
	services []infra.RunningService,
) string {
	switch e := expr.(type) {
	case spec.LiteralString:
		return e.Value
	case spec.EnvRef:
		if val := os.Getenv(e.Var); val != "" {
			return val
		}
		return e.Default
	case spec.ServiceRef:
		return resolveServiceURL(e.Name, target, services)
	default:
		return fmt.Sprintf("%v", e)
	}
}

// resolveServiceURL finds the URL for a named service. It checks running
// services first, then checks SPECRUN_SVC_PORT_* env vars (set by a parent
// specrun process that started the container on a declared port).
func resolveServiceURL(
	name string,
	target *spec.Target,
	services []infra.RunningService,
) string {
	for _, svc := range services {
		if svc.Name == name {
			return svc.URL
		}
	}
	// Check if a parent process started a container on this service's port.
	if target != nil {
		for _, svc := range target.Services {
			if svc.Name == name && svc.Port > 0 {
				if url := inheritedServiceURL(svc.Port); url != "" {
					return url
				}
			}
		}
	}
	return ""
}

// buildInfraConfig constructs an infra.Config from the spec's target block.
func buildInfraConfig(s *spec.Spec, specFile string) infra.Config {
	specDir := filepath.Dir(specFile)
	cfg := infra.Config{
		SpecName: s.Name,
		SpecDir:  specDir,
	}
	if s.Target == nil {
		return cfg
	}
	cfg.ComposePath = resolveRelPath(specDir, s.Target.Compose)
	for _, svc := range s.Target.Services {
		cfg.Services = append(cfg.Services, convertServiceDef(specDir, svc))
	}
	return cfg
}

// convertServiceDef converts a parsed Service into an infra.ServiceDef,
// resolving relative paths and copying maps to avoid aliasing the AST.
func convertServiceDef(specDir string, svc *spec.Service) infra.ServiceDef {
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
func defaultImports() spec.ImportRegistry {
	return spec.ImportRegistry{
		"openapi": &openapi.Resolver{},
		"proto":   &protoresolver.Resolver{},
	}
}
