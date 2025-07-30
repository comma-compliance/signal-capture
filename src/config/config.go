package config

import (
	"os"
	"time"
	"sync"
)

var (
    once     sync.Once
    instance *Config
)

func GetConfig() *Config {
    once.Do(func() {
        instance = LoadConfig() // or read and parse file
    })
    return instance
}

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
		PrivateKey:   getEnv("SIGNAL_PRIVATE_KEY", ""),
		PublicKey:    getEnv("SIGNAL_PUBLIC_KEY", ""),
		AppPubKey:    getEnv("APP_PUBLIC_KEY", ""),
	}
}

// getEnv returns the value of the environment variable or a default if not set
func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
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
