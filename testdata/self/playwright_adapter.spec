# Playwright adapter integration test fixture.
# Exercises: goto, fill, click actions; visible, text, value, disabled, count assertions.
# Requires Playwright browsers installed (specrun install playwright).
spec PlaywrightAdapterTest {
  description: "Verifies Playwright adapter actions and assertion properties"

  target {
    base_url: env(PLAYWRIGHT_TEST_URL, "file:///dev/null")
  }

  locators {
    heading: [h1[data-testid="heading"]]
    description: [p[data-testid="description"]]
    name_input: [input[data-testid="name-input"]]
    email_input: [input[data-testid="email-input"]]
    submit_btn: [button[data-testid="submit-btn"]]
    disabled_btn: [button[data-testid="disabled-btn"]]
    result: [div[data-testid="result"]]
    items: [ul[data-testid="item-list"] li.item]
  }

  # Basic navigation and text assertions
  scope pw_text {
    use playwright

    contract {
      input {}
      output {}
    }

    scenario heading_text {
      given {
        playwright.goto("/")
      }
      then {
        heading@playwright.visible: true
        heading@playwright.text: "Test Page"
        description@playwright.text: "This page is used for Playwright adapter integration tests."
      }
    }
  }

  # Fill and value assertions
  scope pw_fill {
    use playwright

    contract {
      input {}
      output {}
    }

    scenario fill_input {
      given {
        playwright.goto("/")
        playwright.fill(name_input, "Test User")
      }
      then {
        name_input@playwright.value: "Test User"
      }
    }
  }

  # Click and result visibility
  scope pw_click {
    use playwright

    contract {
      input {}
      output {}
    }

    scenario click_submit {
      given {
        playwright.goto("/")
        playwright.fill(name_input, "World")
        playwright.click(submit_btn)
      }
      then {
        result@playwright.visible: true
        result@playwright.text: "Hello, World!"
      }
    }
  }

  # Disabled button assertion
  scope pw_disabled {
    use playwright

    contract {
      input {}
      output {}
    }

    scenario disabled_button {
      given {
        playwright.goto("/")
      }
      then {
        disabled_btn@playwright.disabled: true
        submit_btn@playwright.disabled: false
      }
    }
  }

  # Count assertion
  scope pw_count {
    use playwright

    contract {
      input {}
      output {}
    }

    scenario item_count {
      given {
        playwright.goto("/")
      }
      then {
        items@playwright.count: 3
      }
    }
  }
}
