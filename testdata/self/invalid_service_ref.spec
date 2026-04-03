# Error case: service() references a nonexistent service name.
spec BadRef {
  http {
    base_url: service(nonexistent)
  }

  scope test {
    action run() {
      let result = http.get("/")
      return result
    }

    contract {
      input {}
      output { status: any }
      action: run
    }

    scenario fails {
      given {}
      then { status == 200 }
    }
  }
}
