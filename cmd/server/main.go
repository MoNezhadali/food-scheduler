package main

import (
	"log/slog"
	"os"

	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("FoodScheduler starting", "env", cfg.Env, "port", cfg.Port, "db_driver", cfg.DBDriver)
	// HTTP server, DB connection, and dependency wiring added in Phase 5.
}
