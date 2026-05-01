# Process Adapter

The process adapter executes subprocesses and asserts against their output. It is built into `specrun` — no external binary required. This adapter is also how speclang's self-verification suite works (it invokes `specrun` subcommands as a subprocess).

## Configuration

Configure at the top level of the spec file:

```
process {
  command: "./my-binary"
}
```

| Key | Required | Description |
|-----|----------|-------------|
| `command` | Yes | Binary to run. Supports `env()` expressions. |

## Action: `process.exec`

```
process.exec(arg, arg, ...)
```

Arguments are joined with the configured `command` and executed as a subprocess. Captures:

- Exit code
- Stdout (best-effort JSON parse; raw string if not JSON)
- Stderr (raw string)

## Assertions

Assertions reference `output.*` in `then` blocks and invariants:

| Property | Type | Description |
|----------|------|-------------|
| `output.exit_code` | `int` | Process exit code |
| `output.stdout` | `any` | Full stdout (parsed JSON or raw string) |
| `output.stdout.<field.path>` | `any` | Dot-path into parsed JSON stdout |
| `output.stdout.items.0.name` | `any` | Array index in dot-path (zero-based) |
| `output.stderr` | `string` | Raw stderr output |

## Examples

### Testing a CLI Tool

```
process {
  command: env(MYCLI_BIN, "./mytool")
}

scope help {
  contract HelpContract() -> HelpResult {
    action {
      return process.exec("--help")
    }

    scenario shows_help {
      given {}
      then { output.exit_code == 0 }
    }
  }
}

model HelpResult {
  exit_code: int
  stdout: any
  stderr: string
}
```

### Testing JSON Output

```
process {
  command: env(SPECRUN_BIN, "./specrun")
}

model ParseResult {
  exit_code: int
  scopes: any
}

scope parse_valid {
  contract ParseValidContract(file: string) -> ParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    scenario parses_transfer_spec {
      given { file: "examples/transfer.spec" }
      then {
        output.exit_code == 0
        output.scopes != null
      }
    }
  }
}
```

### Array Index Access

Dot-paths into stdout JSON support numeric segments for array indexing:

```
then {
  output.stdout.items.0.name == "first"
  output.stdout.scopes.0.name == "transfer"
}
```

Out-of-range indices produce an assertion failure.

### Invariant Over Generated Inputs

```
process {
  command: env(SPECRUN_BIN, "./specrun")
}

model VerifyResult {
  exit_code: int
}

scope verify_pass {
  contract VerifyPassContract(file: string) -> VerifyResult {
    action {
      return process.exec("verify", "--json", file)
    }

    # All parseable specs must exit 0 when verified.
    invariant verify_succeeds {
      output.exit_code == 0
    }
  }
}
```

## Self-Verification Pattern

The speclang self-verification suite uses the process adapter to invoke `specrun` as a black-box binary. This is the canonical pattern for verifying any CLI tool:

```
process {
  command: env(SPECRUN_BIN, "./specrun")
}

model ParseResult {
  exit_code: int
}

scope parse_valid {
  contract ParseValidContract(file: string) -> ParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    scenario minimal_spec {
      given { file: "testdata/self/minimal.spec" }
      then { output.exit_code == 0 }
    }
  }
}
```

Run with:
```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```
