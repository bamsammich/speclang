model EnumVariantResult {
  ok: bool
}

scope test {
  contract EnumVariantContract -> EnumVariantResult {
    status: enum("active", "inactive")

    action {
      return http.get("/test")
    }

    scenario smoke {
      given {
        status: "deleted"
      }
      then {
        output.ok == true
      }
    }
  }
}
