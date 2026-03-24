# Speclang Syntax Reference

## File Structure

```
include "<path>"                     # optional: top-level include

spec <Name> {

  description: "<text>"              # optional: AI context about the system

  target {
    base_url: env(APP_URL)           # plugin-dependent config
  }

  include "<path>"                   # spec-body include

  locators {                         # UI specs only: named element locators
    <name>: [<css-selector>]
  }

  model <Name> {
    <field>: <type>
    <field>: <type> { <constraint> }
  }

  action <name>(<params>) {          # reusable action sequence
    <plugin>.<verb>(<args>)
  }

  scope <name> {
    use <plugin>                     # required: which adapter drives this scope

    config {                         # opaque key-value pairs for adapter
      path: "/api/v1/resource"
      method: "POST"
    }

    contract {
      input {
        <field>: <type>
      }
      output {
        <field>: <type>
      }
    }

    invariant <name> {
      when <predicate>:              # optional guard
        <assertion>
    }

    scenario <name> {
      given { ... }                  # concrete values and/or action calls
      when { ... }                   # predicate (generative)
      then { ... }                   # assertions
    }
  }
}
```

**Important:** `use <plugin>` is declared at the **scope** level, not at spec level. Each scope independently declares which plugin drives it. This allows a single spec to mix plugins (e.g., one scope using `http` and another using `playwright`).

## Types

- `int` — integer
- `float` — floating-point number (e.g., `3.14`)
- `string` — string
- `bytes` — binary data (base64-encoded in JSON)
- `bool` — boolean
- `any` — untyped (passed through)
- `[]T` — array/slice of type T (e.g., `[]int`, `[]Account`)
- `map[K, V]` — map with key type K and value type V (e.g., `map[string, int]`)
- `enum("val1", "val2", ...)` — one of a fixed set of string values (e.g., `enum("http", "process", "playwright")`)
- `<ModelName>` — reference to a defined model
- Append `?` for optional: `string?`, `[]int?`, `enum("a", "b")?` (optional)

## Expressions

- **Literals**: `42`, `3.14`, `"hello"`, `true`, `false`, `null`
- **Field references**: `from.balance`, `output.error`
- **Environment**: `env(VAR)`, `env(VAR, "default")`
- **Objects**: `{ id: "alice", balance: 100 }`
- **Arrays**: `[expr, expr, ...]` — comma-separated list of expressions of the same type
- **Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `+`, `-`, `*`, `/`, `%`, `&&`, `||`, `!`
- **Functions**:
  - `len(expr)` — returns length of array, map, or string
  - `contains(haystack, needle)` — returns `bool`. String haystack + string needle: substring check. `[]any` haystack + any needle: element membership check.
  - `exists(expr)` — returns `true` if the path resolves to a value (including `null`), `false` if the path doesn't exist
  - `has_key(expr, "key")` — returns `true` if the map contains the specified key, `false` otherwise

## Comments

Lines starting with `#` are comments. Use them to explain intent.

```
# Money is neither created nor destroyed on successful transfers.
invariant conservation { ... }
```

## Three Scenario Types (Ascending Verification Strength)

### 1. `scenario` with `given` — Concrete smoke test

Fixed input values. Runs once. Documents expected behavior.

`then` assertions can use **relational expressions** that reference input fields. The expected value is computed from the input, not hardcoded. This makes assertions resistant to memorization — an LLM can't learn "the answer is 70" because the answer depends on the input.

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    from.balance: from.balance - amount   # 100 - 30 = 70
    to.balance: to.balance + amount       # 50 + 30 = 80
    error: null                           # literals still work
  }
}
```

### 2. `scenario` with `when` — Generative predicate

Defines a predicate over the input space. Runtime generates many matching inputs.

```
scenario overdraft {
  when {
    amount > from.balance
  }
  then {
    error: "insufficient_funds"
  }
}
```

### 3. `invariant` — Universal law

Must hold for ALL valid inputs. Optional `when` guard filters to a subspace.

```
invariant conservation {
  when error == null:
    output.from.balance + output.to.balance
      == input.from.balance + input.to.balance
}

invariant non_negative {
  output.from.balance >= 0
  output.to.balance >= 0
}
```

## Constraints

Model fields can have constraints that bound the generator:

```
model Transfer {
  amount: int { 0 < amount <= from.balance }
}
```

## Locators Block

UI specs declare named element locators at the spec level. Locator names are used in `given` blocks (as arguments to plugin actions) and in `then` blocks (with the `@plugin.property` assertion syntax).

```
locators {
  username_field: [data-testid=username]
  password_field: [data-testid=password]
  submit_btn:     [data-testid=submit]
  error_msg:      [data-testid=error]
  welcome:        [data-testid=welcome]
}
```

All locators must be pre-declared here. Inline selectors in action calls are not supported.

## `@plugin.property` Assertion Syntax

Use `locator@plugin.property: expected` in `then` blocks to assert on UI element state. The locator name is resolved from the spec's `locators` block to a CSS selector.

```
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
  error_msg@playwright.visible: false
}
```

This syntax is available to all adapters but is primarily used with `playwright`.

## Mixed `given` Block Syntax

`given` blocks accept both **data assignments** and **action calls**, interleaved in any order. Steps execute in the order written. This works with any adapter that supports action calls (Playwright and HTTP).

```
# Playwright example
given {
  playwright.fill(username_field, "alice")   # action call
  playwright.fill(password_field, "secret")  # action call
  user: "alice"                               # data assignment
  pass: "secret"                              # data assignment
  playwright.click(submit_btn)               # action call
}

# HTTP multi-step example
given {
  http.header("Authorization", "Bearer token")   # set persistent header
  http.post("/api/items", { name: "widget" })     # POST to create
  http.get("/api/items/1")                        # GET to verify
}
```

Data assignments populate the input context for assertion evaluation. Action calls execute against the adapter immediately. For the HTTP adapter, headers and cookies persist across calls within a scenario, and `then` assertions apply to the last response.

You can also call named actions defined in `action` blocks:

```
action login(user, pass) {
  playwright.fill(username_field, user)
  playwright.fill(password_field, pass)
  playwright.click(submit_btn)
  playwright.wait(welcome)
}

scope login {
  use playwright
  scenario successful_login {
    given {
      login("alice", "secret")   # invoke named action
    }
    then {
      welcome@playwright.visible: true
    }
  }
}
```

Array literals can be used in `given` blocks to assign to array fields:

```
scenario process_batch {
  given {
    ids: [1, 2, 3]
    names: ["alice", "bob", "charlie"]
  }
  then {
    count: 3
  }
}
```

## Include Directive

Split specs across files. Paths are relative to the including file.

```
include "models/account.spec"
include "scopes/transfer.spec"
```

## Import Directive

Import models and scopes from external schema files. Supports OpenAPI 3.x and protobuf.

### OpenAPI

```
import openapi("schema.yaml")
```

Generates models from `components/schemas` and scopes from `paths`. Type mapping: `integer` → `int`, `string` → `string`, `boolean` → `bool`, `$ref` → model name. Constraints (`minimum`/`maximum`) are converted to field constraint expressions.

### Protobuf

```
import proto("service.proto")
```

Generates models from `message` definitions and scopes from unary `rpc` methods. Type mapping: all integer types → `int`, `string` → `string`, `bool` → `bool`, message reference → model name.

Unsupported types (array, float, enum, bytes, map) are skipped with a warning in both importers. Imported scopes have config and contracts populated but no invariants or scenarios — those are hand-authored.

## Validation

Specs are automatically validated after parsing, before generation and verification begin. Validation checks:

- **Model resolution**: All model references exist and are well-defined
- **Type checking**: Literal values match their declared types (e.g., assigning a string to an `int` field)
- **Array element types**: All elements in array literals match the array's element type
- **Object field validation**: Object literals contain only declared fields with matching types
- **Given completeness**: All required contract input fields are assigned in `given` blocks
- **Then field validation**: All fields in `then` blocks are valid model fields or locator assertions

**Validation failures are hard errors:** They cause `specrun` to exit with code 1 and print detailed messages. Verification does not proceed if validation fails — the user sees validation errors first.

Example error output:

```
error validating spec:
  scope transfer / scenario success:
    given: required field "to" not assigned
    error: type mismatch: expected int, got string "pending"
```

Error messages are hierarchical: `scope / scenario` or `scope / invariant` for context, then the specific error.

### `use http`

For HTTP APIs. Target uses `base_url`.

**Single-request scopes**: Scope config uses `path` and `method`. All `given` assignments become the request body.

**Multi-step scopes**: Use action calls in `given` blocks. No `path`/`method` config needed — each call specifies its own path. Headers and cookies persist across calls. `then` assertions apply to the last response.

#### Actions

| Action | Args | Description |
|--------|------|-------------|
| `http.get(path)` | URL path | GET request |
| `http.post(path, body)` | URL path + JSON body | POST request |
| `http.put(path, body)` | URL path + JSON body | PUT request |
| `http.delete(path)` | URL path | DELETE request |
| `http.header(name, value)` | header name + value | Set persistent header |

#### Assertions

| Property | Type | Description |
|----------|------|-------------|
| `status` | `int` | HTTP status code |
| `body` | `any` | Full response body |
| `header.<name>` | `string` | Response header |
| `<field.path>` | `any` | Dot-path into JSON body |

#### Multi-step Example

```
scope create_and_verify {
  use http

  contract {
    input { name: string }
    output { id: int, name: string }
  }

  scenario create_then_get {
    given {
      name: "widget"
      http.post("/api/resources", { name: "widget" })
      http.get("/api/resources/1")
    }
    then {
      status: 200
      id: 1
      name: "widget"
    }
  }

  scenario with_auth_header {
    given {
      http.header("Authorization", "Bearer token")
      http.get("/api/protected")
    }
    then {
      status: 200
    }
  }
}
```

### `use process`

For CLI tools. Runs subprocesses, captures exit code/stdout/stderr. Target uses `command`. Scope config uses `args`.

Dot-paths in `then` blocks support numeric array indexing. `stdout.items.0.name` accesses the first element of the `items` array in the parsed JSON response, then its `name` field. Out-of-range indices produce an assertion failure. This also applies to the `http` adapter when asserting against JSON response bodies.

### `use playwright`

For browser UI testing. Controls a real browser via Playwright. Target uses `base_url`, `headless` (default `"true"`), and `timeout` (milliseconds, default `"5000"`). Scope config uses `url` (the page path to navigate to).

Requires Playwright browsers to be installed: `npx playwright install chromium`

#### Actions

| Action | Args | Description |
|--------|------|-------------|
| `playwright.goto(url)` | URL string | Navigate (prepends `base_url` if relative) |
| `playwright.click(locator)` | locator name | Click element |
| `playwright.fill(locator, value)` | locator name + text | Clear and type into input |
| `playwright.type(locator, value)` | locator name + text | Append text (no clear) |
| `playwright.select(locator, value)` | locator name + option | Select dropdown option |
| `playwright.check(locator)` | locator name | Check checkbox |
| `playwright.uncheck(locator)` | locator name | Uncheck checkbox |
| `playwright.wait(locator)` | locator name | Wait for element to be visible |
| `playwright.new_page()` | — | Create a fresh browser page |
| `playwright.close_page()` | — | Close current page |
| `playwright.clear_state()` | — | Clear cookies and localStorage |

#### Assertions

| Property | Type | Description |
|----------|------|-------------|
| `visible` | `bool` | Element is visible |
| `text` | `string` | Text content |
| `value` | `string` | Input field value |
| `checked` | `bool` | Checkbox state |
| `disabled` | `bool` | Whether element is disabled |
| `count` | `int` | Number of matching elements |
| `attribute.<name>` | `string` | Named attribute value (e.g., `attribute.href`) |

#### Full Example

```
spec LoginApp {
  description: "Login UI verification"

  target {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
    timeout: "5000"
  }

  locators {
    username_field: [data-testid=username]
    password_field: [data-testid=password]
    submit_btn:     [data-testid=submit]
    welcome:        [data-testid=welcome]
    error_msg:      [data-testid=error]
  }

  action login(user, pass) {
    playwright.fill(username_field, user)
    playwright.fill(password_field, pass)
    playwright.click(submit_btn)
  }

  scope login {
    use playwright

    config {
      url: "/login"
    }

    contract {
      input {
        user: string
        pass: string
      }
      output {
        ok: bool
      }
    }

    scenario successful_login {
      given {
        login("alice", "secret")
        user: "alice"
      }
      then {
        welcome@playwright.visible: true
        welcome@playwright.text: "Welcome, alice"
        error_msg@playwright.visible: false
      }
    }

    scenario invalid_credentials {
      when {
        pass != "secret"
      }
      then {
        error_msg@playwright.visible: true
      }
    }

    # The welcome banner must never appear when login fails.
    invariant no_welcome_on_failure {
      when ok == false:
        welcome@playwright.visible: false
    }
  }
}
```
