package config

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	Port                  int    `json:"port"`
	StripeSecretKey       string `json:"stripe_secret_key"`
	StripeWebhookSecret   string `json:"stripe_webhook_secret"`
	StripePricePro        string `json:"stripe_price_pro"`
	StripePriceBusiness   string `json:"stripe_price_business"`
	StripePriceEnterprise string `json:"stripe_price_enterprise"`
	FirebaseCredentials   string `json:"firebase_credentials"` // path to service account JSON
	FrontendDir           string `json:"frontend_dir"`         // path to frontend/ folder
}

var (
	once     sync.Once
	instance *Config
)

func Load(path string) *Config {
	once.Do(func() {
		instance = defaults()
		if data, err := os.ReadFile(path); err == nil {
			json.Unmarshal(data, instance)
		}
	})
	return instance
}

func Get() *Config {
	if instance != nil {
		return instance
	}
	return Load("config.json")
}

func defaults() *Config {
	return &Config{
		Port:                11501,
		FirebaseCredentials: "firebase-credentials.json",
		FrontendDir:         "../frontend",
	}
}
