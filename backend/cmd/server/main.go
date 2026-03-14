// Package main is the entry point for the bookshelf API server.
package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/rs/cors"

	"github.com/tanjd/bookshelf/internal/config"
	"github.com/tanjd/bookshelf/internal/db"
	"github.com/tanjd/bookshelf/internal/handlers"
	appmiddleware "github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/services"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		panic("failed to open database: " + err.Error())
	}

	// Services
	emailSvc := services.NewEmailService(cfg.ResendAPIKey, cfg.EmailFrom)
	workflow := services.NewLoanWorkflow(database, emailSvc)

	// Handlers
	authH := handlers.NewAuthHandler(database, cfg.JWTSecret)
	bookH := handlers.NewBookHandler(database)
	copyH := handlers.NewCopyHandler(database)
	loanH := handlers.NewLoanRequestHandler(database, workflow)
	notifH := handlers.NewNotificationHandler(database)

	// Router
	mux := http.NewServeMux()

	// Health check (plain net/http, outside huma)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Huma API — auto-generates OpenAPI spec at /openapi.yaml + /openapi.json
	// and serves interactive docs at /docs
	apiConfig := huma.DefaultConfig("Bookshelf API", "1.0.0")
	apiConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}
	api := humago.New(mux, apiConfig)

	// Register routes
	authH.RegisterRoutes(api)
	bookH.RegisterRoutes(api)
	copyH.RegisterRoutes(api)
	loanH.RegisterRoutes(api)
	notifH.RegisterRoutes(api)

	// Middleware chain: CORS → auth enrichment → mux
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPatch,
			http.MethodPut, http.MethodDelete, http.MethodOptions,
		},
		AllowedHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	})

	handler := corsHandler.Handler(appmiddleware.SetAuth(cfg.JWTSecret)(mux))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("bookshelf API starting", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		panic("server failed: " + err.Error())
	}
}
