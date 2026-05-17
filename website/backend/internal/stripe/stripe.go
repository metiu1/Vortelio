package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
	fb "github.com/vortelio/website/internal/firebase"
	"github.com/vortelio/website/internal/config"
)

func Init() {
	stripe.Key = config.Get().StripeSecretKey
}

var planPrices = map[string]func() string{
	"pro":        func() string { return config.Get().StripePricePro },
	"business":   func() string { return config.Get().StripePriceBusiness },
	"enterprise": func() string { return config.Get().StripePriceEnterprise },
}

// CreateCheckoutURL creates a Stripe Checkout session and returns the URL.
func CreateCheckoutURL(plan, email, uid, successURL, cancelURL string) (string, error) {
	Init()
	priceFn, ok := planPrices[plan]
	if !ok {
		return "", fmt.Errorf("unknown plan: %s", plan)
	}
	priceID := priceFn()
	if priceID == "" {
		return "", fmt.Errorf("price ID not configured for plan %s", plan)
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:    stripe.String(successURL),
		CancelURL:     stripe.String(cancelURL),
		CustomerEmail: stripe.String(email),
	}
	// Store uid in metadata so the webhook can find the user
	params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
		Metadata: map[string]string{"uid": uid},
	}

	s, err := session.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

// HandleWebhook verifies the Stripe signature and processes subscription events.
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", 400)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), config.Get().StripeWebhookSecret)
	if err != nil {
		slog.Warn("stripe webhook sig invalid", "err", err)
		http.Error(w, "invalid signature", 400)
		return
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		processSubscription(r.Context(), event, false)
	case "customer.subscription.deleted":
		processSubscription(r.Context(), event, true)
	}

	w.WriteHeader(200)
}

func processSubscription(ctx context.Context, event stripe.Event, cancelled bool) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook unmarshal", "err", err)
		return
	}

	uid := sub.Metadata["uid"]
	if uid == "" {
		slog.Warn("stripe webhook: no uid in subscription metadata", "sub", sub.ID)
		return
	}

	if cancelled || sub.Status == stripe.SubscriptionStatusCanceled || sub.Status == stripe.SubscriptionStatusUnpaid {
		if err := fb.UpdateUserPlan(ctx, uid, "free"); err != nil {
			slog.Error("stripe: downgrade failed", "uid", uid, "err", err)
		}
		return
	}

	plan := planFromSub(sub)
	if plan == "" {
		slog.Warn("stripe: unknown price in subscription", "sub", sub.ID)
		return
	}
	if err := fb.UpdateUserPlan(ctx, uid, plan); err != nil {
		slog.Error("stripe: upgrade failed", "uid", uid, "plan", plan, "err", err)
	}
}

func planFromSub(sub stripe.Subscription) string {
	cfg := config.Get()
	for _, item := range sub.Items.Data {
		switch item.Price.ID {
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
