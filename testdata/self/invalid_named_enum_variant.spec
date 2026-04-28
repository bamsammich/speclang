enum Role { admin, user, viewer }

model RoleResult {
  ok: bool
}

scope test {
  contract RoleContract -> RoleResult {
    role: Role

    action {
      return http.get("/test")
    }

    invariant valid_role {
      output.role == Role.bogus
    }
  }
}
