package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"

	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/seed"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/config"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.OpenSQLite(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.RunMigrations(db, migrations.SQLiteFS, "sqlite"); err != nil {
		logger.Error("run migrations", "error", err)
		os.Exit(1)
	}

	fixtureFS, err := fs.Sub(seed.FixtureFS, "fixtures")
	if err != nil {
		logger.Error("load fixtures", "error", err)
		os.Exit(1)
	}

	seeder := seed.NewSeeder(
		sqliteadapter.NewIngredientRepo(db),
		sqliteadapter.NewFoodRepo(db),
		fixtureFS,
	)

	logger.Info("seeding database", "db", cfg.DBPath)
	result, err := seeder.Seed(context.Background())
	if err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}

	logger.Info("seed complete",
		"ingredients_inserted", result.IngredientsInserted,
		"ingredients_skipped", result.IngredientsSkipped,
		"foods_inserted", result.FoodsInserted,
		"foods_skipped", result.FoodsSkipped,
	)
}
