package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type Account struct {
	ID      string `json:"id"`
	Balance int    `json:"balance"`
}

type TransferRequest struct {
	From   Account `json:"from"`
	To     Account `json:"to"`
	Amount int     `json:"amount"`
}

type TransferResponse struct {
	Error *string `json:"error"`
	From  Account `json:"from"`
	To    Account `json:"to"`
}

var mu sync.Mutex

func main() {
	http.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	http.HandleFunc("POST /api/v1/accounts/transfer", func(w http.ResponseWriter, r *http.Request) {
		var req TransferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		resp := TransferResponse{From: req.From, To: req.To}

		switch {
		case req.Amount <= 0:
			e := "invalid_amount"
			resp.Error = &e
		case req.Amount > req.From.Balance:
			e := "insufficient_funds"
			resp.Error = &e
		default:
			resp.From.Balance -= req.Amount
			resp.To.Balance += req.Amount
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Println("listening on :8080")
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
