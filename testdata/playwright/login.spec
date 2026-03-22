spec LoginUI {
    description: "Login page UI verification"

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
                playwright.fill(username, "alice")
                playwright.fill(password, "secret")
                user: "alice"
                pass: "secret"
                playwright.click(submit)
            }
            then {
                welcome@playwright.visible: true
                welcome@playwright.text: "Welcome, alice"
            }
        }
    }
}
