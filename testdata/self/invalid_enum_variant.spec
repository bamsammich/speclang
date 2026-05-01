model EnumVariantResult {
  ok: bool
}

scope test {
  contract EnumVariantContract(status: enum("active", "inactive")) -> EnumVariantResult {
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
