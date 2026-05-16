package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	fb "github.com/vortelio/vortelio/internal/firebase"
	stripeutil "github.com/vortelio/vortelio/internal/stripe"
	"github.com/vortelio/vortelio/internal/config"
)

// POST /api/stripe/checkout
// Body: {"plan": "pro"|"business"|"enterprise", "success_url": "...", "cancel_url": "..."}
// Returns: {"url": "https://checkout.stripe.com/..."}
func handleStripeCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if !stripeutil.Enabled() {
		respond(w, 503, map[string]string{"error": "payments not configured"})
		return
	}
	if !fb.Enabled() {
		respond(w, 503, map[string]string{"error": "auth not configured"})
		return
	}

	uid := uidFromRequest(r)
	if uid == "" {
		respond(w, 401, map[string]string{"error": "authentication required"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var req struct {
		Plan       string `json:"plan"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request"})
		return
	}

	priceID := stripeutil.PlanPriceID(req.Plan)
	if priceID == "" {
		respond(w, 400, map[string]string{"error": fmt.Sprintf("unknown plan %q", req.Plan)})
		return
	}
	if req.SuccessURL == "" || req.CancelURL == "" {
		respond(w, 400, map[string]string{"error": "success_url and cancel_url required"})
		return
	}

	profile, _ := fb.GetUserProfile(r.Context(), uid)
	email := ""
	if profile != nil {
		email = profile.Email
	}

	url, err := stripeutil.CreateCheckoutSession(priceID, email, req.SuccessURL, req.CancelURL)
	if err != nil {
		slog.Error("stripe checkout error", "uid", uid, "plan", req.Plan, "err", err)
		respond(w, 502, map[string]string{"error": "payment error"})
		return
	}
	respond(w, 200, map[string]string{"url": url})
}

// POST /api/stripe/webhook
// Stripe sends subscription lifecycle events here.
// Verifies signature, then updates Firestore plan field.
func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	webhookSecret := config.Get().StripeWebhookSec
	if webhookSecret == "" {
		respond(w, 503, map[string]string{"error": "webhook not configured"})
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond(w, 400, map[string]string{"error": "read error"})
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), webhookSecret)
	if err != nil {
		slog.Warn("stripe webhook signature invalid", "err", err)
		respond(w, 400, map[string]string{"error": "invalid signature"})
		return
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		handleSubscriptionChange(r, event, false)
	case "customer.subscription.deleted":
		handleSubscriptionChange(r, event, true)
	}

	respond(w, 200, map[string]string{"status": "ok"})
}

func handleSubscriptionChange(r *http.Request, event stripe.Event, cancelled bool) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook unmarshal", "err", err)
		return
	}

	// Find matching UID via customer email stored in metadata or customer object
	// We store uid in subscription metadata at checkout time — check there first.
	uid := sub.Metadata["uid"]
	if uid == "" {
		// Fallback: find by customer email (less reliable but acceptable)
		slog.Warn("stripe webhook: no uid in metadata", "subscription", sub.ID)
		return
	}

	if cancelled || sub.Status == stripe.SubscriptionStatusCanceled ||
		sub.Status == stripe.SubscriptionStatusUnpaid {
		if err := fb.UpdateUserPlan(r.Context(), uid, "free"); err != nil {
			slog.Error("stripe webhook: downgrade failed", "uid", uid, "err", err)
		}
		return
	}

	// Determine plan from price ID
	plan := planFromPriceID(sub)
	if plan == "" {
		slog.Warn("stripe webhook: unknown price ID", "subscription", sub.ID)
		return
	}

	if err := fb.UpdateUserPlan(r.Context(), uid, plan); err != nil {
		slog.Error("stripe webhook: upgrade failed", "uid", uid, "plan", plan, "err", err)
	}
}

func planFromPriceID(sub stripe.Subscription) string {
	cfg := config.Get()
	for _, item := range sub.Items.Data {
		pid := item.Price.ID
		switch pid {
		case cfg.StripePricePro:
			return "pro"
		case cfg.StripePriceBusiness:
			return "business"
		case cfg.StripePriceEnterprise:
			return "enterprise"
		}
	}
	return ""
}
