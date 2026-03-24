# Self-Verification

Speclang verifies itself using its own specification language. This is black-box verification through the runtime -- speclang is both the verifier and the system under test.

## The Concept

The self-verification specs use the **process adapter** to invoke `specrun` subcommands as a subprocess and verify their behavior. The specs treat `specrun` as an opaque binary, asserting only on its outputs (exit code, stdout, stderr) without knowledge of its internals.

The root spec (`specs/speclang.spec`) declares services for `transfer_server`, `broken_server`, and `http_test_server` in its `target` block. When Docker is available, these containers are managed automatically. In CI or when Docker is unavailable, servers are started manually and `SPECRUN_NO_SERVICES=1` is set so subprocess invocations skip service management.

This creates a bootstrapping loop: `specrun verify specs/speclang.spec` launches `specrun` to verify specs that themselves invoke `specrun` subcommands. The outer instance orchestrates; the inner instances are the system under test.

## Current Coverage

The self-verification suite includes **63 scenarios** and **19 invariants** across these spec files:

| Spec file | Scopes | What it verifies |
|-----------|--------|------------------|
| `parse.spec` | `parse_valid`, `parse_invalid`, `parse_validation` | Parser accepts valid specs, rejects malformed ones, validates types |
| `import.spec` | `import_openapi_*`, `import_proto_*` | OpenAPI and protobuf imports produce correct models and scopes |
| `generate.spec` | `generate` | Generator produces constraint-satisfying outputs |
| `generate_types.spec` | `generate_types` | Generator handles all types (float, bytes, arrays, maps, optionals) |
| `types.spec` | `types` | Type system parsing and generation for extended types |
| `enum.spec` | `enum` | Enum type parsing, generation, and variant validation |
| `exists.spec` | `exists` | `exists()` and `has_key()` function behavior |
| `error_assertions.spec` | `error_assertions` | Error pseudo-field parsing and verification |
| `verify.spec` | `verify_pass` | Correct implementations pass verification |
| `verify_fail.spec` | `verify_fail` | Incorrect implementations are detected |
| `shrinking.spec` | `shrinking` | Counterexample shrinking produces minimal values |
| `adapters.spec` | `adapter_fixtures` | HTTP and process adapter fixture tests pass |
| `cli_flags.spec` | `cli_flags_*` | CLI flag parsing (seed, iterations, json, errors) |
| `services.spec` | `verify_service_lifecycle`, `parse_service_ref`, `invalid_service_ref` | Service lifecycle, service ref parsing, validation errors |

### Shrinking Specs

The shrinking specs verify behavioral quality -- that counterexamples are minimized to boundary values (ints near zero, empty strings, zero balances) rather than testing the shrinking algorithm's implementation details. These are performance specs in the sense that they verify the quality of output, not the mechanics of how it's produced.

## Running Self-Verification

Build specrun first, then run:

```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

The `SPECRUN_BIN` environment variable tells the process adapter which binary to invoke for inner `specrun` calls. Without it, the adapter uses the system-installed `specrun`.

Sample output:

```
verifying specs/speclang.spec (Speclang) with seed=42, iterations=100

  scope parse_valid:
    ✓ scenario minimal_spec
    ✓ scenario transfer_spec
    ✓ scenario openapi_import
    ...

  scope generate:
    ✓ invariant produces_output (100 inputs)
    ✓ invariant constraints_satisfied (100 inputs)

  scope verify_pass:
    ✓ scenario transfer_spec_passes
    ...

Scenarios:  63/63 passed
Invariants: 19/19 passed
```

## The Bootstrapping Pattern

Self-verification creates a trust hierarchy:

1. **Go tests** (`go test ./...`) verify the parser, generator, and adapters at the unit level
2. **Self-verification specs** verify the assembled system end-to-end through the CLI
3. **The same runtime** that runs the specs is the runtime being tested

This means a bug in the runtime could theoretically hide itself during self-verification. The Go unit tests serve as the independent ground truth that prevents this -- they don't go through the spec runtime at all.

The practical value of self-verification is regression detection: if a change breaks spec parsing, generation, or verification, the self-verification suite catches it through a different execution path than the unit tests.
