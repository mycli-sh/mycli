package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/database"
	"mycli.sh/api/internal/email"
	"mycli.sh/api/internal/handler"
	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	flag.Parse()

	cfg := config.Load()

	// Connect to database
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	if *migrateOnly {
		fmt.Println("migrations complete")
		return
	}

	// Initialize unified store
	s := store.New(pool)

	// Initialize email sender
	var emailSender email.Sender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom)
	} else {
		log.Println("WARNING: RESEND_API_KEY not set, using dev log sender for emails")
		emailSender = &email.LogSender{}
	}

	// Initialize handlers
	authHandler := handler.NewAuthHandler(cfg, s, emailSender)
	cmdHandler := handler.NewCommandHandler(s)
	catalogHandler := handler.NewCatalogHandler(s)
	meHandler := handler.NewMeHandler(s)
	libraryHandler := handler.NewLibraryHandler(cfg, s)
	sessionHandler := handler.NewSessionHandler(s)
	webAuthHandler := handler.NewWebAuthHandler(cfg, s, emailSender)

	// Rate limiters
	authLimiter := middleware.NewRateLimiter(1, 10) // 1 req/sec, burst 10 for auth
	apiLimiter := middleware.NewRateLimiter(10, 50) // 10 req/sec, burst 50 for API

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"ETag"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// Public routes (no auth)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(authLimiter, middleware.IPKey))

		// Device flow
		r.Post("/v1/auth/device/start", authHandler.StartDeviceFlow)
		r.Post("/v1/auth/device/token", authHandler.PollDeviceToken)
		r.Post("/v1/auth/device/resend", authHandler.ResendVerification)
		r.Post("/v1/auth/verify-code", authHandler.VerifyOTP)
		r.Post("/v1/auth/refresh", authHandler.RefreshTokenHandler)

		// Device verification pages (HTML)
		r.Get("/device", authHandler.DevicePage)
		r.Post("/device", authHandler.DeviceSubmit)
		r.Get("/v1/auth/verify", authHandler.VerifyMagicLink)

		// Username availability (public)
		r.Get("/v1/usernames/{username}/available", meHandler.CheckUsernameAvailable)

		// Web auth flow
		r.Post("/v1/auth/web/login", webAuthHandler.Login)
		r.Post("/v1/auth/web/verify", webAuthHandler.Verify)
	})

	// Library browsing routes (optional auth — works for anonymous + authenticated)
	r.Group(func(r chi.Router) {
		r.Use(middleware.OptionalAuth(cfg.JWTSecret))
		r.Use(middleware.RateLimit(apiLimiter, middleware.IPKey))

		r.Get("/v1/libraries", libraryHandler.Search)
		r.Get("/v1/libraries/{owner}/{slug}", libraryHandler.GetDetail)
		r.Get("/v1/libraries/{owner}/{slug}/releases", libraryHandler.ListReleases)
		r.Get("/v1/libraries/{owner}/{slug}/releases/{version}", libraryHandler.GetRelease)
		r.Get("/v1/libraries/{owner}/{slug}/commands/{commandSlug}", libraryHandler.GetCommand)
		r.Get("/v1/libraries/{owner}/{slug}/commands/{commandSlug}/versions", libraryHandler.ListCommandVersions)
	})

	// Authenticated routes (no username required)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))
		r.Use(middleware.RateLimit(apiLimiter, middleware.UserKey))

		// Me
		r.Get("/v1/me", meHandler.GetMe)
		r.Patch("/v1/me/username", meHandler.SetUsername)

		// Sessions
		r.Get("/v1/sessions", sessionHandler.List)
		r.Delete("/v1/sessions/{id}", sessionHandler.Revoke)
		r.Delete("/v1/sessions", sessionHandler.RevokeAll)
	})

	// Authenticated routes (username required)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))
		r.Use(middleware.RequireUsername(s))
		r.Use(middleware.RateLimit(apiLimiter, middleware.UserKey))

		// Me
		r.Get("/v1/me/sync-summary", meHandler.SyncSummary)

		// Commands
		r.Post("/v1/commands", cmdHandler.Create)
		r.Get("/v1/commands", cmdHandler.List)
		r.Get("/v1/commands/{id}", cmdHandler.Get)
		r.Delete("/v1/commands/{id}", cmdHandler.Delete)
		r.Post("/v1/commands/{id}/versions", cmdHandler.PublishVersion)
		r.Get("/v1/commands/{id}/versions/{version}", cmdHandler.GetVersion)

		// Catalog
		r.Get("/v1/catalog", catalogHandler.GetCatalog)

		// Libraries
		r.Post("/v1/libraries/{slug}/releases", libraryHandler.CreateRelease)
		r.Post("/v1/libraries/{owner}/{slug}/install", libraryHandler.Install)
		r.Delete("/v1/libraries/{owner}/{slug}/install", libraryHandler.Uninstall)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\nshutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	fmt.Printf("mycli api server listening on :%s\n", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
