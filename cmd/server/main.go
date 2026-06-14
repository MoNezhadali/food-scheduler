package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	httpadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	pgadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/postgres"
	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	appuser "github.com/MoNezhadali/foodscheduler/internal/application/user"
	domfood "github.com/MoNezhadali/foodscheduler/internal/domain/food"
	doming "github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/config"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"

	"database/sql"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var (
		db       *sql.DB
		userRepo domuser.Repository
		ingRepo  doming.Repository
		foodRepo domfood.Repository
	)

	switch cfg.DBDriver {
	case "postgres":
		if cfg.DBURL == "" {
			log.Error("DB_URL must be set when DB_DRIVER=postgres")
			os.Exit(1)
		}
		db, err = database.OpenPostgres(cfg.DBURL)
		if err != nil {
			log.Error("failed to open postgres", "error", err)
			os.Exit(1)
		}
		if err := database.RunMigrations(db, migrations.PostgresFS, "postgres"); err != nil {
			log.Error("migrations failed", "error", err)
			os.Exit(1)
		}
		userRepo = pgadapter.NewUserRepo(db)
		ingRepo = pgadapter.NewIngredientRepo(db)
		foodRepo = pgadapter.NewFoodRepo(db)

	default: // "sqlite" or unset
		db, err = database.OpenSQLite(cfg.DBPath)
		if err != nil {
			log.Error("failed to open database", "error", err)
			os.Exit(1)
		}
		if err := database.RunMigrations(db, migrations.SQLiteFS, "sqlite"); err != nil {
			log.Error("migrations failed", "error", err)
			os.Exit(1)
		}
		userRepo = sqliteadapter.NewUserRepo(db)
		ingRepo = sqliteadapter.NewIngredientRepo(db)
		foodRepo = sqliteadapter.NewFoodRepo(db)
	}
	defer db.Close()

	// Infrastructure
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	tokenSvc := auth.NewJWTService(jwtSecret)

	// Use-cases
	registerUC := appuser.NewRegisterUseCase(userRepo)
	loginUC := appuser.NewLoginUseCase(userRepo, tokenSvc)
	refreshUC := appuser.NewRefreshUseCase(userRepo, tokenSvc)

	// Handlers
	healthHandler := handlers.NewHealthHandler(db)
	userHandler := handlers.NewUserHandler(registerUC, loginUC, refreshUC)
	meHandler := handlers.NewMeHandler(userRepo)
	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)
	slHandler := handlers.NewShoppingListHandler(foodRepo, ingRepo)
	mpHandler := handlers.NewMealPlanHandler(foodRepo)

	// Router
	router := httpadapter.NewRouter(httpadapter.RouterDeps{
		Logger:       log,
		TokenSvc:     tokenSvc,
		CORSOrigins:  cfg.CORSOrigins,
		Health:       healthHandler,
		User:         userHandler,
		Me:           meHandler,
		Ingredient:   ingHandler,
		Food:         foodHandler,
		ShoppingList: slHandler,
		MealPlan:     mpHandler,
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Info("server starting", "addr", addr, "env", cfg.Env, "driver", cfg.DBDriver)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}
