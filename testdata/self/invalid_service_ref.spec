# Error case: service() references a nonexistent service name.
spec BadRef {
  target {
    base_url: service(nonexistent)
  }

  scope test {
    use http
    config {
      path: "/"
      method: "GET"
    }
    contract {
      input {}
      output { status: any }
    }
    scenario fails {
      given {}
      then { status: 200 }
    }
  }
}
