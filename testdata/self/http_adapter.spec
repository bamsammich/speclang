# HTTP adapter integration test fixture.
# Exercises: get, post, put, delete actions; status, body, header.*, dot-path assertions.
# Requires the test HTTP server running on HTTP_TEST_URL (default http://localhost:8082).
spec HTTPAdapterTest {
  description: "Verifies all HTTP adapter actions and assertion properties"

  http {
    base_url: env(HTTP_TEST_URL, "http://localhost:8082")
  }

  # GET — list items, check status, body dot-path, and response header
  scope http_get {
    action run() {
      let result = http.get("/api/items")
      return result
    }

    contract {
      input {}
      output {
        status: any
        header: any
        count: int
        items: any
      }
      action: run
    }

    scenario list_items {
      given {}
      then {
        status == 200
        count == 2
        header.Requestid == "test-123"
      }
    }
  }

  # GET — single item with dot-path assertions
  scope http_get_item {
    action run() {
      let result = http.get("/api/items/1")
      return result
    }

    contract {
      input {}
      output {
        status: any
        id: int
        name: string
        tags: any
      }
      action: run
    }

    scenario get_single_item {
      given {}
      then {
        status == 200
        id == 1
        name == "alpha"
      }
    }
  }

  # POST — create item, check 201 status and echoed body
  scope http_post {
    action run(name: string) {
      let result = http.post("/api/items", { name: name })
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        status: any
        id: int
        name: string
      }
      action: run
    }

    scenario create_item {
      given {
        name: "gamma"
      }
      then {
        status == 201
        id == 42
        name == "gamma"
      }
    }
  }

  # PUT — update item, check echoed body
  scope http_put {
    action run(name: string) {
      let result = http.put("/api/items/1", { name: name })
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        status: any
        id: int
        name: string
      }
      action: run
    }

    scenario update_item {
      given {
        name: "alpha-updated"
      }
      then {
        status == 200
        id == 1
        name == "alpha-updated"
      }
    }
  }

  # DELETE — delete item
  scope http_delete {
    action run() {
      let result = http.delete("/api/items/1")
      return result
    }

    contract {
      input {}
      output {
        status: any
        deleted: any
      }
      action: run
    }

    scenario delete_item {
      given {}
      then {
        status == 200
        deleted == true
      }
    }
  }

  # Multi-step workflow — POST to create, then GET to verify
  scope http_multi_step {
    action run(name: string) {
      http.post("/api/resources", { name: name })
      let result = http.get("/api/resources/1")
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        status: any
        id: int
        name: string
      }
      action: run
    }

    scenario create_then_verify {
      given {
        name: "widget"
      }
      then {
        status == 200
        id == 1
        name == "widget"
      }
    }
  }

  # Multi-step with header persistence — set header, then make two requests
  scope http_multi_step_headers {
    action run() {
      http.header("Authorization", "Bearer multi-token")
      http.header("X-Custom", "persistent-value")
      let result = http.get("/api/headers")
      return result
    }

    contract {
      input {}
      output {
        status: any
        auth: string
        custom: string
      }
      action: run
    }

    scenario headers_persist_across_calls {
      given {}
      then {
        status == 200
        auth == "Bearer multi-token"
        custom == "persistent-value"
      }
    }
  }

  # Header action — set persistent headers then make a request
  scope http_header {
    action run() {
      http.header("Authorization", "Bearer test-token")
      http.header("X-Custom", "custom-value")
      let result = http.get("/api/headers")
      return result
    }

    contract {
      input {}
      output {
        status: any
        auth: string
        custom: string
      }
      action: run
    }

    scenario custom_headers {
      given {}
      then {
        status == 200
        auth == "Bearer test-token"
        custom == "custom-value"
      }
    }
  }
}
