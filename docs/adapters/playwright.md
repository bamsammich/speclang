# Playwright Adapter

The Playwright adapter drives a real browser via [playwright-go](https://github.com/playwright-community/playwright-go). It is compiled into `specrun` — no subprocess or external binary required.

## Install

Playwright requires browser binaries (Chromium). Install once:

```bash
specrun install playwright
```

This downloads Chromium (~165 MB). The browser is reused across runs.

## Configuration

Configure at the top level of the spec file:

```
playwright {
  base_url: env(APP_URL, "http://localhost:3000")
  headless: "true"
  timeout: "5000"
}
```

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `base_url` | Yes | — | App URL, prepended to relative `goto` paths |
| `headless` | No | `"true"` | Set `"false"` to see the browser during a run |
| `timeout` | No | `"5000"` | Action/assertion timeout in milliseconds |

## Selectors

Pass CSS selectors as inline string arguments to every method call. Prefer `data-testid` attributes — they're stable across styling changes.

```
playwright.fill('[data-testid="username"]', "alice")
playwright.click('[data-testid="submit"]')
```

Single-quoted strings for selectors containing double quotes:

```
playwright.fill('[data-testid="email"]', "alice@example.com")
```

There is no `locators` block in v4 — inline selectors replace the old locator declaration pattern.

## Actions

| Action | Args | Description |
|--------|------|-------------|
| `playwright.goto(url)` | URL string | Navigate (prepends `base_url` for relative paths) |
| `playwright.click(selector)` | CSS selector string | Click element |
| `playwright.fill(selector, value)` | CSS selector + text | Clear and type into input |
| `playwright.type(selector, value)` | CSS selector + text | Append text (no clear) |
| `playwright.select(selector, value)` | CSS selector + option | Select dropdown option |
| `playwright.check(selector)` | CSS selector | Check checkbox |
| `playwright.uncheck(selector)` | CSS selector | Uncheck checkbox |
| `playwright.wait(selector)` | CSS selector | Wait for element to be visible |
| `playwright.new_page()` | — | Create a fresh browser page |
| `playwright.close_page()` | — | Close current page |
| `playwright.clear_state()` | — | Clear cookies and localStorage |

## Assertion Methods

Use `playwright.method(selector) == expected` in `then` and invariant blocks:

```
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Welcome, alice"
  playwright.visible('[data-testid="error"]') == false
}
```

| Method | Return type | Description |
|--------|-------------|-------------|
| `playwright.visible(selector)` | `bool` | Element is visible |
| `playwright.text(selector)` | `string` | Text content |
| `playwright.value(selector)` | `string` | Input field value |
| `playwright.checked(selector)` | `bool` | Checkbox state |
| `playwright.disabled(selector)` | `bool` | Whether element is disabled |
| `playwright.count(selector)` | `int` | Number of matching elements |

## Full Example

```
playwright {
  base_url: env(APP_URL, "http://localhost:3000")
  headless: "true"
  timeout: "5000"
}

action login(user: string, pass: string) {
  playwright.goto("/login")
  playwright.fill('[data-testid="username"]', user)
  playwright.fill('[data-testid="password"]', pass)
  playwright.click('[data-testid="submit"]')
}

model LoginResult {
  ok: bool
}

scope login_flow {
  contract Login(
    user: string,
    pass: string,
  ) -> LoginResult {
    action {
      login(user, pass)
      let ok = playwright.visible('[data-testid="welcome"]')
      return { ok: ok }
    }

    scenario successful_login {
      given {
        user: "alice"
        pass: "secret"
      }
      then {
        playwright.visible('[data-testid="welcome"]') == true
        playwright.text('[data-testid="welcome"]') == "Welcome, alice"
        playwright.visible('[data-testid="error"]') == false
        output.ok == true
      }
    }

    scenario invalid_credentials {
      when { pass != "secret" }
      then {
        playwright.visible('[data-testid="error"]') == true
        output.ok == false
      }
    }

    # The welcome banner must never appear when login fails.
    invariant no_welcome_on_failure {
      output.ok == false implies
        playwright.visible('[data-testid="welcome"]') == false
    }
  }
}
```

## Mixed Adapters

A scope can mix HTTP and Playwright adapters freely. Authenticate via HTTP, then verify the UI:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}
playwright {
  base_url: env(APP_URL, "http://localhost:3000")
}

action authenticate(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  playwright.goto("/dashboard")
}
```

## Page Isolation

For generative scenarios, the adapter resets page state between generated inputs. Use `playwright.goto(url)` at the start of an action to ensure the right page is loaded. Use `playwright.new_page()`, `playwright.close_page()`, and `playwright.clear_state()` for explicit lifecycle control in multi-step scenarios.

## Running a Playwright Spec

```bash
# 1. Install browsers (one-time)
specrun install playwright

# 2. Start your web app (specrun connects to a running server)

# 3. Verify
APP_URL=http://localhost:3000 specrun verify login.spec
```
