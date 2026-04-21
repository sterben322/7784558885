package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lastop/config"
	"lastop/database"
	"lastop/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		const fallbackJWTSecret = "lastop-insecure-default-jwt-secret"
		log.Printf("WARNING: JWT_SECRET is empty, using fallback secret for startup")
		if err := os.Setenv("JWT_SECRET", fallbackJWTSecret); err != nil {
			log.Fatalf("failed to set fallback JWT_SECRET: %v", err)
		}
	}

	database.Startup(cfg.DatabaseURL)
	defer database.CloseDB()

	r := gin.Default()
	allowedOrigins := buildAllowedOrigins()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	routes.RegisterAuthRoutes(r)

	healthHandler := func(c *gin.Context) {
		if !database.IsConfigured() {
			c.JSON(http.StatusOK, gin.H{
				"status":   "degraded",
				"database": "down",
				"error":    "DATABASE_URL is not configured",
			})
			return
		}

		if database.DB == nil || !database.IsReady() {
			c.JSON(http.StatusOK, gin.H{
				"status":   "degraded",
				"database": "down",
				"error":    database.LastError(),
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":   "degraded",
				"database": "down",
				"error":    err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "up"})
	}
	r.GET("/api/health", healthHandler)
	r.HEAD("/api/health", healthHandler)

	routes.RegisterProtectedRoutes(r)

	r.Static("/assets", "./web/assets")
	r.Static("/uploads", "./web/uploads")
	if err := os.MkdirAll(filepath.Join("web", "uploads", "chat"), 0o755); err != nil {
		log.Fatalf("failed to create uploads directory: %v", err)
	}

	pageRoutes := map[string]string{
		"/":                       "index.html",
		"/dashboard":              "dashboard.html",
		"/dashboard.html":         "dashboard.html",
		"/community":              "community.html",
		"/community.html":         "community.html",
		"/community-profile":      "community-profile.html",
		"/community-profile.html": "community-profile.html",
		"/communities":            "communities.html",
		"/communities.html":       "communities.html",
		"/company":                "company.html",
		"/company.html":           "company.html",
		"/company-profile":        "company-profile.html",
		"/company-profile.html":   "company-profile.html",
		"/create-community":       "create-community.html",
		"/create-community.html":  "create-community.html",
		"/create-company":         "create-company.html",
		"/create-company.html":    "create-company.html",
		"/employees":              "employees.html",
		"/employees.html":         "employees.html",
		"/forum":                  "forum.html",
		"/forum.html":             "forum.html",
		"/friends":                "friends.html",
		"/friends.html":           "friends.html",
		"/chat":                   "chat.html",
		"/chat.html":              "chat.html",
		"/catalog":                "catalog.html",
		"/catalog.html":           "catalog.html",
		"/companies":              "companies.html",
		"/companies.html":         "companies.html",
		"/settings":               "settings.html",
		"/settings.html":          "settings.html",
		"/search":                 "search.html",
		"/search.html":            "search.html",
		"/profile":                "profile.html",
		"/profile.html":           "profile.html",
		"/jobs":                   "jobs.html",
		"/jobs.html":              "jobs.html",
		"/projects":               "projects.html",
		"/projects.html":          "projects.html",
		"/analytics":              "analytics.html",
		"/analytics.html":         "analytics.html",
		"/exhibitions":            "exhibitions.html",
		"/exhibitions.html":       "exhibitions.html",
		"/events":                 "events.html",
		"/events.html":            "events.html",
		"/privacy":                "privacy.html",
		"/privacy.html":           "privacy.html",
		"/terms":                  "terms.html",
		"/terms.html":             "terms.html",
		"/change-password":        "change-password.html",
		"/change-password.html":   "change-password.html",
		"/stub":                   "stub.html",
		"/stub.html":              "stub.html",
	}
	for route, file := range pageRoutes {
		routeCopy := route
		fileCopy := file
		r.GET(routeCopy, func(c *gin.Context) { c.File(filepath.Join("web", fileCopy)) })
	}

	r.GET("/login", func(c *gin.Context) {
		c.File(filepath.Join("web", "login.html"))
	})
	r.GET("/login.html", func(c *gin.Context) {
		c.File(filepath.Join("web", "login.html"))
	})
	r.GET("/register", func(c *gin.Context) {
		c.File(filepath.Join("web", "register.html"))
	})
	r.GET("/register.html", func(c *gin.Context) {
		c.File(filepath.Join("web", "register.html"))
	})

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		c.File(filepath.Join("web", "index.html"))
	})

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("Server running on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func buildAllowedOrigins() []string {
	defaults := []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://localhost:3000", "http://127.0.0.1:3000"}
	if railwayDomain := strings.TrimSpace(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); railwayDomain != "" {
		defaults = append(defaults, "https://"+railwayDomain)
	}
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		return defaults
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return defaults
	}
	return origins
}
