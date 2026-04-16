package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/wasp/helixtrace-api/internal/config"
	"github.com/wasp/helixtrace-api/internal/database"
	"github.com/wasp/helixtrace-api/internal/handlers"
	authmiddleware "github.com/wasp/helixtrace-api/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	conn, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}
	defer conn.Close()

	log.Println("connected to clickhouse")

	execDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}
	sqlDir := filepath.Join(execDir, "sql")

	if err := database.InitDB(conn, sqlDir); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	log.Println("database initialized")

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	if len(cfg.CorsAllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.CorsAllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			MaxAge:         300,
		}))
	}

	authHandler := &handlers.AuthHandler{Conn: conn}
	tracePathHandler := &handlers.TracePathHandler{Conn: conn, Cfg: cfg}
	pointHandler := &handlers.PointHandler{Conn: conn, Cfg: cfg}

	r.Post("/api/login", authHandler.Login)
	r.Post("/api/register", authHandler.Register)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authmiddleware.AuthMiddleware(conn, next.ServeHTTP)(w, r)
			})
		})

		r.Get("/api/profile", func(w http.ResponseWriter, r *http.Request) {
			email, _ := handlers.EmailFromContext(r.Context())
			handlers.WriteJSON(w, http.StatusOK, map[string]string{"email": email})
		})

		r.Get("/api/trace-path", tracePathHandler.TracePath)

		r.Post("/api/point", pointHandler.CreatePoint)
		r.Get("/api/point/{id}", pointHandler.GetPoint)
		r.Put("/api/point/{id}", pointHandler.UpdatePoint)
		r.Delete("/api/point/{id}", pointHandler.DeletePoint)
		r.Get("/api/points", pointHandler.ListPoints)
		r.Get("/api/point-categories", pointHandler.ListCategories)
	})

	addr := fmt.Sprintf("%s:%d", cfg.APIHost, cfg.APIPort)
	log.Printf("server starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
