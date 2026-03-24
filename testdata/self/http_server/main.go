// HTTP test server with predictable responses for adapter integration tests.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	port := "8082"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// GET /api/items — returns a known JSON array
	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "test-123")
		w.Header().Set("Requestid", "test-123")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "name": "alpha"},
				{"id": 2, "name": "beta"},
			},
			"count": 2,
		})
	})

	// GET /api/items/1 — returns a single item
	mux.HandleFunc("GET /api/items/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   1,
			"name": "alpha",
			"tags": []string{"first", "primary"},
		})
	})

	// POST /api/items — echoes the body back with an id
	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body["id"] = 42
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	})

	// PUT /api/items/1 — echoes the body back
	mux.HandleFunc("PUT /api/items/1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body["id"] = 1
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	})

	// DELETE /api/items/1 — returns 204 with empty JSON
	mux.HandleFunc("DELETE /api/items/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})

	// GET /api/headers — echoes back request headers
	mux.HandleFunc("GET /api/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"auth":         r.Header.Get("Authorization"),
			"custom":       r.Header.Get("X-Custom"),
			"content_type": r.Header.Get("Content-Type"),
		})
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "http test server listening on :%s\n", port)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
