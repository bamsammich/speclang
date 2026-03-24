# CLI Migration to urfave/cli v3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate specrun's CLI from manual stdlib `flag` dispatch to urfave/cli v3 with colored output, live progress, and improved help — making specrun pleasant to use.

**Architecture:** Replace `switch os.Args[1]` dispatch with `cli.Command` tree. Add `fatih/color` for terminal-aware colored output. Add live progress messages to stderr during service lifecycle and verification. Replace `--no-services` flag with `SPECRUN_NO_SERVICES` env var. Delete custom arg-splitting functions.

**Tech Stack:** urfave/cli v3 (v3.7.0), fatih/color, Go stdlib

---

## Phase 1: Dependencies and scaffolding

### Task 1: Add dependencies and create CLI skeleton

**Files:**
- Modify: `cmd/specrun/main.go`
- Modify: `go.mod`

**Step 1: Add dependencies**

```bash
go get github.com/urfave/cli/v3@latest
go get github.com/fatih/color@latest
go mod tidy
```

**Step 2: Create the root command skeleton**

Replace the `main()` function and `switch` dispatch with a urfave/cli root command. Keep all existing `runX` functions — just wire them as `Action` funcs on subcommands.

```go
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
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
```

Each `*Cmd()` function returns a `*cli.Command` with the appropriate flags and action. For now, the action bodies just extract flags from `cmd` and delegate to the existing `runX` logic.

**Step 3: Wire `parseCmd`**

```go
func parseCmd() *cli.Command {
	return &cli.Command{
		Name:      "parse",
		Usage:     "Parse a spec file and output the AST as JSON",
		ArgsUsage: "<spec-file>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return cli.Exit("usage: specrun parse <spec-file>", 1)
			}
			code := runParse(cmd.Args().Slice())
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}
```

**Step 4: Wire `generateCmd`**

```go
func generateCmd() *cli.Command {
	return &cli.Command{
		Name:      "generate",
		Usage:     "Generate one random input for a scope",
		ArgsUsage: "<spec-file>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "scope", Usage: "scope name to generate for", Required: true},
			&cli.Uint64Flag{Name: "seed", Value: 42, Usage: "random seed for input generation"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Extract flags and delegate to existing logic
			// No more splitFlagsAndPositional needed
			specFile := cmd.Args().First()
			if specFile == "" {
				return cli.Exit("usage: specrun generate <spec-file> --scope <name> [--seed N]", 1)
			}
			scope := cmd.String("scope")
			seed := cmd.Uint64("seed")
			// ... call existing generate logic with these values
			return nil
		},
	}
}
```

**Step 5: Wire `verifyCmd`**

```go
func verifyCmd() *cli.Command {
	return &cli.Command{
		Name:      "verify",
		Usage:     "Run verification against a spec",
		ArgsUsage: "<spec-file>",
		Flags: []cli.Flag{
			&cli.Uint64Flag{Name: "seed", Value: 42, Usage: "random seed for input generation"},
			&cli.IntFlag{Name: "iterations", Value: 100, Usage: "inputs per invariant/when-scenario"},
			&cli.BoolFlag{Name: "json", Usage: "output results as JSON"},
			&cli.BoolFlag{Name: "keep-services", Usage: "keep containers running after verification for debugging"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// --no-services is now SPECRUN_NO_SERVICES env var
			noServices := os.Getenv("SPECRUN_NO_SERVICES") == "1"
			specFile := cmd.Args().First()
			if specFile == "" {
				return cli.Exit("usage: specrun verify <spec-file> [--seed N] [--iterations N] [--json] [--keep-services]", 1)
			}
			// ... delegate to existing verify logic
			return nil
		},
	}
}
```

**Step 6: Wire `installCmd`**

```go
func installCmd() *cli.Command {
	return &cli.Command{
		Name:      "install",
		Usage:     "Install external tool dependencies",
		ArgsUsage: "<plugin>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return cli.Exit("usage: specrun install <plugin>", 1)
			}
			code := runInstall(cmd.Args().Slice())
			if code != 0 {
				return cli.Exit("", code)
			}
			return nil
		},
	}
}
```

**Step 7: Delete old dispatch code**

Remove: `splitFlagsAndPositional`, `splitVerifyArgs`, `verifyOpts`, `parseVerifyOpts`. Refactor `runGenerate` and `runVerify` to accept extracted values directly instead of parsing args themselves.

**Step 8: Verify**

```bash
go build ./cmd/specrun
./specrun --help                    # should show subcommands
./specrun verify --help             # should show flags
./specrun parse examples/transfer.spec  # should work
go test ./... -count=1              # all pass
golangci-lint run ./...             # zero issues
```

**Step 9: Commit**

```bash
git add -A
git commit -m "refactor(cli): migrate to urfave/cli v3"
```

---

## Phase 2: Colored output

### Task 2: Add colored output with auto-detection

**Files:**
- Modify: `cmd/specrun/main.go` (printResults, printPassedCheck, printFailedCheck, summary)

**Step 1: Create a color helper**

Use `fatih/color` which auto-detects terminal vs pipe. Define colors at package level:

```go
var (
	colorGreen  = color.New(color.FgGreen)
	colorRed    = color.New(color.FgRed)
	colorBold   = color.New(color.Bold)
	colorDim    = color.New(color.FgHiBlack)
)
```

`fatih/color` automatically disables colors when stdout is not a terminal (piped) or when `NO_COLOR` env var is set.

**Step 2: Update print functions**

- `printPassedCheck`: `colorGreen.Printf("    ✓ %s %s", kind, name)` + dim inputs count
- `printFailedCheck`: `colorRed.Printf("    ✗ %s %s", kind, name)` + failure details
- Scope headers: `colorBold.Printf("  scope %s:\n", name)`
- Summary: green if all pass, red if any fail
- `--json` output is never colored (it goes through `json.Encoder`, not print functions)

**Step 3: Verify**

Run `./specrun verify examples/transfer.spec` in terminal — should see colors.
Run `./specrun verify examples/transfer.spec | cat` — should see no ANSI codes.
Run `NO_COLOR=1 ./specrun verify examples/transfer.spec` — should see no ANSI codes.

**Step 4: Commit**

```bash
git add cmd/specrun/main.go go.mod go.sum
git commit -m "feat(cli): add colored output with terminal auto-detection"
```

---

## Phase 3: Live progress

### Task 3: Add live progress for service lifecycle and verification

**Files:**
- Modify: `cmd/specrun/main.go` (startServices, runVerify)
- Possibly modify: `pkg/infra/docker.go`, `pkg/infra/compose.go` (if progress callbacks are needed)

**Step 1: Add progress output to service lifecycle**

In `startServices`, add progress messages to stderr (so stdout stays clean for `--json`):

```go
colorDim.Fprintf(os.Stderr, "Starting services...\n")
// For each service started:
colorDim.Fprintf(os.Stderr, "  Building %s... done (%.1fs)\n", svc.Name, elapsed.Seconds())
colorDim.Fprintf(os.Stderr, "  Health check %s:%d... ready\n", svc.Name, svc.Port)
```

The `infra.ServiceManager.Start()` currently returns `[]RunningService` after all are started. To show per-service progress, either:
- (a) Add a progress callback to `Start()`: `Start(ctx, func(event string))`
- (b) Log inside the infra package (less clean)
- (c) Start services one at a time from main.go (if the API supports it)

Option (a) is cleanest — add an optional `ProgressFunc func(string)` field to `infra.Config`. The manager calls it at key points.

**Step 2: Add progress to verification output**

The current flow: `r.Verify()` returns a complete `Result`. Results are printed after. For live progress, either:
- (a) Print each scope result as it completes (requires changing `runner.Verify` to accept a callback)
- (b) Keep current behavior, print all at end (simpler, no runner changes)

Go with (b) for now — the current batch output is fine. Live progress is for the service lifecycle, which is the slow part.

**Step 3: Add cleanup progress**

```go
colorDim.Fprintf(os.Stderr, "Stopping services... ")
// after stop:
colorDim.Fprintf(os.Stderr, "done\n")
```

**Step 4: Suppress progress in `--json` mode**

Check if `opts.jsonOutput` before printing progress. Progress goes to stderr but it's still noise for automated consumers.

Actually — progress to stderr is fine even in `--json` mode. JSON consumers read stdout. stderr progress is for the human watching. Keep it.

**Step 5: Verify**

Run `./specrun verify examples/transfer.spec` — should see service lifecycle messages and colored results.

**Step 6: Commit**

```bash
git add -A
git commit -m "feat(cli): add live progress for service lifecycle"
```

---

## Phase 4: Replace `--no-services` with env var

### Task 4: Replace `--no-services` flag with `SPECRUN_NO_SERVICES` env var

**Files:**
- Modify: `cmd/specrun/main.go` — remove `--no-services` flag, check env var instead
- Modify: `specs/cli_flags.spec` — update `config.args` to remove `--no-services`, set env var
- Modify: `specs/verify.spec` — same
- Modify: `cmd/specrun/main_test.go` — update `TestSelfVerification_Parse` to set env var instead of flag
- Modify: `.github/workflows/ci.yml` — set `SPECRUN_NO_SERVICES=1` in self-verification step
- Modify: `CLAUDE.md` — update Commands section
- Modify: `docs/services.md` — update `--no-services` references

**Step 1: Replace flag with env var check**

In the verify action (or wherever `noServices` is checked):
```go
noServices := os.Getenv("SPECRUN_NO_SERVICES") == "1"
```

Remove `--no-services` from the `Flags` list and the `verifyOpts` struct (if it still exists).

**Step 2: Update self-verification specs**

In `specs/cli_flags.spec` and `specs/verify.spec`, the `config.args` lines currently include `--no-services`. Remove the flag from args. Instead, the process adapter needs to pass the env var to the subprocess.

IMPORTANT: The process adapter runs commands via `os/exec`. The subprocess inherits the parent's environment. So if the parent has `SPECRUN_NO_SERVICES=1`, the subprocess will too. The self-verification root spec (`specs/speclang.spec`) is run with `SPECRUN_NO_SERVICES=1` set by CI. The subprocess invocations in fixture specs will inherit it automatically.

This means: just remove `--no-services` from the `config.args` in the specs. The env var propagates naturally.

BUT WAIT: currently, `--no-services` is in `config.args` for specific scopes. These scopes run `specrun verify <fixture>` as a subprocess. The subprocess MUST have `SPECRUN_NO_SERVICES=1` to avoid trying to start containers that don't exist in the fixture spec.

Since the root `specrun verify specs/speclang.spec` is run with `SPECRUN_NO_SERVICES=1` in CI (because the root spec's services are started externally), the subprocess inherits it. When services ARE managed by the root spec, the subprocesses should ALSO skip service management (the containers are already running from the root).

So the flow is:
1. CI sets `SPECRUN_NO_SERVICES=1` and starts servers manually → root specrun skips services → subprocess specrun inherits the env var → subprocess also skips services → servers are on the ports they expect
2. Local dev with Docker → root specrun starts services → root sets `SPECRUN_NO_SERVICES=1` before subprocess invocations? No — the process adapter just inherits the parent env.

Problem: when the root spec manages services, the subprocesses need `SPECRUN_NO_SERVICES=1` too (they shouldn't try to start their own containers). The root spec's `startServices` should set `os.Setenv("SPECRUN_NO_SERVICES", "1")` after starting services, so all child processes inherit it.

**Step 3: Set env var after starting services**

In `startServices` (after `manager.Start` succeeds):
```go
os.Setenv("SPECRUN_NO_SERVICES", "1")
```

This way, any subprocess spawned by the process adapter automatically inherits the env var and skips service management.

**Step 4: Update CI workflow**

In `.github/workflows/ci.yml`, the self-verification step sets `SPECRUN_NO_SERVICES=1` (since CI starts servers manually):
```yaml
- name: Self-verification
  env:
    SPECRUN_NO_SERVICES: "1"
  run: ...
```

**Step 5: Update docs**

Update `CLAUDE.md`, `docs/services.md`, `skills/verify/SKILL.md` to reference `SPECRUN_NO_SERVICES` instead of `--no-services`.

**Step 6: Verify**

```bash
go test ./... -count=1
golangci-lint run ./...
# Run self-verification with env var:
SPECRUN_NO_SERVICES=1 SPECRUN_BIN=./specrun APP_URL=... ./specrun verify specs/speclang.spec
```

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor(cli): replace --no-services flag with SPECRUN_NO_SERVICES env var"
```

---

## Phase 5: Update tests

### Task 5: Update CLI tests

**Files:**
- Modify: `cmd/specrun/main_test.go`

**Step 1: Update test functions**

Tests currently call `runVerify([]string{...})` or build specrun and exec it. The exec-based tests should still work. Any tests calling internal functions directly need updating to match new signatures.

Update `TestVerify_HumanOutput` to account for colored output — when running in tests, `fatih/color` should auto-disable (no terminal), so the ANSI codes won't be present. Verify this.

Update `TestSelfVerification_Parse` to set `SPECRUN_NO_SERVICES=1` in the subprocess env instead of passing `--no-services` in args.

**Step 2: Add test for `--help`**

```go
func TestHelp(t *testing.T) {
	bin := specrunBin(t)
	cmd := exec.Command(bin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"parse", "generate", "verify", "install"} {
		if !strings.Contains(output, want) {
			t.Errorf("--help missing %q:\n%s", want, output)
		}
	}
}
```

**Step 3: Verify**

```bash
go test ./cmd/specrun/ -v
go test ./... -count=1
```

**Step 4: Commit**

```bash
git add cmd/specrun/main_test.go
git commit -m "test(cli): update tests for urfave/cli migration"
```

---

## Phase 6: Update specs and docs

### Task 6: Update self-verification specs

**Files:**
- Modify: `specs/cli_flags.spec` — remove `--no-services` from all `config.args`
- Modify: `specs/verify.spec` — same
- Modify: `specs/services.spec` — verify `--help` output if applicable

**Step 1: Remove `--no-services` from spec args**

In every scope that has `args: "verify --json --no-services ..."`, change to `args: "verify --json ..."`. The env var propagates automatically.

**Step 2: Add self-verification for help output** (optional)

Add a scope that verifies `specrun --help` exits 0 and produces output.

**Step 3: Run self-verification**

```bash
SPECRUN_NO_SERVICES=1 SPECRUN_BIN=./specrun APP_URL=http://localhost:8080 BROKEN_APP_URL=http://localhost:8081 HTTP_TEST_URL=http://localhost:8082 ECHO_TOOL_BIN=./echo_tool ./specrun verify specs/speclang.spec
```

ALL scenarios and invariants must pass.

**Step 4: Commit**

```bash
git add specs/
git commit -m "spec: update self-verification for CLI migration"
```

---

### Task 7: Update documentation

**Files:**
- Modify: `CLAUDE.md` — update Commands section (remove `--no-services`, add `SPECRUN_NO_SERVICES`)
- Modify: `docs/services.md` — replace `--no-services` references with env var
- Modify: `docs/getting-started.md` — update CLI examples if output format changed
- Modify: `skills/verify/SKILL.md` — replace `--no-services` with env var
- Modify: `skills/author/references/api_reference.md` — if it references CLI flags
- Modify: `README.md` — update verify example output if it changed (colored output won't show in README, but text format might differ)
- Modify: `.github/workflows/ci.yml` — ensure `SPECRUN_NO_SERVICES=1` is set

**Step 1: Update all docs**

Replace all `--no-services` references with `SPECRUN_NO_SERVICES=1`. Update any CLI output examples to match the new format.

**Step 2: Commit**

```bash
git add -A
git commit -m "docs: update documentation for CLI migration"
```

---

## Phase 7: Final verification and PR

### Task 8: Final verification and PR

**Step 1: Full checks**

```bash
golangci-lint run ./...       # zero issues
go test ./... -count=1        # all pass
go build -o ./specrun ./cmd/specrun
./specrun --help              # shows subcommands
./specrun verify --help       # shows flags
./specrun parse --help        # shows usage
```

**Step 2: Manual UX check**

```bash
# Colored output:
./specrun verify examples/transfer.spec
# No color when piped:
./specrun verify examples/transfer.spec | cat
# JSON output:
./specrun verify examples/transfer.spec --json
# Unknown command:
./specrun unknown
# Missing args:
./specrun verify
./specrun generate
```

**Step 3: Self-verification**

Start servers and run full self-verification.

**Step 4: Create PR**

```bash
git push -u origin <branch>
gh pr create --title "refactor(cli): migrate to urfave/cli v3 with colored output and live progress (#71)" --body "..."
gh pr checks <number> --watch
gh pr merge <number> --squash --delete-branch
```
