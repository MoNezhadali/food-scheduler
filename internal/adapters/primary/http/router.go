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
	Logger   *slog.Logger
	TokenSvc auth.Service
	Health   *handlers.HealthHandler
	User     *handlers.UserHandler
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

		// Protected routes — populated from Phase 6 onward
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(deps.TokenSvc))
		})
	})

	return r
}
