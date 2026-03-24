# HTTP Adapter

The HTTP adapter (`use http`) tests REST APIs. It supports single-request scopes and multi-step workflows.

## Configuration

### Target Block

| Key | Required | Description |
|-----|----------|-------------|
| `base_url` | Yes | API base URL. Supports `env()` expressions. |

```
target {
  base_url: env(APP_URL, "http://localhost:8080")
}
```

### Scope Config

| Key | Description |
|-----|-------------|
| `path` | Request path (for single-request scopes) |
| `method` | HTTP method: `"GET"`, `"POST"`, `"PUT"`, `"DELETE"` |

```
config {
  path: "/api/v1/accounts/transfer"
  method: "POST"
}
```

When `given` contains action calls, `config` is not used for request dispatch -- each action call specifies its own path.

## Actions

| Action | Args | Description |
|--------|------|-------------|
| `http.get(path)` | URL path | GET request |
| `http.post(path, body)` | URL path + JSON body | POST request |
| `http.put(path, body)` | URL path + JSON body | PUT request |
| `http.delete(path)` | URL path | DELETE request |
| `http.header(name, value)` | header name + value | Set persistent header |

## Assertions

| Property | Type | Description |
|----------|------|-------------|
| `status` | `int` | HTTP status code |
| `body` | `any` | Full response body (parsed JSON) |
| `header.<name>` | `string` | Response header value |
| `<field.path>` | `any` | Dot-path into JSON response body |
| `<field.path>.0.<field>` | `any` | Array index access in dot-path |

## Single-Request Pattern

Use scope `config` to define the endpoint. All `given` assignments become the request body.

```
spec AccountAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  model Account {
    id: string
    balance: int
  }

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

    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance: from.balance - amount
        to.balance: to.balance + amount
        error: null
      }
    }

    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        error: "insufficient_funds"
      }
    }
  }
}
```

## Multi-Step Pattern

Use action calls in `given` blocks for multi-step workflows. Headers and cookies persist across calls within a scenario. `then` assertions apply to the last response.

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

  scenario with_auth {
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

## Array Index Access

Dot-paths support numeric segments for array indexing:

```
then {
  items.0.name: "first"
  items.1.name: "second"
}
```

Out-of-range indices produce an assertion failure.
