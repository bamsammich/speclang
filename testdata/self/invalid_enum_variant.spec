spec InvalidEnumVariant {
  scope test {
    contract {
      input {
        status: enum("active", "inactive")
      }
      output {
        ok: bool
      }
    }
    scenario smoke {
      given {
        status: "deleted"
      }
      then {
        ok == true
      }
    }
  }
}
