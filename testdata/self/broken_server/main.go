package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	port := "8081"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	http.HandleFunc("POST /api/v1/accounts/transfer", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			From struct {
				ID      string `json:"id"`
				Balance int    `json:"balance"`
			} `json:"from"`
			To struct {
				ID      string `json:"id"`
				Balance int    `json:"balance"`
			} `json:"to"`
			Amount int `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := map[string]any{
			"from":  map[string]any{"id": req.From.ID, "balance": req.From.Balance},
			"to":    map[string]any{"id": req.To.ID, "balance": req.To.Balance},
			"error": nil,
		}

		switch {
		case req.Amount <= 0:
			resp["error"] = "invalid_amount"
		case req.Amount > req.From.Balance:
			resp["error"] = "insufficient_funds"
		default:
			// BUG: only credits to-account, does NOT debit from-account
			resp["to"] = map[string]any{"id": req.To.ID, "balance": req.To.Balance + req.Amount}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "broken server listening on :%s\n", port)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
