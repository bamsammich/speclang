spec GroupAPI {
  description: "Group management API"

  target {
    base_url: "http://localhost:8080"
  }

  scope leave_group {
    use http
    config {
      path: "/api/v1/groups/:group_id/leave"
      method: "POST"
    }

    before {
      http.post("/api/v1/auth/login", { provider: "google", id_token: "test-token" })
      http.header("Authorization", "Bearer " + body.access_token)
    }

    contract {
      input {
        group_id: string
      }
      output {
        success: bool?
        error: string?
      }
    }

    scenario last_manager_cannot_leave {
      given {
        http.post("/api/v1/groups", { name: "Solo Manager Group" })
        group_id: body.group.id
      }
      then {
        error: "last_manager_cannot_leave"
      }
    }
  }
}
