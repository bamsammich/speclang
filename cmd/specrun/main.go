package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/urfave/cli/v3"

	"github.com/bamsammich/speclang/v4/internal/adapter"
	"github.com/bamsammich/speclang/v4/internal/generator"
	"github.com/bamsammich/speclang/v4/internal/migrate"
	"github.com/bamsammich/speclang/v4/internal/parser"
	"github.com/bamsammich/speclang/v4/internal/infra"
	"github.com/bamsammich/speclang/v4/internal/openapi"
	protoresolver "github.com/bamsammich/speclang/v4/internal/proto"
	"github.com/bamsammich/speclang/v4/internal/runner"
	"github.com/bamsammich/speclang/v4/pkg/spec"
	"github.com/bamsammich/speclang/v4/pkg/specrun"
	"gopkg.in/yaml.v3"
)

// expandGlobs resolves a list of patterns (which may include ** for recursive
// matching) into a sorted, deduplicated list of file paths. Patterns without **
// are handled by filepath.Glob. Patterns containing ** are resolved via
// filepath.Walk with per-segment filepath.Match, keeping stdlib-only.
func expandGlobs(patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var files []string

	for _, pat := range patterns {
		if !strings.Contains(pat, "**") {
			matches, err := filepath.Glob(pat)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", pat, err)
			}
			for _, m := range matches {
				abs, err := filepath.Abs(m)
				if err != nil {
					return nil, fmt.Errorf("abs path %q: %w", m, err)
				}
				if _, dup := seen[abs]; !dup {
					seen[abs] = struct{}{}
					files = append(files, abs)
				}
			}
			continue
		}

		// Recursive glob: walk from the non-glob prefix.
		root, rest := splitDoublestar(pat)
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			ok, matchErr := matchDoublestar(rest, rel)
			if matchErr != nil {
				return fmt.Errorf("invalid glob segment %q: %w", rest, matchErr)
			}
			if !ok {
				return nil
			}
			abs, absErr := filepath.Abs(path)
			if absErr != nil {
				return absErr
			}
			if _, dup := seen[abs]; !dup {
				seen[abs] = struct{}{}
				files = append(files, abs)
			}
			return nil
		}); err != nil {
			// A walk error on a non-existent root means zero matches, not a fatal error.
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("walk %q: %w", root, err)
		}
	}

	return files, nil
}

// splitDoublestar returns (root, rest) where root is the longest path prefix
// before the first path segment containing ** and rest is the remainder of the
// pattern (relative to root). Examples:
//
//	"specs/**/*.spec"         → ("specs", "**/*.spec")
//	"a/b/**/*.spec"           → ("a/b", "**/*.spec")
//	"**/*.spec"               → (".", "**/*.spec")
//	"/abs/path/**/*.spec"     → ("/abs/path", "**/*.spec")
func splitDoublestar(pat string) (root, rest string) {
	// Use os.PathSeparator-normalised slashes, but track whether path is absolute.
	slashPat := filepath.ToSlash(pat)
	segments := strings.Split(slashPat, "/")

	for i, seg := range segments {
		if !strings.Contains(seg, "**") {
			continue
		}
		if i == 0 {
			return ".", strings.Join(segments, "/")
		}
		// Re-join the prefix segments back into a native path, preserving
		// any leading separator for absolute patterns.
		prefix := strings.Join(segments[:i], "/")
		return filepath.FromSlash(prefix), strings.Join(segments[i:], "/")
	}
	// No ** found — shouldn't happen given caller check, but be safe.
	return filepath.Dir(pat), filepath.Base(pat)
}

// matchDoublestar reports whether rel (a slash-separated path relative to the
// walk root) matches the glob pattern pat, which may contain ** segments.
// ** matches zero or more path segments.
func matchDoublestar(pat, rel string) (bool, error) {
	// Normalise to forward slashes for uniform handling.
	pat = filepath.ToSlash(pat)
	rel = filepath.ToSlash(rel)

	patSegs := strings.Split(pat, "/")
	relSegs := strings.Split(rel, "/")

	return matchSegments(patSegs, relSegs)
}

// matchSegments is the recursive worker for matchDoublestar.
func matchSegments(patSegs, relSegs []string) (bool, error) {
	for len(patSegs) > 0 && len(relSegs) > 0 {
		p := patSegs[0]
		if p == "**" {
			// ** can consume zero or more segments. Try all possibilities.
			for skip := 0; skip <= len(relSegs); skip++ {
				ok, err := matchSegments(patSegs[1:], relSegs[skip:])
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
			}
			return false, nil
		}
		ok, err := filepath.Match(p, relSegs[0])
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		patSegs = patSegs[1:]
		relSegs = relSegs[1:]
	}
	// Both exhausted = full match.
	return len(patSegs) == 0 && len(relSegs) == 0, nil
}

// specHasContracts reports whether a parsed spec has at least one contract
// (at top level or inside a scope).
func specHasContracts(s *spec.Spec) bool {
	if len(s.Contracts) > 0 {
		return true
	}
	for _, sc := range s.Scopes {
		if len(sc.Contracts) > 0 {
			return true
		}
	}
	return false
}

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
			migrateCmd(),
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
		ArgsUsage:       "<spec-file|glob> [spec-file|glob...]",
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
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return cli.Exit(
					"usage: specrun verify <spec-file|glob> [spec-file|glob...] [--seed N] [--iterations N] [--json] [--keep-services]",
					1,
				)
			}

			patterns := make([]string, cmd.NArg())
			for i := range cmd.NArg() {
				patterns[i] = cmd.Args().Get(i)
			}

			files, err := expandGlobs(patterns)
			if err != nil {
				return cli.Exit(fmt.Sprintf("glob error: %v", err), 1)
			}
			if len(files) == 0 {
				fmt.Fprintf(os.Stderr, "error: no files matched: %s\n", strings.Join(patterns, " "))
				return cli.Exit("", 1)
			}

			baseOpts := &verifyOpts{
				seed:         cmd.Uint64("seed"),
				iterations:   cmd.Int("iterations"),
				jsonOutput:   cmd.Bool("json"),
				keepServices: cmd.Bool("keep-services"),
			}

			exitCode := 0
			for _, specFile := range files {
				opts := *baseOpts
				opts.specFile = specFile
				c := runVerify(ctx, &opts)
				if c != 0 {
					exitCode = c
				}
			}
			if exitCode != 0 {
				return cli.Exit("", exitCode)
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

func migrateCmd() *cli.Command {
	return &cli.Command{
		Name:            "migrate",
		Usage:           "convert a spec file to a newer syntax version",
		ArgsUsage:       "<spec-file> [spec-file...]",
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "write",
				Aliases: []string{"w"},
				Usage:   "write result back to source file(s)",
			},
			&cli.StringFlag{
				Name:  "to",
				Usage: "target version: v3 (v2→v3) or v4 (v3→v4)",
				Value: "v4",
			},
			&cli.BoolFlag{
				Name:  "backup",
				Usage: "write <file>.v3.bak (v4) or <file>.v2.bak (v3) with original content before overwriting (implies --write)",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "write output even if it does not re-parse as the target version (skips round-trip validation)",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return cli.Exit("usage: specrun migrate [--to v3|v4] <spec-file> [spec-file...]", 1)
			}
			write := cmd.Bool("write")
			backup := cmd.Bool("backup")
			force := cmd.Bool("force")
			target := cmd.String("to")
			if target != "v3" && target != "v4" {
				return cli.Exit("--to must be v3 or v4", 1)
			}
			code := 0
			for i := range cmd.NArg() {
				specFile := cmd.Args().Get(i)
				var c int
				if target == "v4" {
					c = runMigrateV4(specFile, write, backup, force)
				} else {
					c = runMigrate(specFile, write, backup)
				}
				if c != 0 {
					code = c
				}
			}
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}

func runMigrate(specFile string, write, backup bool) int {
	results, err := migrate.MigrateFile(specFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate error: %v\n", err)
		return 1
	}

	for _, mf := range results {
		if mf.Warning != "" {
			fmt.Fprintf(os.Stderr, "warning: %s: %s\n", mf.Path, mf.Warning)
		}

		if write || backup {
			orig, err := os.ReadFile(mf.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read error: %v\n", err)
				return 1
			}
			if backup {
				bakPath := mf.Path + ".v2.bak"
				if err := os.WriteFile(bakPath, orig, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "backup error: %v\n", err)
					return 1
				}
			}
			if err := atomicWrite(mf.Path, []byte(mf.Output)); err != nil {
				fmt.Fprintf(os.Stderr, "write error: %v\n", err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "migrated: %s\n", mf.Path)
		} else {
			if len(results) > 1 {
				fmt.Fprintf(os.Stdout, "# --- %s ---\n", mf.Path)
			}
			fmt.Print(mf.Output)
		}
	}

	return 0
}

func runMigrateV4(specFile string, write, backup, force bool) int {
	src, err := os.ReadFile(specFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		return 1
	}

	output, err := migrate.MigrateV3File(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate error: %v\n", err)
		return 1
	}

	// Round-trip parse validation: ensure the migrated output is valid v4.
	if !force {
		if _, parseErr := parser.Parse(output); parseErr != nil {
			fmt.Fprintf(os.Stderr,
				"migration produced output that does not parse as v4 (internal bug — please file an issue with the input): %v\nuse --force to bypass this check\n",
				parseErr,
			)
			return 1
		}
	}

	if write || backup {
		if backup {
			bakPath := specFile + ".v3.bak"
			if err := os.WriteFile(bakPath, src, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "backup error: %v\n", err)
				return 1
			}
		}
		if err := atomicWrite(specFile, []byte(output)); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "migrated: %s\n", specFile)
	} else {
		fmt.Print(output)
	}

	return 0
}

// atomicWrite writes data to path using a temp file + rename to avoid
// leaving a half-written file if the process is interrupted mid-write.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".migrate-*.spec.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Clean up temp file on any error after creation.
	var writeErr error
	defer func() {
		if writeErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		writeErr = fmt.Errorf("writing temp file: %w", err)
		_ = tmp.Close()
		return writeErr
	}
	if err := tmp.Sync(); err != nil {
		writeErr = fmt.Errorf("syncing temp file: %w", err)
		_ = tmp.Close()
		return writeErr
	}
	if err := tmp.Close(); err != nil {
		writeErr = fmt.Errorf("closing temp file: %w", err)
		return writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		writeErr = fmt.Errorf("renaming temp file: %w", err)
		return writeErr
	}
	return nil
}

func validateSpec(s *spec.Spec) int {
	errs := specrun.Validate(s, nil)
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

func runVerify(ctx context.Context, opts *verifyOpts) int {
	s, err := specrun.ParseFile(opts.specFile, defaultImports())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return 1
	}

	if code := validateSpec(s); code != 0 {
		return code
	}

	if !specHasContracts(s) {
		fmt.Fprintf(os.Stderr, "%s: no contracts, skipping\n", filepath.Base(opts.specFile))
		return 0
	}

	runningServices, cleanup, err := startServices(ctx, s, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if cleanup != nil {
		defer cleanup()
	}

	config := resolveTargetConfig(s.Target, runningServices)

	adapters, err := createAdapters(ctx, s, config, runningServices)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adapter init error: %v\n", err)
		return 1
	}
	defer closeAdapters(ctx, adapters)

	r := runner.New(s, adapters, opts.seed)
	r.SetN(opts.iterations)

	if !opts.jsonOutput {
		colorBold.Printf("Verifying %s", filepath.Base(opts.specFile))
		colorDim.Printf(" (seed=%d, iterations=%d)\n\n", opts.seed, opts.iterations)
	}

	return runAndReport(ctx, r, opts.jsonOutput, filepath.Base(opts.specFile))
}

func runAndReport(ctx context.Context, r *runner.Runner, jsonOutput bool, specName string) int {
	res, err := r.Verify(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		return 1
	}
	res.Spec = specName

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
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
	s *spec.Spec,
	opts *verifyOpts,
) ([]infra.RunningService, func(), error) {
	cfg := buildInfraConfig(s, opts.specFile)

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
	detail := buildFailureDetail(f)
	buf, err := yaml.Marshal(detail)
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(buf), "\n"), "\n") {
		fmt.Printf("        %s\n", line)
	}
}

// failureDetail is the YAML-serialized failure context shown to the user.
type failureDetail struct {
	Description string `yaml:"description,omitempty"`
	Expected    any    `yaml:"expected,omitempty"`
	Actual      any    `yaml:"actual,omitempty"`
	Input       any    `yaml:"input,omitempty"`
}

func buildFailureDetail(f *spec.Failure) failureDetail {
	return failureDetail{
		Description: f.Description,
		Expected:    f.Expected,
		Actual:      f.Actual,
		Input:       f.Input,
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
// In v2 mode, plugins come from scope.Use directives.
// collectPlugins returns adapter names declared via config blocks (http { ... }, playwright { ... }).
func collectPlugins(s *spec.Spec) []string {
	seen := make(map[string]bool)
	var plugins []string

	// Adapter config blocks (http { ... }, playwright { ... })
	for name := range s.AdapterConfigs {
		if !seen[name] {
			seen[name] = true
			plugins = append(plugins, name)
		}
	}

	return plugins
}

func createAdapters(
	ctx context.Context,
	s *spec.Spec,
	targetConfig map[string]string,
	services []infra.RunningService,
) (map[string]adapter.Adapter, error) {
	plugins := collectPlugins(s)
	if len(plugins) == 0 {
		return nil, errors.New("no adapters configured (no 'use' directives or adapter config blocks)")
	}

	adapters := make(map[string]adapter.Adapter, len(plugins))
	for _, name := range plugins {
		// Build per-adapter config: merge v2 target config with v3 adapter config
		adapterConfig := make(map[string]string, len(targetConfig))
		for k, v := range targetConfig {
			adapterConfig[k] = v
		}
		// v3: overlay adapter-specific config
		if acfg, ok := s.AdapterConfigs[name]; ok {
			for key, expr := range acfg {
				adapterConfig[key] = resolveExprToString(expr, services)
			}
		}

		adp, err := createSingleAdapter(ctx, name, adapterConfig)
		if err != nil {
			closeAdapters(ctx, adapters)
			return nil, fmt.Errorf("initializing %q adapter: %w", name, err)
		}
		adapters[name] = adp
	}
	return adapters, nil
}

func createSingleAdapter(
	ctx context.Context,
	pluginName string,
	targetConfig map[string]string,
) (adapter.Adapter, error) {
	switch pluginName {
	case "http":
		adp, err := adapter.NewHTTPAdapter()
		if err != nil {
			return nil, fmt.Errorf("creating http adapter: %w", err)
		}
		if err := adp.Init(ctx, targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	case "process":
		adp := adapter.NewProcessAdapter()
		if err := adp.Init(ctx, targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	case "playwright":
		adp := adapter.NewPlaywrightAdapter()
		if err := adp.Init(ctx, targetConfig); err != nil {
			return nil, err
		}
		return adp, nil
	default:
		return nil, fmt.Errorf("unknown plugin %q", pluginName)
	}
}

func closeAdapters(ctx context.Context, adapters map[string]adapter.Adapter) {
	for _, adp := range adapters {
		adp.Close(ctx) //nolint:errcheck // best-effort cleanup at program exit
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
		config[key] = resolveExprToString(expr, services)
	}
	return config
}

func resolveExprToString(
	expr spec.Expr,
	services []infra.RunningService,
) string {
	if e, ok := expr.(spec.ServiceRef); ok {
		return resolveServiceURL(e.Name, services)
	}
	val, ok := generator.Eval(expr, nil)
	if !ok {
		return ""
	}
	if s, isStr := val.(string); isStr {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// resolveServiceURL finds the URL for a named service from running services.
func resolveServiceURL(
	name string,
	services []infra.RunningService,
) string {
	for _, svc := range services {
		if svc.Name == name {
			return svc.URL
		}
	}
	return ""
}

// buildInfraConfig constructs an infra.Config from the spec's target/services blocks.
func buildInfraConfig(s *spec.Spec, specFile string) infra.Config {
	specDir := filepath.Dir(specFile)
	cfg := infra.Config{
		SpecName: filepath.Base(specFile),
		SpecDir:  specDir,
	}

	// v3: spec-level services block
	for _, svc := range s.Services {
		cfg.Services = append(cfg.Services, convertServiceDef(specDir, svc))
	}

	if s.Target == nil {
		return cfg
	}
	// v2 compat: target block
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
		Name:    svc.Name,
		Build:   resolveRelPath(specDir, svc.Build),
		Compose: resolveRelPath(specDir, svc.Compose),
		Image:   svc.Image,
		Port:    svc.Port,
		Health:  svc.Health,
		Env:     copyMap(svc.Env),
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
