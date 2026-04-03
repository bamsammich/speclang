spec GroupAPI {
  description: "Group management API"

  http {
    base_url: "http://localhost:8080"
  }

  scope leave_group {
    action leave_group(group_id: string) {
      let result = http.post("/api/v1/groups/" + group_id + "/leave", { group_id: group_id })
      return result
    }

    before {
      let r0 = http.post("/api/v1/auth/login", { provider: "google", id_token: "test-token" })
      http.header("Authorization", "Bearer " + r0.access_token)
    }

    contract {
      input {
        group_id: string
      }
      output {
        success: bool?
        error: string?
      }
      action: leave_group
    }

    scenario last_manager_cannot_leave {
      given {
        let r0 = http.post("/api/v1/groups", { name: "Solo Manager Group" })
        group_id: r0.group.id
      }
      then {
        error == "last_manager_cannot_leave"
      }
    }
  }
}
