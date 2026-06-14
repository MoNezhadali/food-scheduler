package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env        string // "development" | "production"
	Port       int
	DBDriver   string // "sqlite" | "postgres"
	DBPath     string // SQLite file path (DBDriver == "sqlite")
	DBURL      string // Postgres DSN (DBDriver == "postgres")
	JWTSecret  string
	USDAAPIKey string
}

func Load() (Config, error) {
	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PORT %q: %w", v, err)
		}
		port = p
	}

	cfg := Config{
		Env:        getenv("ENV", "development"),
		Port:       port,
		DBDriver:   getenv("DB_DRIVER", "sqlite"),
		DBPath:     getenv("DB_PATH", "foodscheduler.db"),
		DBURL:      os.Getenv("DB_URL"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		USDAAPIKey: os.Getenv("USDA_API_KEY"),
	}

	if cfg.Env == "production" && cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
