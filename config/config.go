package config

import (
	"fmt"
	"os"
)

// AppConfig stores runtime configuration loaded from environment variables.
type AppConfig struct {
	Host        string
	Port        string
	DatabaseURL string
}

// Load reads application configuration from environment variables.
func Load() (*AppConfig, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && os.Getenv("TEST_MODE") != "1" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &AppConfig{
		Host:        "0.0.0.0",
		Port:        port,
		DatabaseURL: databaseURL,
	}, nil
}
