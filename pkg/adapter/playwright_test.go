package adapter_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/bamsammich/speclang/pkg/adapter"
	pw "github.com/playwright-community/playwright-go"
)

const testLoginPage = `<!DOCTYPE html>
<html>
<body>
  <h1>Login</h1>
  <form id="login-form">
    <input data-testid="username" type="text" placeholder="Username" />
    <input data-testid="password" type="password" placeholder="Password" />
    <button data-testid="submit" type="submit">Log In</button>
  </form>
  <div data-testid="welcome" style="display:none"></div>
  <div data-testid="error" style="display:none"></div>
  <script>
    document.getElementById('login-form').addEventListener('submit', function(e) {
      e.preventDefault();
      var user = document.querySelector('[data-testid=username]').value;
      var pass = document.querySelector('[data-testid=password]').value;
      var welcome = document.querySelector('[data-testid=welcome]');
      var error = document.querySelector('[data-testid=error]');
      welcome.style.display = 'none';
      error.style.display = 'none';
      if (user && pass === 'secret') {
        welcome.textContent = 'Welcome, ' + user;
        welcome.style.display = 'block';
      } else {
        error.textContent = 'Invalid credentials';
        error.style.display = 'block';
      }
    });
  </script>
</body>
</html>`

// skipIfNoBrowsers attempts to start playwright; skips the test if browsers
// aren't installed rather than failing.
func skipIfNoBrowsers(t *testing.T) {
	t.Helper()
	instance, err := pw.Run()
	if err != nil {
		t.Skipf("playwright not available (run 'specrun install playwright'): %v", err)
	}
	browser, err := instance.Chromium.Launch(pw.BrowserTypeLaunchOptions{
		Headless: pw.Bool(true),
	})
	if err != nil {
		instance.Stop() //nolint:errcheck
		t.Skipf("chromium not installed (run 'specrun install playwright'): %v", err)
	}
	browser.Close() //nolint:errcheck
	instance.Stop() //nolint:errcheck
}

func startTestServer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, testLoginPage)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting test server: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	return fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
}

func TestPlaywrightAdapter_Integration(t *testing.T) {
	skipIfNoBrowsers(t)

	baseURL := startTestServer(t)

	adp := adapter.NewPlaywrightAdapter()
	if err := adp.Init(map[string]string{
		"base_url": baseURL,
		"headless": "true",
		"timeout":  "5000",
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer adp.Close()

	t.Run("goto and fill and click", func(t *testing.T) {
		// Navigate to login page.
		gotoArgs, _ := json.Marshal([]string{"/login"})
		resp, err := adp.Action("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto failed: %s", resp.Error)
		}

		// Fill username.
		fillUser, _ := json.Marshal([]string{"[data-testid=username]", "alice"})
		resp, err = adp.Action("fill", fillUser)
		if err != nil {
			t.Fatalf("fill username: %v", err)
		}
		if !resp.OK {
			t.Fatalf("fill username failed: %s", resp.Error)
		}

		// Fill password.
		fillPass, _ := json.Marshal([]string{"[data-testid=password]", "secret"})
		resp, err = adp.Action("fill", fillPass)
		if err != nil {
			t.Fatalf("fill password: %v", err)
		}
		if !resp.OK {
			t.Fatalf("fill password failed: %s", resp.Error)
		}

		// Click submit.
		clickArgs, _ := json.Marshal([]string{"[data-testid=submit]"})
		resp, err = adp.Action("click", clickArgs)
		if err != nil {
			t.Fatalf("click: %v", err)
		}
		if !resp.OK {
			t.Fatalf("click failed: %s", resp.Error)
		}
	})

	t.Run("assert visible", func(t *testing.T) {
		expected, _ := json.Marshal(true)
		resp, err := adp.Assert("visible", "[data-testid=welcome]", expected)
		if err != nil {
			t.Fatalf("assert visible: %v", err)
		}
		if !resp.OK {
			t.Errorf("welcome should be visible: %s", resp.Error)
		}
	})

	t.Run("assert text", func(t *testing.T) {
		expected, _ := json.Marshal("Welcome, alice")
		resp, err := adp.Assert("text", "[data-testid=welcome]", expected)
		if err != nil {
			t.Fatalf("assert text: %v", err)
		}
		if !resp.OK {
			t.Errorf("welcome text mismatch: expected 'Welcome, alice', got %s", string(resp.Actual))
		}
	})

	t.Run("assert value", func(t *testing.T) {
		expected, _ := json.Marshal("alice")
		resp, err := adp.Assert("value", "[data-testid=username]", expected)
		if err != nil {
			t.Fatalf("assert value: %v", err)
		}
		if !resp.OK {
			t.Errorf("username value mismatch: expected 'alice', got %s", string(resp.Actual))
		}
	})

	t.Run("assert not visible", func(t *testing.T) {
		expected, _ := json.Marshal(false)
		resp, err := adp.Assert("visible", "[data-testid=error]", expected)
		if err != nil {
			t.Fatalf("assert not visible: %v", err)
		}
		if !resp.OK {
			t.Errorf("error should not be visible: %s", resp.Error)
		}
	})

	t.Run("new_page and close_page", func(t *testing.T) {
		resp, err := adp.Action("new_page", nil)
		if err != nil {
			t.Fatalf("new_page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("new_page failed: %s", resp.Error)
		}

		gotoArgs, _ := json.Marshal([]string{"/login"})
		resp, err = adp.Action("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto on new page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto on new page failed: %s", resp.Error)
		}

		resp, err = adp.Action("close_page", nil)
		if err != nil {
			t.Fatalf("close_page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("close_page failed: %s", resp.Error)
		}

		// After closing the new page, the original page should still work.
		expected, _ := json.Marshal(true)
		resp, err = adp.Assert("visible", "[data-testid=welcome]", expected)
		if err != nil {
			t.Fatalf("assert after close_page: %v", err)
		}
		if !resp.OK {
			t.Errorf("welcome should still be visible on original page: %s", resp.Error)
		}
	})

	t.Run("resize viewport", func(t *testing.T) {
		// Resize to mobile.
		resizeArgs, _ := json.Marshal([]int{375, 812})
		resp, err := adp.Action("resize", resizeArgs)
		if err != nil {
			t.Fatalf("resize: %v", err)
		}
		if !resp.OK {
			t.Fatalf("resize failed: %s", resp.Error)
		}

		// Page should still work at mobile size.
		gotoArgs, _ := json.Marshal([]string{"/login"})
		resp, err = adp.Action("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto after resize: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto after resize failed: %s", resp.Error)
		}

		// Resize back to desktop.
		resizeArgs, _ = json.Marshal([]int{1920, 1080})
		resp, err = adp.Action("resize", resizeArgs)
		if err != nil {
			t.Fatalf("resize back: %v", err)
		}
		if !resp.OK {
			t.Fatalf("resize back failed: %s", resp.Error)
		}
	})

	t.Run("failed login shows error", func(t *testing.T) {
		// Navigate fresh.
		gotoArgs, _ := json.Marshal([]string{"/login"})
		adp.Action("goto", gotoArgs) //nolint:errcheck

		fillUser, _ := json.Marshal([]string{"[data-testid=username]", "bob"})
		adp.Action("fill", fillUser) //nolint:errcheck

		fillPass, _ := json.Marshal([]string{"[data-testid=password]", "wrong"})
		adp.Action("fill", fillPass) //nolint:errcheck

		clickArgs, _ := json.Marshal([]string{"[data-testid=submit]"})
		adp.Action("click", clickArgs) //nolint:errcheck

		// Error should be visible.
		expected, _ := json.Marshal(true)
		resp, err := adp.Assert("visible", "[data-testid=error]", expected)
		if err != nil {
			t.Fatalf("assert error visible: %v", err)
		}
		if !resp.OK {
			t.Errorf("error should be visible after bad login: %s", resp.Error)
		}

		// Error text.
		expectedText, _ := json.Marshal("Invalid credentials")
		resp, err = adp.Assert("text", "[data-testid=error]", expectedText)
		if err != nil {
			t.Fatalf("assert error text: %v", err)
		}
		if !resp.OK {
			t.Errorf("error text mismatch: got %s", string(resp.Actual))
		}
	})
}
