spec LoginSmoke {
    description: "Smoke test: playwright adapter verifying a login page"

    target {
        base_url: env(APP_URL, "http://localhost:3000")
    }

    locators {
        username: [data-testid=username]
        password: [data-testid=password]
        submit:   [data-testid=submit]
        welcome:  [data-testid=welcome]
        error:    [data-testid=error]
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
                playwright.goto("/login")
                playwright.fill(username, "alice")
                playwright.fill(password, "secret")
                playwright.click(submit)
                user: "alice"
                pass: "secret"
            }
            then {
                welcome@playwright.visible: true
                welcome@playwright.text: "Welcome, alice"
            }
        }
    }
}
