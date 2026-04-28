# HTTP adapter integration test fixture.
# Exercises: get, post, put, delete actions; status, body, header.*, dot-path assertions.
# Requires the test HTTP server running on HTTP_TEST_URL (default http://localhost:8082).

http {
  base_url: env(HTTP_TEST_URL, "http://localhost:8082")
}

model HTTPGetResult {
  status: any
  header: any
  count: int
  items: any
}

model HTTPGetItemResult {
  status: any
  id: int
  name: string
  tags: any
}

model HTTPPostResult {
  status: any
  id: int
  name: string
}

model HTTPDeleteResult {
  status: any
  deleted: any
}

model HTTPHeaderResult {
  status: any
  auth: string
  custom: string
}

# GET — list items, check status, body dot-path, and response header
scope http_get {
  contract HTTPGetContract -> HTTPGetResult {
    action {
      let result = http.get("/api/items")
      return result
    }

    scenario list_items {
      given {}
      then {
        output.status == 200
        output.count == 2
        output.header.Requestid == "test-123"
      }
    }
  }
}

# GET — single item with dot-path assertions
scope http_get_item {
  contract HTTPGetItemContract -> HTTPGetItemResult {
    action {
      let result = http.get("/api/items/1")
      return result
    }

    scenario get_single_item {
      given {}
      then {
        output.status == 200
        output.id == 1
        output.name == "alpha"
      }
    }
  }
}

# POST — create item, check 201 status and echoed body
scope http_post {
  contract HTTPPostContract -> HTTPPostResult {
    name: string

    action {
      let result = http.post("/api/items", { name: name })
      return result
    }

    scenario create_item {
      given {
        name: "gamma"
      }
      then {
        output.status == 201
        output.id == 42
        output.name == "gamma"
      }
    }
  }
}

# PUT — update item, check echoed body
scope http_put {
  contract HTTPPutContract -> HTTPPostResult {
    name: string

    action {
      let result = http.put("/api/items/1", { name: name })
      return result
    }

    scenario update_item {
      given {
        name: "alpha-updated"
      }
      then {
        output.status == 200
        output.id == 1
        output.name == "alpha-updated"
      }
    }
  }
}

# DELETE — delete item
scope http_delete {
  contract HTTPDeleteContract -> HTTPDeleteResult {
    action {
      let result = http.delete("/api/items/1")
      return result
    }

    scenario delete_item {
      given {}
      then {
        output.status == 200
        output.deleted == true
      }
    }
  }
}

# Multi-step workflow — POST to create, then GET to verify
scope http_multi_step {
  contract HTTPMultiStepContract -> HTTPPostResult {
    name: string

    action {
      http.post("/api/resources", { name: name })
      let result = http.get("/api/resources/1")
      return result
    }

    scenario create_then_verify {
      given {
        name: "widget"
      }
      then {
        output.status == 200
        output.id == 1
        output.name == "widget"
      }
    }
  }
}

# Multi-step with header persistence — set header, then make two requests
scope http_multi_step_headers {
  contract HTTPMultiStepHeadersContract -> HTTPHeaderResult {
    action {
      http.header("Authorization", "Bearer multi-token")
      http.header("X-Custom", "persistent-value")
      let result = http.get("/api/headers")
      return result
    }

    scenario headers_persist_across_calls {
      given {}
      then {
        output.status == 200
        output.auth == "Bearer multi-token"
        output.custom == "persistent-value"
      }
    }
  }
}

# Header action — set persistent headers then make a request
scope http_header {
  contract HTTPHeaderContract -> HTTPHeaderResult {
    action {
      http.header("Authorization", "Bearer test-token")
      http.header("X-Custom", "custom-value")
      let result = http.get("/api/headers")
      return result
    }

    scenario custom_headers {
      given {}
      then {
        output.status == 200
        output.auth == "Bearer test-token"
        output.custom == "custom-value"
      }
    }
  }
}
