// Minimal login page server for playwright adapter integration testing.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const loginPage = `<!DOCTYPE html>
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
      const user = document.querySelector('[data-testid=username]').value;
      const pass = document.querySelector('[data-testid=password]').value;
      const welcome = document.querySelector('[data-testid=welcome]');
      const error = document.querySelector('[data-testid=error]');
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

func main() {
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, loginPage)
	})

	// Health check for test readiness.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	fmt.Printf("listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
