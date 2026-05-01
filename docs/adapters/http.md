# HTTP Adapter

The HTTP adapter tests REST APIs. It is built into `specrun` — no subprocess or external binary required.

## Configuration

Configure at the top level of the spec file (not inside a `target` block):

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}
```

With a service reference:

```
services {
  app { build: "./server", port: 8080, health: "/healthz" }
}

http {
  base_url: service(app)
}
```

| Key | Required | Description |
|-----|----------|-------------|
| `base_url` | Yes | API base URL. Supports `env()` expressions and `service()` references. |

## Actions

| Action | Args | Description |
|--------|------|-------------|
| `http.get(path)` | URL path | GET request |
| `http.post(path, body)` | URL path + JSON body | POST request |
| `http.put(path, body)` | URL path + JSON body | PUT request |
| `http.delete(path)` | URL path | DELETE request |
| `http.header(name, value)` | header name + value | Set persistent header for subsequent calls |

Headers and cookies persist across calls within a single scenario/invariant iteration.

## Assertions

Assertions reference `output.*` in `then` blocks and invariants:

| Property | Type | Description |
|----------|------|-------------|
| `output.status` | `int` | HTTP status code |
| `output.body` | `any` | Full response body (parsed JSON) |
| `output.header.<name>` | `string` | Response header value |
| `output.<field.path>` | `any` | Dot-path into JSON response body |
| `output.items.0.name` | `any` | Array index access in dot-path (zero-based) |

The `then` block assertions apply to what the contract's `action` block returned.

## Single-Request Pattern

The contract's `action` block calls the endpoint and returns the response:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

model Account {
  id: string
  balance: int
}

model TransferResult {
  from: Account
  to: Account
  error: string?
}

scope transfer {
  contract Transfer(
    from: Account,
    to: Account,
    amount: int { 0 < amount <= from.balance },
  ) -> TransferResult {
    action {
      return http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
    }

    # Money is conserved on successful transfers.
    invariant conservation {
      output.error == null implies
        output.from.balance + output.to.balance == from.balance + to.balance
    }

    # Balances never go negative.
    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        output.from.balance == from.balance - amount
        output.to.balance == to.balance + amount
        output.error == null
      }
    }

    scenario overdraft {
      when { amount > from.balance }
      then { output.error == "insufficient_funds" }
    }
  }
}
```

## Multi-Step Pattern

Use `let` bindings in the action block for multi-step workflows. `then` assertions apply to the value returned by the action:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

model Widget {
  id: int
  name: string
}

scope create_and_verify {
  contract CreateWidget(name: string) -> Widget {
    action {
      let created = http.post("/api/widgets", { name: name })
      let item = http.get("/api/widgets/" + created.body.id)
      return item.body
    }

    scenario create_then_get {
      given { name: "widget" }
      then {
        output.id != null
        output.name == "widget"
      }
    }
  }
}
```

## Authentication Pattern

Use a top-level `action` for shared auth setup and call it from `before` blocks:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
}

scope protected_resource {
  before {
    login("admin", "test")
  }

  contract GetResource(id: string) -> ResourceResult {
    action {
      return http.get("/api/resources/" + id)
    }

    scenario exists {
      given { id: "r1" }
      then { output.status == 200 }
    }
  }
}
```

## Array Index Access

Dot-paths support numeric segments for array indexing:

```
then {
  output.items.0.name == "first"
  output.items.1.name == "second"
}
```

Out-of-range indices produce an assertion failure.
