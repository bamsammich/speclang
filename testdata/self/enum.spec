spec EnumTest {
  description: "Fixture exercising enum type for generator and parser testing"

  target {
    base_url: "http://localhost:8080"
  }

  scope enum_inputs {
    use http
    config {
      path: "/test"
      method: "POST"
    }

    contract {
      input {
        adapter_name: enum("http", "process", "playwright")
        subcommand: enum("parse", "generate", "verify", "install")
        opt_role: enum("admin", "user")?
      }
      output {
        ok: bool
      }
    }

    scenario smoke {
      given {
        adapter_name: "http"
        subcommand: "parse"
        opt_role: "admin"
      }
      then {
        ok: true
      }
    }
  }
}
