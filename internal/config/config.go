package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Settings struct {
	Address    string
	BackendURL string
	RateLimit  int
	RateWindow time.Duration
}

func Load() (Settings, error) {
	settings := Settings{
		Address:    valueOrDefault("THROTTLEGUARD_ADDRESS", ":8080"),
		BackendURL: valueOrDefault("THROTTLEGUARD_BACKEND_URL", "http://localhost:9000"),
		RateLimit:  10,
		RateWindow: time.Second,
	}

	if rawLimit := os.Getenv("THROTTLEGUARD_RATE_LIMIT"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 {
			return Settings{}, fmt.Errorf("invalid THROTTLEGUARD_RATE_LIMIT %q", rawLimit)
		}
		settings.RateLimit = limit
	}

	if rawWindow := os.Getenv("THROTTLEGUARD_RATE_WINDOW"); rawWindow != "" {
		window, err := time.ParseDuration(rawWindow)
		if err != nil || window <= 0 {
			return Settings{}, fmt.Errorf("invalid THROTTLEGUARD_RATE_WINDOW %q", rawWindow)
		}
		settings.RateWindow = window
	}

	return settings, nil
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
