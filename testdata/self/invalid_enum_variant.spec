spec InvalidEnumVariant {
  scope test {
    use http
    config {
      path: "/test"
      method: "POST"
    }
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
        ok: true
      }
    }
  }
}
