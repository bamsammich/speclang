# HTTP adapter integration test fixture.
# Exercises: get, post, put, delete actions; status, body, header.*, dot-path assertions.
# Requires the test HTTP server running on HTTP_TEST_URL (default http://localhost:8082).
spec HTTPAdapterTest {
  description: "Verifies all HTTP adapter actions and assertion properties"

  target {
    base_url: env(HTTP_TEST_URL, "http://localhost:8082")
  }

  # GET — list items, check status, body dot-path, and response header
  scope http_get {
    use http
    config {
      path: "/api/items"
      method: "GET"
    }

    contract {
      input {}
      output {
        status: any
        header: any
        count: int
        items: any
      }
    }

    scenario list_items {
      given {}
      then {
        status: 200
        count: 2
        header.Requestid: "test-123"
      }
    }
  }

  # GET — single item with dot-path assertions
  scope http_get_item {
    use http
    config {
      path: "/api/items/1"
      method: "GET"
    }

    contract {
      input {}
      output {
        status: any
        id: int
        name: string
        tags: any
      }
    }

    scenario get_single_item {
      given {}
      then {
        status: 200
        id: 1
        name: "alpha"
      }
    }
  }

  # POST — create item, check 201 status and echoed body
  scope http_post {
    use http
    config {
      path: "/api/items"
      method: "POST"
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
    }

    scenario create_item {
      given {
        name: "gamma"
      }
      then {
        status: 201
        id: 42
        name: "gamma"
      }
    }
  }

  # PUT — update item, check echoed body
  scope http_put {
    use http
    config {
      path: "/api/items/1"
      method: "PUT"
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
    }

    scenario update_item {
      given {
        name: "alpha-updated"
      }
      then {
        status: 200
        id: 1
        name: "alpha-updated"
      }
    }
  }

  # DELETE — delete item
  scope http_delete {
    use http
    config {
      path: "/api/items/1"
      method: "DELETE"
    }

    contract {
      input {}
      output {
        status: any
        deleted: any
      }
    }

    scenario delete_item {
      given {}
      then {
        status: 200
        deleted: true
      }
    }
  }

  # Header action — set persistent headers then make a request
  scope http_header {
    use http
    config {
      path: "/api/headers"
      method: "GET"
    }

    contract {
      input {}
      output {
        status: any
        auth: string
        custom: string
      }
    }

    scenario custom_headers {
      given {
        http.header("Authorization", "Bearer test-token")
        http.header("X-Custom", "custom-value")
        http.get("/api/headers")
      }
      then {
        status: 200
        auth: "Bearer test-token"
        custom: "custom-value"
      }
    }
  }
}
