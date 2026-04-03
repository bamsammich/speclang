spec InvalidEnumEmpty {
  scope test {
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
