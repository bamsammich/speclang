http {
  base_url: "http://localhost:8080"
}

enum Role { admin, user, viewer }

model User {
  role: Role
}

model NamedEnumResult {
  ok: bool
}

scope named_enum_inputs {
  contract NamedEnumContract(role: Role) -> NamedEnumResult {
    action {
      let result = http.post("/test", { role: role })
      return result
    }

    scenario admin_role {
      given {
        role: Role.admin
      }
      then {
        output.ok == true
      }
    }
  }
}
