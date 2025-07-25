package config

import (
	"log"
	"os"
	"time"
)

// Config holds all the configuration values used throughout the app
type Config struct {
	WebSocketURL string
	WebhookURL   string
	BatchSize    string
	JobId        string
	PrivateKey   string
	PublicKey	 string
	AppPubKey    string
}

// LoadConfig initializes configuration with env variables or default values
func LoadConfig() *Config {
	return &Config{
		WebSocketURL: getEnv("WEBSOCKET_URL", "ws://100.91.74.113:3000/cable?token=this_should_never_be_in_prod"),
		WebhookURL:   getEnv("WEBHOOK_URL", "http://100.91.74.113:3000/webhooks/incoming/signal_webhooks"),
		BatchSize:    getEnv("BATCH_SIZE", "50"),
		JobId:        getJobID(),
		PrivateKey:   getEnv("PRIVATE_KEY", "XLteChPpSK+9y7fU6WNTOX0tSLyyTrekhygfJTLN3/E="),
		PublicKey:    getEnv("PUBLIC_KEY", "Dx841BPrnzzgZQos4wAe4KHayyv0vZwSUgzaBEk4axE="),
		AppPubKey:    getEnv("RAILS_PUBLIC_KEY", "9sjyElWmHnZkYrHz6/dlKViDBR7kvuT9db0sgSBPs2Q="),
	}
}

// getEnv returns the value of the environment variable or a default if not set
func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Printf("⚠️ %s not set. Using default: %s", key)
		return defaultVal
	}
	return val
}

func getJobID() string {
	envJobID := os.Getenv("JOB_ID")
	if envJobID != "" {
		return envJobID
	}
	ts := time.Now().Format("20060102150405")
	return ts
}
