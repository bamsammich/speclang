spec InvalidEnumEmpty {
  scope test {
    use http
    config {
      path: "/test"
      method: "POST"
    }
    contract {
      input {
        status: enum()
      }
      output {
        ok: bool
      }
    }
  }
}
