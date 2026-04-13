package routes

import (
	"lastop/handlers"
	"lastop/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes registers authentication endpoints.
func RegisterAuthRoutes(r *gin.Engine) {
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
	}

	// Compatibility endpoint requested for external clients.
	r.POST("/api/register", handlers.Register)
	r.GET("/api/public/companies/:id", handlers.GetPublicCompany)
	r.GET("/api/public/companies/:id/news", handlers.GetPublicCompanyNews)
}

// RegisterProtectedRoutes registers API endpoints that require JWT auth.
func RegisterProtectedRoutes(r *gin.Engine) {
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/auth/logout", handlers.Logout)
		api.GET("/auth/me", handlers.GetMe)
		api.GET("/me", handlers.GetMe)
		api.PUT("/profile", handlers.UpdateProfile)
		api.GET("/corporate-profile", handlers.GetMyCorporateProfile)
		api.POST("/corporate-profile", handlers.CreateCorporateProfile)

		api.GET("/friends", handlers.GetFriends)
		api.GET("/friends/requests/incoming", handlers.GetIncomingFriendRequests)
		api.GET("/friends/requests/outgoing", handlers.GetOutgoingFriendRequests)
		api.GET("/friends/candidates", handlers.GetAddableUsers)
		api.GET("/friends/status/:id", handlers.GetFriendStatus)
		api.POST("/friends/request/:id", handlers.SendFriendRequest)
		api.POST("/friends/accept/:id", handlers.AcceptFriendRequest)
		api.POST("/friends/reject/:id", handlers.RejectFriendRequest)
		api.POST("/friends/cancel/:id", handlers.CancelFriendRequest)
		api.DELETE("/friends/:id", handlers.RemoveFriend)

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
		api.POST("/companies/:id/corporate-profiles", handlers.CreateEmployeeCorporateProfile)
		api.POST("/companies/:id/invite", handlers.InviteToCompany)
		api.GET("/companies/invites", handlers.GetMyCompanyInvites)
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

		api.GET("/forum/sections", handlers.GetForumSections)
		api.POST("/forum/sections", handlers.CreateForumSection)
		api.GET("/forum/sections/:id/topics", handlers.GetSectionTopics)
		api.POST("/forum/sections/:id/topics", handlers.CreateSectionTopic)
		api.GET("/forum/topics/:id", handlers.GetTopicDiscussion)
		api.POST("/forum/topics/:id/posts", handlers.AddTopicPost)
		api.PUT("/forum/posts/:id", handlers.UpdateForumPost)

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
}
