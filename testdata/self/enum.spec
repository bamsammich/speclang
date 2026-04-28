http {
  base_url: "http://localhost:8080"
}

model EnumResult {
  ok: bool
}

scope enum_inputs {
  contract EnumContract -> EnumResult {
    adapter_name: enum("http", "process", "playwright")
    subcommand: enum("parse", "generate", "verify", "install")
    opt_role: enum("admin", "user")?

    action {
      let result = http.post("/test", { adapter_name: adapter_name, subcommand: subcommand, opt_role: opt_role })
      return result
    }

    scenario smoke {
      given {
        adapter_name: "http"
        subcommand: "parse"
        opt_role: "admin"
      }
      then {
        output.ok == true
      }
    }
  }
}
