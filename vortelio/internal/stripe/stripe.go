package stripe

import (
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/vortelio/vortelio/internal/config"
)

// Init sets the Stripe secret key from config.
func Init() {
	stripe.Key = config.Get().StripeSecretKey
}

// Enabled reports whether Stripe is configured.
func Enabled() bool {
	return config.Get().StripeSecretKey != ""
}

// PlanPriceID returns the Stripe Price ID for the given plan name.
func PlanPriceID(plan string) string {
	cfg := config.Get()
	switch plan {
	case "pro":
		return cfg.StripePricePro
	case "business":
		return cfg.StripePriceBusiness
	case "enterprise":
		return cfg.StripePriceEnterprise
	}
	return ""
}

// CreateCheckoutSession creates a Stripe Checkout session for the given plan.
// successURL and cancelURL are the redirect targets after checkout.
func CreateCheckoutSession(priceID, customerEmail, successURL, cancelURL string) (string, error) {
	Init()
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL:    stripe.String(successURL),
		CancelURL:     stripe.String(cancelURL),
		CustomerEmail: stripe.String(customerEmail),
	}
	s, err := session.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}
