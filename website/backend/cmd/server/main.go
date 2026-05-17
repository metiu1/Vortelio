package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/vortelio/website/internal/config"
	fb "github.com/vortelio/website/internal/firebase"
	stripeutil "github.com/vortelio/website/internal/stripe"
)

func main() {
	cfgPath := "config.json"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg := config.Load(cfgPath)

	if err := fb.Init(); err != nil {
		slog.Warn("firebase disabled", "err", err)
	}

	mux := http.NewServeMux()

	// Static frontend files
	mux.Handle("/", http.FileServer(http.Dir(cfg.FrontendDir)))

	// API
	mux.HandleFunc("/api/checkout", handleCheckout)
	mux.HandleFunc("/api/stripe/webhook", stripeutil.HandleWebhook)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("vortelio website server", "addr", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// POST /api/checkout
// Body: {"plan":"pro","email":"...","uid":"...","success_url":"...","cancel_url":"..."}
func handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Plan       string `json:"plan"`
		Email      string `json:"email"`
		UID        string `json:"uid"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid request")
		return
	}
	if req.Plan == "" || req.SuccessURL == "" || req.CancelURL == "" {
		jsonErr(w, 400, "plan, success_url and cancel_url required")
		return
	}

	url, err := stripeutil.CreateCheckoutURL(req.Plan, req.Email, req.UID, req.SuccessURL, req.CancelURL)
	if err != nil {
		slog.Error("checkout error", "err", err)
		jsonErr(w, 502, "checkout error")
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (strings.Contains(origin, "vortelio.app") || strings.Contains(origin, "localhost")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		h.ServeHTTP(w, r)
	})
}
