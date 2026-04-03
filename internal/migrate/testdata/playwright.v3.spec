spec LoginUI {
  description: "UI login flow"

  playwright {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
  }

  scope ui_login {
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
        playwright.goto("/login")
        playwright.fill('[data-testid=email]', email)
        playwright.fill('[data-testid=password]', password)
        playwright.click('[data-testid=submit]')
      }
      then {
        playwright.visible('[data-testid=welcome]') == true
        playwright.text('[data-testid=welcome]') == "Welcome, alice"
        playwright.visible('[data-testid=error]') == false
      }
    }
  }
}
