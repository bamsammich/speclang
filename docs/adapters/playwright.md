# Playwright Adapter

The Playwright adapter (`use playwright`) drives a real browser via [playwright-go](https://github.com/playwright-community/playwright-go). It is compiled into specrun -- no subprocess or external binary required.

## Install

Playwright requires browser binaries (Chromium). Install once:

```bash
specrun install playwright
```

This downloads Chromium (~165 MB). The browser is reused across runs.

## Configuration

### Target Block

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `base_url` | Yes | -- | App URL, prepended to relative `goto` paths |
| `headless` | No | `"true"` | Set `"false"` to see the browser |
| `timeout` | No | `"5000"` | Action/assertion timeout in milliseconds |

```
target {
  base_url: env(APP_URL, "http://localhost:3000")
  headless: "true"
  timeout: "5000"
}
```

### Scope Config

| Key | Description |
|-----|-------------|
| `url` | Page URL for generative scenario isolation |

## Locators

Declare named CSS selectors at spec level. All locators must be pre-declared here -- inline selectors in action calls are not supported.

```
locators {
  username_field: [data-testid=username]
  password_field: [data-testid=password]
  submit_btn:     [data-testid=submit]
  welcome:        [data-testid=welcome]
  error_msg:      [data-testid=error]
}
```

Prefer `data-testid` attributes over CSS classes or IDs -- they're stable across styling changes.

## Actions

| Action | Args | Description |
|--------|------|-------------|
| `playwright.goto(url)` | URL string | Navigate (prepends `base_url` for relative paths) |
| `playwright.click(locator)` | locator name | Click element |
| `playwright.fill(locator, value)` | locator name + text | Clear and type into input |
| `playwright.type(locator, value)` | locator name + text | Append text (no clear) |
| `playwright.select(locator, value)` | locator name + option | Select dropdown option |
| `playwright.check(locator)` | locator name | Check checkbox |
| `playwright.uncheck(locator)` | locator name | Uncheck checkbox |
| `playwright.wait(locator)` | locator name | Wait for element to be visible |
| `playwright.resize(width, height)` | integers | Set viewport size |
| `playwright.new_page()` | -- | Create a fresh browser page |
| `playwright.close_page()` | -- | Close current page |
| `playwright.clear_state()` | -- | Clear cookies and localStorage |

## Assertions

Use `locator@playwright.property: expected` syntax in `then` blocks:

```
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
  error_msg@playwright.visible: false
}
```

| Property | Type | Description |
|----------|------|-------------|
| `visible` | `bool` | Element is visible |
| `text` | `string` | Text content |
| `value` | `string` | Input field value |
| `checked` | `bool` | Checkbox state |
| `disabled` | `bool` | Whether element is disabled |
| `count` | `int` | Number of matching elements |
| `attribute.<name>` | `string` | Named attribute value (e.g., `attribute.href`) |

## Page Lifecycle

For generative scenarios, the adapter reloads the scope's `config.url` before each generated input to ensure test isolation.

Use `new_page()`, `close_page()`, and `clear_state()` for explicit page lifecycle control within multi-step scenarios.

## Mixed `given` Blocks

`given` blocks can interleave data assignments and action calls. Steps execute in order:

```
given {
  playwright.fill(amount_field, "50")
  from_balance: 100
  playwright.click(transfer_btn)
}
```

## Full Example

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

    invariant no_welcome_on_failure {
      when ok == false:
        welcome@playwright.visible: false
    }
  }
}
```

## Running a Playwright Spec

```bash
# 1. Install browsers (one-time)
specrun install playwright

# 2. Start your web app
#    (specrun doesn't manage your app — it connects to a running server)

# 3. Verify
APP_URL=http://localhost:3000 specrun verify login.spec
```
