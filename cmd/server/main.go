package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	httpadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	appuser "github.com/MoNezhadali/foodscheduler/internal/application/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/config"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := database.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.RunMigrations(db, migrations.SQLiteFS, "sqlite"); err != nil {
		log.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	// Infrastructure
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	tokenSvc := auth.NewJWTService(jwtSecret)

	// Repositories
	userRepo := sqliteadapter.NewUserRepo(db)
	ingRepo := sqliteadapter.NewIngredientRepo(db)

	// Use-cases
	registerUC := appuser.NewRegisterUseCase(userRepo)
	loginUC := appuser.NewLoginUseCase(userRepo, tokenSvc)
	refreshUC := appuser.NewRefreshUseCase(userRepo, tokenSvc)

	// Handlers
	healthHandler := handlers.NewHealthHandler(db)
	userHandler := handlers.NewUserHandler(registerUC, loginUC, refreshUC)
	ingHandler := handlers.NewIngredientHandler(ingRepo)

	// Router
	router := httpadapter.NewRouter(httpadapter.RouterDeps{
		Logger:     log,
		TokenSvc:   tokenSvc,
		Health:     healthHandler,
		User:       userHandler,
		Ingredient: ingHandler,
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Info("server starting", "addr", addr, "env", cfg.Env)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}
