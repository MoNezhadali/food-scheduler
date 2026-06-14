package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

type RouterDeps struct {
	Logger       *slog.Logger
	TokenSvc     auth.Service
	Health       *handlers.HealthHandler
	User         *handlers.UserHandler
	Ingredient   *handlers.IngredientHandler
	Food         *handlers.FoodHandler
	ShoppingList *handlers.ShoppingListHandler
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))

	r.Get("/health", deps.Health.Check)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", deps.User.Register)
			r.Post("/login", deps.User.Login)
			r.Post("/refresh", deps.User.Refresh)
		})

		// Protected routes (auth required for all)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(deps.TokenSvc))

			r.Route("/ingredients", func(r chi.Router) {
				r.Get("/", deps.Ingredient.List)
				r.Get("/{id}", deps.Ingredient.GetByID)
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/", deps.Ingredient.Create)
					r.Put("/{id}", deps.Ingredient.Update)
					r.Delete("/{id}", deps.Ingredient.Delete)
				})
			})

			r.Route("/foods", func(r chi.Router) {
				r.Get("/", deps.Food.List)
				r.Get("/{id}", deps.Food.GetByID)
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/", deps.Food.Create)
					r.Put("/{id}", deps.Food.Update)
					r.Delete("/{id}", deps.Food.Delete)
				})
			})

			r.Post("/shopping-list", deps.ShoppingList.Generate)
		})
	})

	return r
}
