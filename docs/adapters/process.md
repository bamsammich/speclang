# Process Adapter

The process adapter (`use process`) executes subprocesses and asserts against their output. It mirrors the HTTP adapter's pattern.

## Configuration

### Target Block

| Key | Required | Description |
|-----|----------|-------------|
| `command` | Yes | Binary to run. Supports `env()` expressions and `service()` references. |

```
target {
  command: "./my-binary"
}
```

### Scope Config

| Key | Description |
|-----|-------------|
| `args` | Base arguments prepended to every exec call (space-separated) |

```
config {
  args: "verify --json"
}
```

## Action: `exec`

Runs `command [...args] [...input_fields]`. Captures:

- Exit code
- Stdout (best-effort JSON parse; raw string if not JSON)
- Stderr (raw string)

## Assertions

| Property | Type | Description |
|----------|------|-------------|
| `exit_code` | `int` | Process exit code |
| `stdout` | `any` | Full stdout (parsed JSON or raw string) |
| `stdout.<field.path>` | `any` | Dot-path into parsed JSON stdout |
| `stdout.<field>.<N>.<field>` | `any` | Array index in dot-path (zero-based) |
| `stderr` | `string` | Raw stderr output |

## Examples

### Testing a CLI Tool

```
spec CLI {
  target {
    command: "./mytool"
  }

  scope help {
    use process

    config {
      args: "--help"
    }

    contract {
      input {}
      output {
        ok: bool
      }
    }

    scenario shows_help {
      given {}
      then {
        exit_code: 0
      }
    }
  }
}
```

### Testing JSON Output

```
spec Parser {
  target {
    command: "./specrun"
  }

  scope parse_valid {
    use process

    config {
      args: "parse"
    }

    contract {
      input {
        spec_file: string
      }
      output {
        name: string
      }
    }

    scenario parses_spec {
      given {
        spec_file: "examples/transfer.spec"
      }
      then {
        exit_code: 0
        stdout.name: "AccountAPI"
      }
    }
  }
}
```

### Array Index Access

Dot-paths into stdout JSON support numeric segments for array indexing:

```
then {
  stdout.items.0.name: "first"
  stdout.scopes.0.name: "transfer"
}
```

Out-of-range indices produce an assertion failure. This is the same behavior as the HTTP adapter.
