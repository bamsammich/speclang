spec LoginUI {
  description: "UI login flow"

  target {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
  }

  locators {
    email_input: [data-testid="email"]
    password_input: [data-testid="password"]
    submit_btn: [data-testid="submit"]
    welcome_msg: [data-testid="welcome"]
    error_msg: [data-testid="error"]
  }

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
        playwright.goto("/login")
        playwright.fill(email_input, email)
        playwright.fill(password_input, password)
        playwright.click(submit_btn)
      }
      then {
        welcome_msg@playwright.visible: true
        welcome_msg@playwright.text: "Welcome, alice"
        error_msg@playwright.visible: false
      }
    }
  }
}
