package adapter_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"

	pw "github.com/playwright-community/playwright-go"

	"github.com/bamsammich/speclang/v3/internal/adapter"
)

// mustMarshal is a test helper that marshals v to JSON, failing the test on error.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

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
		instance.Stop() //nolint:errcheck // best-effort cleanup before skip
		t.Skipf("chromium not installed (run 'specrun install playwright'): %v", err)
	}
	browser.Close() //nolint:errcheck // best-effort cleanup in probe
	instance.Stop() //nolint:errcheck // best-effort cleanup in probe
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
	go srv.Serve(listener) //nolint:errcheck // server lifecycle managed by t.Cleanup
	t.Cleanup(func() { srv.Close() })

	return fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
}

// assertPlaywrightQuery calls Call with a query method and selector, then checks the actual value.
func assertPlaywrightQuery(t *testing.T, adp *adapter.PlaywrightAdapter, method, selector string, expected json.RawMessage) {
	t.Helper()
	args := mustMarshal(t, []string{selector})
	resp, err := adp.Call(method, args)
	if err != nil {
		t.Fatalf("query %q on %q: %v", method, selector, err)
	}
	if !resp.OK {
		t.Fatalf("query %q on %q not OK: %s", method, selector, resp.Error)
	}
	if string(resp.Actual) != string(expected) {
		t.Errorf("query %q on %q: expected %s, got %s", method, selector, string(expected), string(resp.Actual))
	}
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
		gotoArgs := mustMarshal(t, []string{"/login"})
		resp, err := adp.Call("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto failed: %s", resp.Error)
		}

		// Fill username.
		fillUser := mustMarshal(t, []string{"[data-testid=username]", "alice"})
		resp, err = adp.Call("fill", fillUser)
		if err != nil {
			t.Fatalf("fill username: %v", err)
		}
		if !resp.OK {
			t.Fatalf("fill username failed: %s", resp.Error)
		}

		// Fill password.
		fillPass := mustMarshal(t, []string{"[data-testid=password]", "secret"})
		resp, err = adp.Call("fill", fillPass)
		if err != nil {
			t.Fatalf("fill password: %v", err)
		}
		if !resp.OK {
			t.Fatalf("fill password failed: %s", resp.Error)
		}

		// Click submit.
		clickArgs := mustMarshal(t, []string{"[data-testid=submit]"})
		resp, err = adp.Call("click", clickArgs)
		if err != nil {
			t.Fatalf("click: %v", err)
		}
		if !resp.OK {
			t.Fatalf("click failed: %s", resp.Error)
		}
	})

	t.Run("query visible", func(t *testing.T) {
		assertPlaywrightQuery(t, adp, "visible", "[data-testid=welcome]", mustMarshal(t, true))
	})

	t.Run("query text", func(t *testing.T) {
		assertPlaywrightQuery(t, adp, "text", "[data-testid=welcome]", mustMarshal(t, "Welcome, alice"))
	})

	t.Run("query value", func(t *testing.T) {
		assertPlaywrightQuery(t, adp, "value", "[data-testid=username]", mustMarshal(t, "alice"))
	})

	t.Run("query not visible", func(t *testing.T) {
		assertPlaywrightQuery(t, adp, "visible", "[data-testid=error]", mustMarshal(t, false))
	})

	t.Run("new_page and close_page", func(t *testing.T) {
		resp, err := adp.Call("new_page", nil)
		if err != nil {
			t.Fatalf("new_page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("new_page failed: %s", resp.Error)
		}

		gotoArgs := mustMarshal(t, []string{"/login"})
		resp, err = adp.Call("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto on new page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto on new page failed: %s", resp.Error)
		}

		resp, err = adp.Call("close_page", nil)
		if err != nil {
			t.Fatalf("close_page: %v", err)
		}
		if !resp.OK {
			t.Fatalf("close_page failed: %s", resp.Error)
		}

		// After closing the new page, the original page should still work.
		assertPlaywrightQuery(t, adp, "visible", "[data-testid=welcome]", mustMarshal(t, true))
	})

	t.Run("resize viewport", func(t *testing.T) {
		// Resize to mobile.
		resizeArgs := mustMarshal(t, []int{375, 812})
		resp, err := adp.Call("resize", resizeArgs)
		if err != nil {
			t.Fatalf("resize: %v", err)
		}
		if !resp.OK {
			t.Fatalf("resize failed: %s", resp.Error)
		}

		// Page should still work at mobile size.
		gotoArgs := mustMarshal(t, []string{"/login"})
		resp, err = adp.Call("goto", gotoArgs)
		if err != nil {
			t.Fatalf("goto after resize: %v", err)
		}
		if !resp.OK {
			t.Fatalf("goto after resize failed: %s", resp.Error)
		}

		// Resize back to desktop.
		resizeArgs = mustMarshal(t, []int{1920, 1080})
		resp, err = adp.Call("resize", resizeArgs)
		if err != nil {
			t.Fatalf("resize back: %v", err)
		}
		if !resp.OK {
			t.Fatalf("resize back failed: %s", resp.Error)
		}
	})

	t.Run("failed login shows error", func(t *testing.T) {
		// Navigate fresh.
		gotoArgs := mustMarshal(t, []string{"/login"})
		resp, err := adp.Call("goto", gotoArgs)
		if err != nil {
			t.Fatalf("setup action failed: %v", err)
		}
		if !resp.OK {
			t.Fatalf("setup action failed: %s", resp.Error)
		}

		fillUser := mustMarshal(t, []string{"[data-testid=username]", "bob"})
		resp, err = adp.Call("fill", fillUser)
		if err != nil {
			t.Fatalf("setup action failed: %v", err)
		}
		if !resp.OK {
			t.Fatalf("setup action failed: %s", resp.Error)
		}

		fillPass := mustMarshal(t, []string{"[data-testid=password]", "wrong"})
		resp, err = adp.Call("fill", fillPass)
		if err != nil {
			t.Fatalf("setup action failed: %v", err)
		}
		if !resp.OK {
			t.Fatalf("setup action failed: %s", resp.Error)
		}

		clickArgs := mustMarshal(t, []string{"[data-testid=submit]"})
		resp, err = adp.Call("click", clickArgs)
		if err != nil {
			t.Fatalf("setup action failed: %v", err)
		}
		if !resp.OK {
			t.Fatalf("setup action failed: %s", resp.Error)
		}

		// Error should be visible.
		assertPlaywrightQuery(t, adp, "visible", "[data-testid=error]", mustMarshal(t, true))

		// Error text.
		assertPlaywrightQuery(t, adp, "text", "[data-testid=error]", mustMarshal(t, "Invalid credentials"))
	})
}
