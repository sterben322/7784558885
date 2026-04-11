package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lastop/database"
	"lastop/handlers"
	"lastop/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	database.InitDB()
	database.CreateTables()
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

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
	}

	r.GET("/api/health", func(c *gin.Context) {
		if !database.IsConfigured() {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "disabled"})
			return
		}

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

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/auth/logout", handlers.Logout)
		api.GET("/me", handlers.GetMe)
		api.PUT("/profile", handlers.UpdateProfile)

		api.GET("/friends", handlers.GetFriends)
		api.GET("/friends/requests", handlers.GetFriendRequests)
		api.POST("/friends/:id/request", handlers.SendFriendRequest)
		api.POST("/friends/:id/accept", handlers.AcceptFriendRequest)
		api.DELETE("/friends/:id/reject", handlers.RejectFriendRequest)
		api.DELETE("/friends/:id/remove", handlers.RemoveFriend)

		api.GET("/communities", handlers.GetCommunities)
		api.GET("/communities/my", handlers.GetMyCommunities)
		api.GET("/communities/:id", handlers.GetCommunity)
		api.POST("/communities", handlers.CreateCommunity)
		api.PUT("/communities/:id", handlers.UpdateCommunity)
		api.DELETE("/communities/:id", handlers.DeleteCommunity)
		api.POST("/communities/:id/join", handlers.JoinCommunity)
		api.DELETE("/communities/:id/leave", handlers.LeaveCommunity)
		api.POST("/communities/:id/request", handlers.RequestJoinCommunity)
		api.GET("/communities/:id/requests", handlers.GetJoinRequests)
		api.POST("/communities/requests/:request_id/approve", handlers.ApproveJoinRequest)
		api.POST("/communities/requests/:request_id/reject", handlers.RejectJoinRequest)

		api.GET("/communities/:id/roles", handlers.GetCommunityRoles)
		api.GET("/communities/:id/members", handlers.GetCommunityMembersWithRoles)
		api.POST("/communities/:id/roles/assign", handlers.AssignCommunityRole)
		api.DELETE("/communities/:id/roles/:user_id", handlers.RemoveCommunityRole)
		api.POST("/communities/:id/invite", handlers.InviteToCommunity)
		api.POST("/communities/invites/:invite_id/accept", handlers.AcceptCommunityInvite)

		api.GET("/company", handlers.GetCompany)
		api.POST("/company", handlers.CreateCompany)
		api.PUT("/company", handlers.UpdateCompany)
		api.POST("/company/:id/follow", handlers.FollowCompany)
		api.DELETE("/company/:id/follow", handlers.UnfollowCompany)
		api.POST("/companies/:id/request", handlers.RequestJoinCompany)
		api.GET("/companies/:id/requests", handlers.GetCompanyJoinRequests)
		api.POST("/companies/requests/:request_id/approve", handlers.ApproveCompanyJoinRequest)
		api.POST("/companies/requests/:request_id/reject", handlers.RejectCompanyJoinRequest)

		api.GET("/companies/:id/roles", handlers.GetCompanyRoles)
		api.POST("/companies/:id/roles", handlers.CreateCompanyRole)
		api.GET("/companies/:id/employees", handlers.GetCompanyEmployees)
		api.POST("/companies/:id/invite", handlers.InviteToCompany)
		api.POST("/companies/invites/:invite_id/accept", handlers.AcceptCompanyInvite)
		api.PUT("/companies/:id/employees/:user_id", handlers.UpdateEmployeeRole)
		api.DELETE("/companies/:id/employees/:user_id", handlers.RemoveEmployee)

		api.POST("/posts", handlers.CreatePost)
		api.GET("/feed", handlers.GetFeed)
		api.GET("/news", handlers.GetNews)
		api.GET("/walls/:type/:id", handlers.GetWall)
		api.GET("/posts/:id", handlers.GetPost)
		api.POST("/posts/:id/like", handlers.LikePost)
		api.DELETE("/posts/:id/like", handlers.UnlikePost)
		api.GET("/posts/:id/comments", handlers.GetComments)
		api.POST("/posts/:id/comments", handlers.AddComment)

		api.GET("/topics", handlers.GetTopics)
		api.GET("/topics/:id", handlers.GetTopic)
		api.POST("/topics", handlers.CreateTopic)
		api.POST("/topics/:id/reply", handlers.AddReply)

		api.GET("/chats", handlers.GetChats)
		api.GET("/chats/:id/messages", handlers.GetMessages)
		api.POST("/chats/:id/messages", handlers.SendMessage)

		api.GET("/dashboard/stats", handlers.GetDashboardStats)
		api.GET("/resumes", handlers.GetResumes)
		api.GET("/resume/me", handlers.GetMyResume)
		api.POST("/resume", handlers.CreateOrUpdateResume)
		api.GET("/vacancies", handlers.GetVacancies)
		api.POST("/vacancies", handlers.CreateVacancy)

		api.GET("/search/communities", handlers.SearchCommunities)
		api.GET("/search/companies", handlers.SearchCompanies)
	}

	r.Static("/assets", "./web/assets")

	pageRoutes := map[string]string{
		"/":                      "index.html",
		"/login.html":            "login.html",
		"/register.html":         "register.html",
		"/dashboard.html":        "dashboard.html",
		"/community.html":        "community.html",
		"/company.html":          "company.html",
		"/create-community.html": "create-community.html",
		"/create-company.html":   "create-company.html",
		"/employees.html":        "employees.html",
		"/forum.html":            "forum.html",
		"/chat.html":             "chat.html",
		"/settings.html":         "settings.html",
		"/profile.html":          "profile.html",
		"/jobs.html":             "jobs.html",
	}
	for route, file := range pageRoutes {
		routeCopy := route
		fileCopy := file
		r.GET(routeCopy, func(c *gin.Context) { c.File(filepath.Join("web", fileCopy)) })
	}

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		c.File(filepath.Join("web", "index.html"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func buildAllowedOrigins() []string {
	defaults := []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://localhost:3000", "http://127.0.0.1:3000"}
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
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
