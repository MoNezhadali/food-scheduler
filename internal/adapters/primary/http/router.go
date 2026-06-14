package httpadapter

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

const (
	maxBodyBytes    = 1 << 20 // 1 MB
	authRateMax     = 20      // auth endpoints: 20 req/min per IP
	defaultRateMax  = 300     // other endpoints: 300 req/min per IP
	requestTimeout  = 30 * time.Second
)

type RouterDeps struct {
	Logger       *slog.Logger
	TokenSvc     auth.Service
	CORSOrigins  []string
	Health       *handlers.HealthHandler
	Docs         *handlers.DocsHandler
	User         *handlers.UserHandler
	Me           *handlers.MeHandler
	Ingredient   *handlers.IngredientHandler
	Food         *handlers.FoodHandler
	ShoppingList *handlers.ShoppingListHandler
	MealPlan     *handlers.MealPlanHandler
}

func NewRouter(deps RouterDeps) http.Handler {
	corsOrigins := deps.CORSOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}

	r := chi.NewRouter()

	// Global middleware (applied to every request)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Timeout(requestTimeout))
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.CORS(corsOrigins))
	r.Use(middleware.Logging(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.BodyLimit(maxBodyBytes))
	r.Use(middleware.RateLimit(defaultRateMax, time.Minute))

	r.Get("/health", deps.Health.Check)
	r.Get("/openapi.yaml", deps.Docs.ServeSpec)
	r.Get("/docs", deps.Docs.ServeUI)

	r.Route("/v1", func(r chi.Router) {
		// Auth endpoints — stricter rate limit
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimit(authRateMax, time.Minute))
			r.Post("/auth/register", deps.User.Register)
			r.Post("/auth/login", deps.User.Login)
			r.Post("/auth/refresh", deps.User.Refresh)
		})

		// Protected routes (auth required for all)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(deps.TokenSvc))

			r.Get("/me", deps.Me.GetProfile)
			r.Get("/me/preferences", deps.Me.GetPreferences)
			r.Put("/me/preferences", deps.Me.UpdatePreferences)

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
			r.Post("/meal-plan", deps.MealPlan.Generate)
		})
	})

	return r
}
