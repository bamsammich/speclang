spec LoginUI {
    description: "Login page UI verification"

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

    scope login {
        action run(user: string, pass: string) {
            playwright.fill(username, user)
            playwright.fill(password, pass)
            playwright.click(submit)
            let result = playwright.snapshot()
            return result
        }

        contract {
            input {
                user: string
                pass: string
            }
            output {
                ok: bool
            }
            action: run
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
