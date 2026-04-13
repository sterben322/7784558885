package config

import (
	"os"
)

// AppConfig stores runtime configuration loaded from environment variables.
type AppConfig struct {
	Host               string
	Port               string
	DatabaseURL        string
	JWTSecret          string
	CORSAllowedOrigins string
}

// Load reads application configuration from environment variables.
func Load() (*AppConfig, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")

	return &AppConfig{
		Host:               "0.0.0.0",
		Port:               port,
		DatabaseURL:        databaseURL,
		JWTSecret:          os.Getenv("JWT_SECRET"),
		CORSAllowedOrigins: os.Getenv("CORS_ALLOWED_ORIGINS"),
	}, nil
}
