spec ContainsTest {
  scope test {
    use http
    config {
      path: "/api/test"
      method: "POST"
    }

    contract {
      input {
        msg: string
        items: []int
      }
      output {
        ok: bool
      }
    }

    # Verify contains() works in invariant expressions
    invariant error_has_keyword {
      when ok == false:
        contains(msg, "error")
    }

    # Verify contains() works with array membership
    invariant items_has_element {
      when contains(items, 1):
        ok == true
    }
  }
}
