spec WithLifecycle {
  http {
    base_url: "http://localhost:8080"
  }

  scope authenticated {
    before {
      let token = http.post("/login", { user: "admin", pass: "secret" })
      http.header("Authorization", "Bearer " + token.body.access_token)
    }

    after {
      http.post("/logout", {})
    }

    action do_thing(name: string) {
      let result = http.post("/thing", { name: name })
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        id: string
        status: string
      }
      action: do_thing
    }

    scenario create {
      given {
        name: "widget"
      }
      then {
        status == "created"
      }
    }
  }
}
