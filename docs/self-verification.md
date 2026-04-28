# Self-Verification

Speclang verifies itself using its own specification language. This is black-box verification through the runtime — speclang is both the verifier and the system under test.

## The Concept

The self-verification specs use the **process adapter** to invoke `specrun` subcommands as a subprocess and verify their behavior. The specs treat `specrun` as an opaque binary, asserting only on its outputs (exit code, stdout, stderr) without knowledge of its internals.

The root spec (`specs/speclang.spec`) declares `services` blocks for `transfer_server`, `broken_server`, and `http_test_server`. These containers are managed automatically by `specrun verify` via Docker. `specs/speclang.spec` is a genuine root spec — it uses `include` for shared library fragments (the process adapter config, shared model definitions) that the individual spec files need, not to pull in the other runnable spec files. The individual spec files (`parse.spec`, `generate.spec`, etc.) are run independently via glob.

This creates a bootstrapping loop: `specrun verify specs/speclang.spec` launches `specrun` to verify specs that themselves invoke `specrun` subcommands. The outer instance orchestrates; the inner instances are the system under test.

## Running Self-Verification

Build specrun first, then run:

```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

The `SPECRUN_BIN` environment variable tells the process adapter which binary to invoke for inner `specrun` calls. Without it, the adapter looks for `specrun` on `PATH`.

In v4, `specrun verify` also accepts a glob:

```bash
SPECRUN_BIN=./specrun ./specrun verify specs/*.spec
```

This verifies each spec file in `specs/` independently. Files with no contracts (e.g., shared model includes) log a skip notice and exit 0.

## Current Coverage

The self-verification suite spans these spec files in `specs/`:

| Spec file | Scopes | What it verifies |
|-----------|--------|------------------|
| `parse.spec` | `parse_valid`, `parse_invalid`, `parse_validation` | Parser accepts valid specs, rejects malformed ones, validates types |
| `import.spec` | `import_openapi_*`, `import_proto_*` | OpenAPI and protobuf imports produce correct models and contracts |
| `generate.spec` | `generate` | Generator produces constraint-satisfying outputs |
| `generate_types.spec` | `generate_types` | Generator handles all types (float, bytes, arrays, maps, optionals) |
| `types.spec` | `types` | Type system parsing and generation for extended types |
| `enum.spec` | `enum` | Enum type parsing, generation, and variant validation |
| `exists.spec` | `exists` | `exists()` and `has_key()` function behavior |
| `error_assertions.spec` | `error_assertions` | Error pseudo-field parsing and verification |
| `verify.spec` | `verify_pass` | Correct implementations pass verification |
| `verify_fail.spec` | `verify_fail` | Incorrect implementations are detected |
| `shrinking.spec` | `shrinking` | Counterexample shrinking produces minimal values |
| `adapters.spec` | `verify_http_adapter`, `verify_process_adapter` | HTTP and process adapter fixture tests pass end-to-end |
| `cli_flags.spec` | `cli_flags_*` | CLI flag parsing (seed, iterations, json, errors) |
| `services.spec` | `verify_service_lifecycle`, `parse_service_ref`, `invalid_service_ref` | Service lifecycle, service ref parsing, validation errors |
| `expressions.spec` | `env_in_config`, others | `env()` expressions in config and given blocks |
| `v4_features.spec` | `parse_named_enum`, `generate_named_enum`, `parse_in_operator`, `parse_implies`, `parse_underscore_numeric`, `parse_contract_inheritance`, `parse_config_block`, `parse_state_dependent_fields`, `parse_no_wrapper` | All v4-specific language features |
| `glob.spec` | `verify_glob_simple`, `verify_glob_recursive`, `verify_glob_no_contracts`, `verify_glob_no_match` | Glob CLI: multi-file verification, recursive glob, no-contracts skip, zero-match error |

## v4-Specific Coverage (in `v4_features.spec`)

These scopes verify features that are new in v4:

**Named enums** (`parse_named_enum`, `generate_named_enum`): `enum Role { admin, user, viewer }` at top level parses correctly; the generator only produces declared variants for a named-enum field (`output.role == "admin" or output.role == "user" or output.role == "viewer"`).

**`in` operator** (`parse_in_operator`): `status in (OrderStatus.pending, OrderStatus.cancelled)` in `when` predicates and `in` as a binary comparison operator in invariants.

**`implies` operator** (`parse_implies`): `output.error == null implies output.from.balance + output.to.balance == from.balance + to.balance` — lowest-precedence logical implication.

**Underscore numeric separators** (`parse_underscore_numeric`): `1_000_000`, `500_000` parse as their underlying integer values.

**Contract model inheritance** (`parse_contract_inheritance`): `contract Name: InputModel -> OutputModel { constrain { ... } }` — the `Inherits` field and `constrain` block are present in the AST.

**Config block** (`parse_config_block`): Top-level `config { key: expr }` parses correctly; `config.key` references resolve.

**State-dependent fields** (`parse_state_dependent_fields`): `tracking: string when status == "shipped"` — `Field.When` is populated in the AST.

**No-wrapper top-level declarations** (`parse_no_wrapper`): Files with top-level models, enums, and contracts but no `spec Name { }` wrapper parse correctly. This is the fundamental v4 structural change.

## Glob CLI Coverage (in `glob.spec`)

These scopes verify the v4 glob execution behavior:

**`verify_glob_simple`**: A simple `*.spec` glob matches multiple files and all pass. Model-only files (no contracts) are skipped, not failed.

**`verify_glob_recursive`**: A `**/*.spec` recursive glob finds files in nested directories.

**`verify_glob_no_contracts`**: A file with no contracts exits 0 (logged as skipped, not an error).

**`verify_glob_no_match`**: A glob matching zero files exits 1.

## Shrinking Specs

The shrinking specs verify behavioral quality — that counterexamples are minimized to boundary values (ints near zero, empty strings, zero balances) rather than testing the shrinking algorithm's implementation. These are performance specs in the sense that they verify the quality of output, not the mechanics of how it's produced.

## The Bootstrapping Pattern

Self-verification creates a trust hierarchy:

1. **Go tests** (`go test ./...`) verify the parser, generator, runner, and adapters at the unit level
2. **Self-verification specs** verify the assembled system end-to-end through the CLI
3. **The same runtime** that runs the specs is the runtime being tested

This means a bug in the runtime could theoretically hide itself during self-verification. The Go unit tests serve as the independent ground truth that prevents this — they don't go through the spec runtime at all.

The practical value of self-verification is regression detection: if a change breaks spec parsing, generation, or verification, the self-verification suite catches it through a different execution path than the unit tests.

## Sample Output

```
Verifying speclang.spec (seed=42, iterations=100)

  scope parse_valid:
    ✓ scenario minimal_spec
    ✓ scenario transfer_spec
    ✓ scenario openapi_import
    ...

  scope parse_named_enum:
    ✓ scenario named_enum_spec

  scope generate_named_enum:
    ✓ invariant produces_output (100 inputs)
    ✓ invariant role_is_valid_variant (100 inputs)

  scope verify_glob_simple:
    ✓ scenario glob_matches_pass

  scope verify_pass:
    ✓ scenario transfer_spec_passes
    ...

Scenarios:  N/N passed
Invariants: N/N passed
```
