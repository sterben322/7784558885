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

	if err := database.InitDB(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}
	defer database.CloseDB()

	if err := database.Ping(context.Background()); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	database.CreateTables()

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

	r.GET("/api/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
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
	})

	routes.RegisterProtectedRoutes(r)

	r.Static("/assets", "./web/assets")

	pageRoutes := map[string]string{
		"/":                      "index.html",
		"/dashboard":             "dashboard.html",
		"/dashboard.html":        "dashboard.html",
		"/community":             "community.html",
		"/community.html":        "community.html",
		"/company":               "company.html",
		"/company.html":          "company.html",
		"/company-profile":       "company-profile.html",
		"/company-profile.html":  "company-profile.html",
		"/create-community":      "create-community.html",
		"/create-community.html": "create-community.html",
		"/create-company":        "create-company.html",
		"/create-company.html":   "create-company.html",
		"/employees":             "employees.html",
		"/employees.html":        "employees.html",
		"/forum":                 "forum.html",
		"/forum.html":            "forum.html",
		"/chat":                  "chat.html",
		"/chat.html":             "chat.html",
		"/settings":              "settings.html",
		"/settings.html":         "settings.html",
		"/profile":               "profile.html",
		"/profile.html":          "profile.html",
		"/jobs":                  "jobs.html",
		"/jobs.html":             "jobs.html",
		"/projects":              "projects.html",
		"/projects.html":         "projects.html",
		"/exhibitions":           "exhibitions.html",
		"/exhibitions.html":      "exhibitions.html",
		"/events":                "events.html",
		"/events.html":           "events.html",
		"/privacy":               "privacy.html",
		"/privacy.html":          "privacy.html",
		"/terms":                 "terms.html",
		"/terms.html":            "terms.html",
		"/change-password":       "change-password.html",
		"/change-password.html":  "change-password.html",
		"/stub":                  "stub.html",
		"/stub.html":             "stub.html",
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
