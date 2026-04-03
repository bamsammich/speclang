# Migrating from v2 to v3

## Why v3

v3 is a breaking redesign of the speclang syntax. Every change was motivated by real friction encountered writing and reading v2 specs. This section explains why each break was necessary.

### 1. Mixed adapters per scope

v2 bound each scope to a single adapter via `use http` or `use playwright`. Real behaviors cross adapter boundaries: a login flow hits an HTTP API for authentication then checks a Playwright UI for the dashboard. v2 forced you to split this into separate scopes with separate contracts, losing the ability to express "login" as one cohesive behavior.

v3 removes `use` entirely. Adapters are named inline per call (`http.post(...)`, `playwright.click(...)`), so a single scope can mix HTTP, Playwright, and process calls freely.

### 2. Uniform syntax

v2 had three inconsistent patterns for similar operations:

| v2 pattern | Used for | Example |
|-----------|----------|---------|
| `locator@plugin.property: value` | Plugin assertions | `error_msg@playwright.visible: true` |
| `plugin.method(locator, args)` | Plugin actions | `playwright.fill(username_field, "alice")` |
| `field: value` | Equality assertions | `error: null` |

The `@` sigil, the `:` for equality, and the locator indirection were all different syntactic mechanisms for conceptually similar things. v3 reduces everything to three patterns:

| v3 pattern | Used for | Example |
|-----------|----------|---------|
| `name: value` | Assignment | `from: { id: "alice", balance: 100 }` |
| `adapter.method(args)` | Action call | `playwright.fill('[data-testid="email"]', "alice")` |
| `expr op expr` | Assertion | `playwright.visible('[data-testid="welcome"]') == true` |

### 3. Variables

v2 had no way to capture action results. Passing state between steps relied on implicit, adapter-specific hacks like `body.access_token` -- a magic field reference that only worked with the HTTP adapter and only referred to the most recent response. There was no way to hold multiple results or name intermediate values.

v3 adds `let` bindings:

```speclang
let result = http.post("/api/auth/login", { username: "admin" })
let token = result.body.access_token
http.header("Authorization", "Bearer " + token)
```

Bindings are immutable and scoped to the block they appear in.

### 4. Custom reusable actions

v2 actions were primitive: untyped parameters, no return values, no composition. v3 actions have typed parameters, can contain `let` and `return`, and are callable from `before`, `given`, and as contract actions:

```speclang
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}
```

### 5. Legibility for humans and LLMs

Every v2 removal was driven by readability:

- **No `locators` indirection** -- selectors appear where they're used, so you don't need to cross-reference a block at the top of the file
- **No `@` sigil** -- plugin assertions use the same `adapter.method(args)` call syntax as actions
- **No `:` for equality** -- `==` is unambiguous; `:` looked like assignment
- **Explicit adapter naming** -- `http.post(...)` is self-documenting; `use http` + `config { method: "POST" }` required reading two blocks to understand what happens

### 6. Contract-action binding

v2 coupled contracts to adapters implicitly via `use` + `config`. The contract declared inputs and outputs, but the execution recipe was hidden in adapter configuration. v3 makes this explicit:

```speclang
scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }

  contract {
    input { ... }
    output { ... }
    action: transfer   # explicit reference to the execution recipe
  }
}
```

The runtime matches contract input fields to action parameters by name. Signature mismatches are caught at compile time.

---

## Migration Guide

### 1. Target block to adapter config blocks

v2 had a single `target` block shared across all adapters. v3 uses namespaced adapter config blocks.

**v2:**
```speclang
target {
  base_url: env(APP_URL, "http://localhost:8080")
}
```

**v3:**
```speclang
http {
  base_url: env(APP_URL, "http://localhost:8080")
}
```

If your spec uses multiple adapters, declare a block for each:

```speclang
http {
  base_url: env(APP_URL, "http://localhost:8080")
}
playwright {
  base_url: env(APP_URL, "http://localhost:8080")
  headless: true
}
```

Services remain the same but move to spec-level:

```speclang
services {
  app {
    build: "./server"
    port: 8080
  }
}
```

### 2. Remove locators block

v2 required pre-declaring all CSS selectors in a `locators` block. v3 uses inline string selectors.

**v2:**
```speclang
locators {
  username_field: [data-testid=username]
  submit_btn:     [data-testid=submit]
  error_msg:      [data-testid=error]
}

# Later, in actions/assertions:
playwright.fill(username_field, "alice")
error_msg@playwright.visible: true
```

**v3:**
```speclang
# Selectors inline where used:
playwright.fill('[data-testid="username"]', "alice")
playwright.visible('[data-testid="error"]') == true
```

Single-quoted strings were added for CSS selectors containing double quotes.

### 3. `use` + `config` to scope-level action + contract `action:`

v2 scopes declared their adapter and configuration separately. v3 replaces this with an explicit action that calls the adapter.

**v2:**
```speclang
scope transfer {
  use http
  config {
    path: "/api/v1/accounts/transfer"
    method: "POST"
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int { 0 < amount <= from.balance }
    }
    output {
      from: Account
      to: Account
      error: string?
    }
  }
}
```

**v3:**
```speclang
scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int { 0 < amount <= from.balance }
    }
    output {
      from: Account
      to: Account
      error: string?
    }
    action: transfer
  }
}
```

### 4. Plugin assertions: `@` syntax to method calls

**v2:**
```speclang
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
  items@playwright.count >= 1
}
```

**v3:**
```speclang
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Welcome, alice"
  playwright.count('[data-testid="items"]') >= 1
}
```

### 5. Equality assertions: `:` to `==`

**v2:**
```speclang
then {
  from.balance: from.balance - amount
  error: null
  status: 200
}
```

**v3:**
```speclang
then {
  from.balance == from.balance - amount
  error == null
  status == 200
}
```

### 6. Multi-step state passing with `let`

**v2** used implicit adapter state:
```speclang
before {
  http.post("/auth/login", { token: "test" })
  http.header("Authorization", "Bearer " + body.access_token)
}
```

**v3** uses explicit `let` bindings:
```speclang
before {
  let result = http.post("/auth/login", { token: "test" })
  http.header("Authorization", "Bearer " + result.body.access_token)
}
```

### 7. New features

**`let` bindings** -- immutable variables scoped to their block:
```speclang
let session = login("admin", "test")
let token = session.access_token
```

**`return` in actions** -- actions can return values:
```speclang
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  return result.body
}
```

**Single-quoted strings** -- for selectors containing double quotes:
```speclang
playwright.fill('[data-testid="email-input"]', "alice@example.com")
```

**Scope-level actions** -- private to the scope they're defined in:
```speclang
scope checkout {
  action place_order(item_id: string, qty: int) {
    let result = http.post("/api/orders", { item_id: item_id, quantity: qty })
    return result.body
  }
}
```

---

## Complete Before/After Example

### v2: HTTP API + Playwright UI

```speclang
spec MyApp {
  description: "E-commerce app with API and UI"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
    headless: "true"
  }

  locators {
    email_input:    [data-testid=email]
    password_input: [data-testid=password]
    submit_btn:     [data-testid=submit]
    welcome_msg:    [data-testid=welcome]
    error_msg:      [data-testid=error]
    cart_count:     [data-testid=cart-count]
  }

  model User {
    id: string
    email: string
    role: enum("admin", "user")
  }

  model CartItem {
    product_id: string
    quantity: int { quantity > 0 }
  }

  # v2 actions: untyped, no return
  action login(email, password) {
    playwright.fill(email_input, email)
    playwright.fill(password_input, password)
    playwright.click(submit_btn)
  }

  # Scope 1: HTTP-only for the API
  scope add_to_cart {
    use http
    config {
      path: "/api/cart/add"
      method: "POST"
    }

    before {
      http.post("/auth/login", { email: "alice@test.com", password: "secret" })
      http.header("Authorization", "Bearer " + body.access_token)
    }

    contract {
      input {
        product_id: string
        quantity: int { quantity > 0 }
      }
      output {
        cart_size: int
        error: string?
      }
    }

    scenario add_one {
      given {
        product_id: "widget-1"
        quantity: 1
      }
      then {
        cart_size: 1
        error: null
      }
    }
  }

  # Scope 2: Playwright-only for the UI login
  scope ui_login {
    use playwright

    contract {
      input {
        email: string
        password: string
      }
      output {
        error: string?
      }
    }

    scenario valid_login {
      given {
        email: "alice@test.com"
        password: "secret"
        login(email, password)
      }
      then {
        welcome_msg@playwright.visible: true
        welcome_msg@playwright.text: "Hello, alice@test.com"
        error_msg@playwright.visible: false
      }
    }
  }
}
```

### v3: Same behavior, unified

```speclang
spec MyApp {
  description: "E-commerce app with API and UI"

  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:8080")
    headless: true
  }

  model User {
    id: string
    email: string
    role: enum("admin", "user")
  }

  model CartItem {
    product_id: string
    quantity: int { quantity > 0 }
  }

  # Spec-level action: typed, returns a value
  action login(email: string, password: string) {
    let result = http.post("/api/auth/login", { email: email, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
    return result.body
  }

  # Spec-level action: UI login via Playwright
  action ui_login(email: string, password: string) {
    playwright.fill('[data-testid="email"]', email)
    playwright.fill('[data-testid="password"]', password)
    playwright.click('[data-testid="submit"]')
  }

  scope add_to_cart {
    action add_to_cart(product_id: string, quantity: int) {
      let result = http.post("/api/cart/add", {
        product_id: product_id, quantity: quantity
      })
      return result.body
    }

    before {
      let session = login("alice@test.com", "secret")
    }

    contract {
      input {
        product_id: string
        quantity: int { quantity > 0 }
      }
      output {
        cart_size: int
        error: string?
      }
      action: add_to_cart
    }

    scenario add_one {
      given {
        product_id: "widget-1"
        quantity: 1
      }
      then {
        cart_size == 1
        error == null
      }
    }
  }

  # Now a single scope can use both HTTP and Playwright
  scope ui_login {
    action authenticate(email: string, password: string) {
      ui_login(email, password)
    }

    contract {
      input {
        email: string
        password: string
      }
      output {
        error: string?
      }
      action: authenticate
    }

    scenario valid_login {
      given {
        email: "alice@test.com"
        password: "secret"
      }
      then {
        playwright.visible('[data-testid="welcome"]') == true
        playwright.text('[data-testid="welcome"]') == "Hello, alice@test.com"
        playwright.visible('[data-testid="error"]') == false
      }
    }
  }
}
```

Key differences:

- **No `locators` block** -- selectors are inline strings
- **No `use` or `config`** -- each scope defines an action that calls adapters directly
- **`let` captures state** -- `login()` returns a value, no implicit `body.access_token`
- **`==` for assertions** -- no `:` ambiguity
- **Mixed adapters** -- `ui_login` scope could call both `http.post(...)` and `playwright.click(...)` if needed

---

## Automated Migration

`specrun migrate` performs mechanical v2-to-v3 transformations. It handles the syntactic changes but may require manual review for action extraction and `let` bindings.

### Usage

```bash
specrun migrate v2-spec.spec           # prints v3 to stdout
specrun migrate v2-spec.spec -w        # writes in place
specrun migrate specs/*.spec -w        # batch migrate
```

### What it does automatically

| Transformation | Automated |
|---------------|-----------|
| `target { base_url: ... }` to `http { base_url: ... }` | Yes |
| `locators` removal + inline selectors | Yes |
| `:` to `==` in `then` blocks | Yes |
| `@plugin.property: value` to `plugin.method('selector') == value` | Yes |
| `use` + `config` to scope-level action | Yes |
| Contract `action:` addition | Yes |
| Implicit `body.x` to `let result = ...; result.body.x` | Best-effort |

### What needs manual review

- **Action extraction** -- the tool generates a scope-level action from `use` + `config`, but you may want to refactor common patterns into spec-level actions
- **`let` bindings in `before`** -- implicit state references (`body.access_token`) are converted to explicit `let` bindings, but complex multi-step sequences may need restructuring
- **Mixed-adapter scopes** -- if you had separate v2 scopes that should merge into one v3 scope, that's a manual refactor

### Workflow

```bash
# 1. Migrate
specrun migrate my-spec.spec -w

# 2. Review the output
git diff my-spec.spec

# 3. Verify
specrun verify my-spec.spec

# 4. Manual cleanup if needed
```
