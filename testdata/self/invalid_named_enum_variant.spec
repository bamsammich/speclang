enum Role { admin, user, viewer }

model RoleResult {
  ok: bool
}

scope test {
  contract RoleContract(role: Role) -> RoleResult {
    action {
      return http.get("/test")
    }

    invariant valid_role {
      output.role == Role.bogus
    }
  }
}
