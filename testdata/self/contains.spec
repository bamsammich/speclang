spec ContainsTest {
  http {
    base_url: "http://localhost:8080"
  }

  scope test {
    action run(msg: string, items: []int) {
      let result = http.post("/api/test", { msg: msg, items: items })
      return result
    }

    contract {
      input {
        msg: string
        items: []int
      }
      output {
        ok: bool
      }
      action: run
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
