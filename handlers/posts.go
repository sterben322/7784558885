package handlers

import (
	"database/sql"
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func CreatePost(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	authorType := c.DefaultQuery("author_type", "user")
	if req.PrivacyLevel == "" {
		req.PrivacyLevel = "public"
	}

	var authorName string
	var targetID *uuid.UUID

	switch authorType {
	case "user":
		if err := database.DB.QueryRow(`SELECT full_name FROM users WHERE id = $1`, userID).Scan(&authorName); err != nil {
			jsonError(c, http.StatusBadRequest, "User not found")
			return
		}
	case "community":
		if req.TargetID == nil {
			jsonError(c, http.StatusBadRequest, "target_id is required for community posts")
			return
		}
		parsed, err := uuid.Parse(*req.TargetID)
		if err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid target_id")
			return
		}
		targetID = &parsed
		if err := database.DB.QueryRow(`SELECT name FROM communities WHERE id = $1`, parsed).Scan(&authorName); err != nil {
			jsonError(c, http.StatusBadRequest, "Community not found")
			return
		}
	case "company":
		if req.TargetID == nil {
			jsonError(c, http.StatusBadRequest, "target_id is required for company posts")
			return
		}
		parsed, err := uuid.Parse(*req.TargetID)
		if err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid target_id")
			return
		}
		targetID = &parsed
		if err := database.DB.QueryRow(`SELECT name FROM companies WHERE id = $1`, parsed).Scan(&authorName); err != nil {
			jsonError(c, http.StatusBadRequest, "Company not found")
			return
		}
		allowed, err := requireCompanyPermission(parsed.String(), userID, "publish_news")
		if err != nil || !allowed {
			jsonError(c, http.StatusForbidden, "Insufficient permissions to publish company news")
			return
		}
		if req.PrivacyLevel == "" {
			req.PrivacyLevel = "public"
		}
	default:
		jsonError(c, http.StatusBadRequest, "Invalid author_type")
		return
	}

	postID := uuid.New()
	_, err := database.DB.Exec(`
        INSERT INTO posts (id, author_id, author_type, author_name, title, content, short_description, image_url, tags, privacy_level, target_id, is_hidden, is_unpublished)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `, postID, userID, authorType, authorName, req.Title, req.Content, req.ShortDescription, req.ImageURL, pq.Array(req.Tags), req.PrivacyLevel, targetID, req.IsHidden, req.IsUnpublished)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create post")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Post created", "post_id": postID})
}

func GetFeed(c *gin.Context) {
	userID := currentUserID(c)
	feedType := c.DefaultQuery("type", "global")

	var query string
	var args []interface{}

	switch feedType {
	case "global":
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.privacy_level = 'public' AND p.is_hidden = false AND p.is_unpublished = false
            ORDER BY p.created_at DESC
            LIMIT 50
        `
		args = []interface{}{userID}
	case "friends":
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.author_type = 'user'
              AND p.is_hidden = false AND p.is_unpublished = false
              AND (p.privacy_level = 'public' OR (p.privacy_level = 'friends' AND EXISTS (
                    SELECT 1 FROM user_friends uf
                    WHERE ((uf.user_id = p.author_id AND uf.friend_id = $1) OR (uf.user_id = $1 AND uf.friend_id = p.author_id))
                      AND uf.status = 'accepted'
              )))
            ORDER BY p.created_at DESC
            LIMIT 50
        `
		args = []interface{}{userID}
	case "my":
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.author_id = $1 AND p.author_type = 'user' AND p.is_hidden = false AND p.is_unpublished = false
            ORDER BY p.created_at DESC
            LIMIT 50
        `
		args = []interface{}{userID}
	case "community":
		communityID := c.Query("community_id")
		if communityID == "" {
			jsonError(c, http.StatusBadRequest, "community_id is required for community feed")
			return
		}
		var isPrivate bool
		if err := database.DB.QueryRow(`SELECT is_private FROM communities WHERE id = $1`, communityID).Scan(&isPrivate); err == nil && isPrivate && !isCommunityMember(communityID, userID) {
			jsonError(c, http.StatusForbidden, "Community posts are hidden until your request is approved")
			return
		}
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.author_type = 'community' AND p.target_id = $2 AND p.is_hidden = false AND p.is_unpublished = false
              AND (p.privacy_level = 'public' OR (p.privacy_level = 'members' AND EXISTS (
                   SELECT 1 FROM community_members WHERE community_id = $2 AND user_id = $1
              )))
            ORDER BY p.created_at DESC
            LIMIT 50
        `
		args = []interface{}{userID, communityID}
	case "company":
		companyID := c.Query("company_id")
		if companyID == "" {
			jsonError(c, http.StatusBadRequest, "company_id is required for company feed")
			return
		}
		var isPublic bool
		if err := database.DB.QueryRow(`SELECT is_public FROM companies WHERE id = $1`, companyID).Scan(&isPublic); err != nil {
			jsonError(c, http.StatusNotFound, "Company not found")
			return
		}
		if !isPublic && !isCompanyMember(companyID, userID) {
			jsonError(c, http.StatusForbidden, "Company posts are hidden until your request is approved")
			return
		}
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.author_type = 'company' AND p.target_id = $2
              AND p.is_hidden = false AND p.is_unpublished = false
              AND (p.privacy_level = 'public' OR (p.privacy_level = 'members' AND EXISTS (
                   SELECT 1 FROM company_members WHERE company_id = $2 AND user_id = $1
              )))
            ORDER BY p.created_at DESC
            LIMIT 50
        `
		args = []interface{}{userID, companyID}
	case "news":
		query = `
            SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
                   p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
                   p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
                   EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
            FROM posts p
            WHERE p.author_type IN ('community', 'company')
              AND p.privacy_level = 'public'
              AND p.is_hidden = false
              AND p.is_unpublished = false
            ORDER BY p.created_at DESC LIMIT 50
        `
		args = []interface{}{userID}
	default:
		jsonError(c, http.StatusBadRequest, "Invalid feed type. Allowed: global, friends, my, community, company, news")
		return
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		var title, shortDesc, imageURL sql.NullString
		var targetID sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.AuthorType, &post.AuthorName, &post.AuthorAvatar, &title, &post.Content, &shortDesc, &imageURL, &tags, &post.PrivacyLevel, &targetID, &post.IsHidden, &post.IsUnpublished, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt, &post.IsLiked); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if title.Valid {
			post.Title = &title.String
		}
		if shortDesc.Valid {
			post.ShortDescription = &shortDesc.String
		}
		if imageURL.Valid {
			post.ImageURL = &imageURL.String
		}
		if targetID.Valid {
			parsed, err := uuid.Parse(targetID.String)
			if err == nil {
				post.TargetID = &parsed
			}
		}
		post.Tags = tags
		posts = append(posts, post)
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func GetPost(c *gin.Context) {
	postID := c.Param("id")
	userID := currentUserID(c)

	var post models.Post
	var title, shortDesc, imageURL, targetID sql.NullString
	var tags pq.StringArray
	err := database.DB.QueryRow(`
        SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
               p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
               p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
        FROM posts p WHERE p.id = $2
    `, userID, postID).Scan(&post.ID, &post.AuthorID, &post.AuthorType, &post.AuthorName, &post.AuthorAvatar, &title, &post.Content, &shortDesc, &imageURL, &tags, &post.PrivacyLevel, &targetID, &post.IsHidden, &post.IsUnpublished, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt, &post.IsLiked)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Post not found")
		return
	}
	if title.Valid {
		post.Title = &title.String
	}
	if shortDesc.Valid {
		post.ShortDescription = &shortDesc.String
	}
	if imageURL.Valid {
		post.ImageURL = &imageURL.String
	}
	if targetID.Valid {
		parsed, err := uuid.Parse(targetID.String)
		if err == nil {
			post.TargetID = &parsed
		}
	}
	post.Tags = tags
	c.JSON(http.StatusOK, gin.H{"post": post})
}

func LikePost(c *gin.Context) {
	postID := c.Param("id")
	userID := currentUserID(c)
	if _, err := database.DB.Exec(`INSERT INTO post_likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to like post")
		return
	}
	recalcPostLikes(postID)
	c.JSON(http.StatusOK, gin.H{"message": "Post liked"})
}

func UnlikePost(c *gin.Context) {
	postID := c.Param("id")
	userID := currentUserID(c)
	if _, err := database.DB.Exec(`DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`, postID, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to unlike post")
		return
	}
	recalcPostLikes(postID)
	c.JSON(http.StatusOK, gin.H{"message": "Post unliked"})
}

func AddComment(c *gin.Context) {
	postID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	var authorName string
	if err := database.DB.QueryRow(`SELECT full_name FROM users WHERE id = $1`, userID).Scan(&authorName); err != nil {
		jsonError(c, http.StatusBadRequest, "User not found")
		return
	}
	commentID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO post_comments (id, post_id, author_id, author_name, content) VALUES ($1, $2, $3, $4, $5)`, commentID, postID, userID, authorName, req.Content); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to add comment")
		return
	}
	recalcPostComments(postID)
	c.JSON(http.StatusCreated, gin.H{"message": "Comment added"})
}

func GetComments(c *gin.Context) {
	postID := c.Param("id")
	rows, err := database.DB.Query(`SELECT id, author_id, author_name, content, likes_count, created_at FROM post_comments WHERE post_id = $1 ORDER BY created_at ASC`, postID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	comments := make([]models.Comment, 0)
	parsedPostID, _ := uuid.Parse(postID)
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(&comment.ID, &comment.AuthorID, &comment.AuthorName, &comment.Content, &comment.LikesCount, &comment.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		comment.PostID = parsedPostID
		comments = append(comments, comment)
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

func GetNews(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
               p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
               p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
        FROM posts p
        WHERE p.author_type IN ('community', 'company')
          AND p.privacy_level = 'public'
          AND p.is_hidden = false
          AND p.is_unpublished = false
        ORDER BY p.created_at DESC
        LIMIT 100
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		var title, shortDesc, imageURL, targetID sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.AuthorType, &post.AuthorName, &post.AuthorAvatar, &title, &post.Content, &shortDesc, &imageURL, &tags, &post.PrivacyLevel, &targetID, &post.IsHidden, &post.IsUnpublished, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt, &post.IsLiked); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if title.Valid {
			post.Title = &title.String
		}
		if shortDesc.Valid {
			post.ShortDescription = &shortDesc.String
		}
		if imageURL.Valid {
			post.ImageURL = &imageURL.String
		}
		if targetID.Valid {
			if parsed, err := uuid.Parse(targetID.String); err == nil {
				post.TargetID = &parsed
			}
		}
		post.Tags = tags
		posts = append(posts, post)
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func GetWall(c *gin.Context) {
	userID := currentUserID(c)
	wallType := c.Param("type")
	entityID := c.Param("id")
	query := `
        SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
               p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
               p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS is_liked
        FROM posts p
        WHERE p.is_hidden = false AND p.is_unpublished = false
    `
	args := []interface{}{userID}
	switch wallType {
	case "user":
		query += " AND p.author_type = 'user' AND p.author_id = $2"
		args = append(args, entityID)
	case "community", "company":
		query += " AND p.author_type = '" + wallType + "' AND p.target_id = $2"
		args = append(args, entityID)
	default:
		jsonError(c, http.StatusBadRequest, "invalid wall type")
		return
	}
	query += " ORDER BY p.created_at DESC LIMIT 100"
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		var title, shortDesc, imageURL, targetID sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.AuthorType, &post.AuthorName, &post.AuthorAvatar, &title, &post.Content, &shortDesc, &imageURL, &tags, &post.PrivacyLevel, &targetID, &post.IsHidden, &post.IsUnpublished, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt, &post.IsLiked); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if title.Valid {
			post.Title = &title.String
		}
		if shortDesc.Valid {
			post.ShortDescription = &shortDesc.String
		}
		if imageURL.Valid {
			post.ImageURL = &imageURL.String
		}
		if targetID.Valid {
			if parsed, err := uuid.Parse(targetID.String); err == nil {
				post.TargetID = &parsed
			}
		}
		post.Tags = tags
		posts = append(posts, post)
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}
