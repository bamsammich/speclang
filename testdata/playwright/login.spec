playwright {
    base_url: env(APP_URL, "http://localhost:3000")
}

locators {
    username: [data-testid=username]
    password: [data-testid=password]
    submit:   [data-testid=submit]
    welcome:  [data-testid=welcome]
    error:    [data-testid=error]
}

model LoginResult {
  ok: bool
}

scope login {
    contract LoginContract -> LoginResult {
        user: string
        pass: string

        action {
            playwright.fill(username, user)
            playwright.fill(password, pass)
            playwright.click(submit)
            let result = playwright.snapshot()
            return result
        }

        scenario successful_login {
            given {
                user: "alice"
                pass: "secret"
            }
            then {
                playwright.visible(welcome) == true
                playwright.text(welcome) == "Welcome, alice"
            }
        }
    }
}
