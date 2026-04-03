spec EnumTest {
  description: "Fixture exercising enum type for generator and parser testing"

  http {
    base_url: "http://localhost:8080"
  }

  scope enum_inputs {
    action run(adapter_name: string, subcommand: string, opt_role: string) {
      let result = http.post("/test", { adapter_name: adapter_name, subcommand: subcommand, opt_role: opt_role })
      return result
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
      action: run
    }

    scenario smoke {
      given {
        adapter_name: "http"
        subcommand: "parse"
        opt_role: "admin"
      }
      then {
        ok == true
      }
    }
  }
}
