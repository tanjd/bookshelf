// Package main is the entry point for the bookshelf API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/rs/cors"

	"github.com/tanjd/bookshelf/internal/config"
	"github.com/tanjd/bookshelf/internal/db"
	"github.com/tanjd/bookshelf/internal/handlers"
	appmiddleware "github.com/tanjd/bookshelf/internal/middleware"
	gormrepo "github.com/tanjd/bookshelf/internal/repository/gorm"
	"github.com/tanjd/bookshelf/internal/services"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		panic("failed to open database: " + err.Error())
	}

	db.Seed(database)

	coversDir := "./data/covers"
	if err := os.MkdirAll(coversDir, 0o750); err != nil {
		panic("failed to create covers dir: " + err.Error())
	}

	// Repositories — only this layer and db.Open ever import gorm.io/gorm.
	userRepo := gormrepo.NewUserRepository(database)
	bookRepo := gormrepo.NewBookRepository(database)
	copyRepo := gormrepo.NewCopyRepository(database)
	loanRepo := gormrepo.NewLoanRequestRepository(database)
	notifRepo := gormrepo.NewNotificationRepository(database)
	adminRepo := gormrepo.NewAdminRepository(database)
	waitlistRepo := gormrepo.NewWaitlistRepository(database)

	// Services
	emailSvc := services.NewEmailService(cfg.ResendAPIKey, cfg.EmailFrom)
	workflow := services.NewLoanWorkflow(copyRepo, loanRepo, notifRepo, userRepo, waitlistRepo, emailSvc)
	scheduler := services.NewScheduler(bookRepo, adminRepo, coversDir, cfg.MetadataRefreshInterval)

	// Handlers
	authH := handlers.NewAuthHandler(userRepo, cfg.JWTSecret, emailSvc)
	metadataH := handlers.NewMetadataHandler(cfg.GoogleBooksAPIKey)
	bookH := handlers.NewBookHandler(bookRepo, coversDir)
	copyH := handlers.NewCopyHandler(copyRepo, userRepo, notifRepo, waitlistRepo)
	loanH := handlers.NewLoanRequestHandler(copyRepo, loanRepo, adminRepo, workflow)
	notifH := handlers.NewNotificationHandler(notifRepo)
	adminH := handlers.NewAdminHandler(adminRepo)
	jobsH := handlers.NewJobsHandler(scheduler)
	waitlistH := handlers.NewWaitlistHandler(copyRepo, waitlistRepo)

	// Router
	mux := http.NewServeMux()

	// Static file serving for locally cached book covers.
	mux.Handle("/covers/", http.StripPrefix("/covers/", http.FileServer(http.Dir(coversDir))))

	// Health check (plain net/http, outside huma)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"` + version + `"}`))
	})

	// Huma API — auto-generates OpenAPI spec at /openapi.yaml + /openapi.json
	// and serves interactive docs at /docs
	apiConfig := huma.DefaultConfig("Bookshelf API", version)
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
	metadataH.RegisterRoutes(api)
	bookH.RegisterRoutes(api)
	copyH.RegisterRoutes(api)
	loanH.RegisterRoutes(api)
	notifH.RegisterRoutes(api)
	adminH.RegisterRoutes(api)
	jobsH.RegisterRoutes(api)
	waitlistH.RegisterRoutes(api)

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

	// Start the background scheduler.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go scheduler.Start(ctx)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("shutting down server")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			slog.Error("server shutdown error", "error", shutErr)
		}
	}()

	slog.Info("bookshelf API starting", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic("server failed: " + err.Error())
	}
}
