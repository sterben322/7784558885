package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastop/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Community struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	ShortDesc     string         `json:"short_description"`
	Category      string         `json:"category"`
	AvatarURL     sql.NullString `json:"avatar_url"`
	BannerURL     sql.NullString `json:"banner_url"`
	Website       sql.NullString `json:"website"`
	Region        sql.NullString `json:"region"`
	Privacy       string         `json:"privacy"`
	Status        sql.NullString `json:"status"`
	Tags          pq.StringArray `json:"tags"`
	MembersCount  int            `json:"members_count"`
	PostsCount    int            `json:"posts_count"`
	ActivityCount int            `json:"activity_count"`
	OwnerID       uuid.UUID      `json:"owner_id"`
	CreatedAt     time.Time      `json:"created_at"`
	IsMember      bool           `json:"is_member"`
	IsOwner       bool           `json:"is_owner"`
}

type CommunityMember struct {
	UserID    uuid.UUID `json:"user_id"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	IsOnline  bool      `json:"is_online"`
	JoinedAt  time.Time `json:"joined_at"`
}

type CommunityPost struct {
	ID            uuid.UUID      `json:"id"`
	CommunityID   uuid.UUID      `json:"community_id"`
	AuthorID      uuid.UUID      `json:"author_id"`
	AuthorName    string         `json:"author_name"`
	AuthorAvatar  sql.NullString `json:"author_avatar"`
	Title         sql.NullString `json:"title"`
	Content       string         `json:"content"`
	ImageURL      sql.NullString `json:"image_url"`
	Tags          pq.StringArray `json:"tags"`
	LikesCount    int            `json:"likes_count"`
	CommentsCount int            `json:"comments_count"`
	IsLiked       bool           `json:"is_liked"`
	CreatedAt     time.Time      `json:"created_at"`
}

func GetCommunities(c *gin.Context) {
	userID := currentUserID(c)
	category := c.Query("category")
	search := c.Query("q")
	limit := queryInt(c, "limit", 30)
	offset := queryInt(c, "offset", 0)

	rows, err := database.DB.Query(`
		SELECT c.id, c.name, c.description,
		       COALESCE(c.short_description,''), COALESCE(c.category,''),
		       c.avatar_url, c.banner_url, c.website, c.region,
		       COALESCE(c.privacy, CASE WHEN c.is_private THEN 'closed' ELSE 'open' END) AS privacy,
		       c.status,
		       COALESCE(c.tags, c.search_tags, '{}'::text[]) AS tags,
		       COALESCE(c.members_count,0), COALESCE(c.posts_count,0), COALESCE(c.activity_count,0),
		       c.owner_id, c.created_at,
		       EXISTS(SELECT 1 FROM community_members m WHERE m.community_id=c.id AND m.user_id=$1) AS is_member,
		       (c.owner_id = $1) AS is_owner
		FROM communities c
		WHERE c.deleted_at IS NULL
		  AND ($2 = '' OR COALESCE(c.category,'') ILIKE $2)
		  AND ($3 = '' OR c.name ILIKE '%' || $3 || '%' OR c.description ILIKE '%' || $3 || '%')
		ORDER BY c.members_count DESC, c.created_at DESC
		LIMIT $4 OFFSET $5`,
		userID, category, search, limit, offset,
	)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	communities := make([]Community, 0)
	for rows.Next() {
		var co Community
		if err := rows.Scan(
			&co.ID, &co.Name, &co.Description, &co.ShortDesc, &co.Category,
			&co.AvatarURL, &co.BannerURL, &co.Website, &co.Region, &co.Privacy,
			&co.Status, &co.Tags, &co.MembersCount, &co.PostsCount, &co.ActivityCount,
			&co.OwnerID, &co.CreatedAt, &co.IsMember, &co.IsOwner,
		); err != nil {
			continue
		}
		communities = append(communities, co)
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}

func GetMyCommunities(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT c.id, c.name, c.description,
		       COALESCE(c.short_description,''), COALESCE(c.category,''),
		       c.avatar_url, c.banner_url, c.website, c.region,
		       COALESCE(c.privacy, CASE WHEN c.is_private THEN 'closed' ELSE 'open' END) AS privacy,
		       c.status,
		       COALESCE(c.tags, c.search_tags, '{}'::text[]) AS tags,
		       COALESCE(c.members_count,0), COALESCE(c.posts_count,0), COALESCE(c.activity_count,0),
		       c.owner_id, c.created_at, true AS is_member, (c.owner_id = $1) AS is_owner
		FROM communities c
		INNER JOIN community_members m ON m.community_id = c.id AND m.user_id = $1
		WHERE c.deleted_at IS NULL
		ORDER BY m.joined_at DESC`, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	communities := make([]Community, 0)
	for rows.Next() {
		var co Community
		if err := rows.Scan(
			&co.ID, &co.Name, &co.Description, &co.ShortDesc, &co.Category,
			&co.AvatarURL, &co.BannerURL, &co.Website, &co.Region, &co.Privacy,
			&co.Status, &co.Tags, &co.MembersCount, &co.PostsCount, &co.ActivityCount,
			&co.OwnerID, &co.CreatedAt, &co.IsMember, &co.IsOwner,
		); err == nil {
			communities = append(communities, co)
		}
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}

func GetCommunity(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	var co Community
	err := database.DB.QueryRow(`
		SELECT c.id, c.name, c.description,
		       COALESCE(c.short_description,''), COALESCE(c.category,''),
		       c.avatar_url, c.banner_url, c.website, c.region,
		       COALESCE(c.privacy, CASE WHEN c.is_private THEN 'closed' ELSE 'open' END) AS privacy,
		       c.status,
		       COALESCE(c.tags, c.search_tags, '{}'::text[]) AS tags,
		       COALESCE(c.members_count,0), COALESCE(c.posts_count,0), COALESCE(c.activity_count,0),
		       c.owner_id, c.created_at,
		       EXISTS(SELECT 1 FROM community_members m WHERE m.community_id=c.id AND m.user_id=$2) AS is_member,
		       (c.owner_id = $2) AS is_owner
		FROM communities c
		WHERE c.id=$1 AND c.deleted_at IS NULL`, id, userID).Scan(
		&co.ID, &co.Name, &co.Description, &co.ShortDesc, &co.Category,
		&co.AvatarURL, &co.BannerURL, &co.Website, &co.Region, &co.Privacy,
		&co.Status, &co.Tags, &co.MembersCount, &co.PostsCount, &co.ActivityCount,
		&co.OwnerID, &co.CreatedAt, &co.IsMember, &co.IsOwner,
	)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "community not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"community": co})
}

func CreateCommunity(c *gin.Context) {
	userID := currentUserID(c)
	var body struct {
		Name        string   `json:"name" binding:"required,min=2,max=100"`
		Description string   `json:"description"`
		ShortDesc   string   `json:"short_description"`
		Category    string   `json:"category"`
		Privacy     string   `json:"privacy"`
		Website     string   `json:"website"`
		Region      string   `json:"region"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if body.Privacy == "" {
		body.Privacy = "open"
	}
	tags := pq.StringArray(body.Tags)

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var coID uuid.UUID
	err = tx.QueryRow(`
		INSERT INTO communities (name, description, short_description, category, privacy, website, region, tags, owner_id, members_count)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,1)
		RETURNING id`,
		body.Name, body.Description, body.ShortDesc, body.Category, body.Privacy,
		body.Website, body.Region, tags, userID,
	).Scan(&coID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = tx.Exec(`
		INSERT INTO community_members (community_id, user_id, role)
		VALUES ($1,$2,'owner')
		ON CONFLICT DO NOTHING`, coID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": coID})
}

func UpdateCommunity(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	if !isCommunityOwnerByUUID(id, userID) {
		jsonError(c, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		ShortDesc   string   `json:"short_description"`
		Category    string   `json:"category"`
		Privacy     string   `json:"privacy"`
		Website     string   `json:"website"`
		Region      string   `json:"region"`
		Tags        []string `json:"tags"`
		AvatarURL   string   `json:"avatar_url"`
		BannerURL   string   `json:"banner_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	tags := pq.StringArray(body.Tags)
	_, err := database.DB.Exec(`
		UPDATE communities SET
			name        = COALESCE(NULLIF($1,''), name),
			description = COALESCE(NULLIF($2,''), description),
			short_description = COALESCE(NULLIF($3,''), short_description),
			category    = COALESCE(NULLIF($4,''), category),
			privacy     = COALESCE(NULLIF($5,''), privacy),
			website     = NULLIF($6,''),
			region      = NULLIF($7,''),
			tags        = COALESCE(NULLIF($8::text[],'{}'), tags),
			avatar_url  = NULLIF($9,''),
			banner_url  = NULLIF($10,''),
			updated_at  = NOW()
		WHERE id=$11 AND deleted_at IS NULL`,
		body.Name, body.Description, body.ShortDesc, body.Category, body.Privacy,
		body.Website, body.Region, tags, body.AvatarURL, body.BannerURL, id,
	)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteCommunity(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	if !isCommunityOwnerByUUID(id, userID) {
		jsonError(c, http.StatusForbidden, "forbidden")
		return
	}
	_, err := database.DB.Exec(`UPDATE communities SET deleted_at=NOW() WHERE id=$1`, id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func JoinCommunity(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
		INSERT INTO community_members (community_id, user_id, role) VALUES ($1,$2,'member')
		ON CONFLICT DO NOTHING`, id, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		_, _ = tx.Exec(`UPDATE communities SET members_count=members_count+1, activity_count=activity_count+1 WHERE id=$1`, id)
	}
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func LeaveCommunity(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	var ownerID uuid.UUID
	_ = database.DB.QueryRow(`SELECT owner_id FROM communities WHERE id=$1`, id).Scan(&ownerID)
	if ownerID == userID {
		jsonError(c, http.StatusBadRequest, "owner cannot leave; delete community instead")
		return
	}
	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()
	result, _ := tx.Exec(`DELETE FROM community_members WHERE community_id=$1 AND user_id=$2`, id, userID)
	if affected, _ := result.RowsAffected(); affected > 0 {
		_, _ = tx.Exec(`UPDATE communities SET members_count=GREATEST(members_count-1,0) WHERE id=$1`, id)
	}
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UpdateCommunityStatus(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	if !isCommunityOwnerByUUID(id, userID) {
		jsonError(c, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	body.Status = strings.TrimSpace(body.Status)
	_, err := database.DB.Exec(`UPDATE communities SET status=NULLIF($1,''), updated_at=NOW() WHERE id=$2`, body.Status, id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ListCommunityPosts(c *gin.Context) {
	userID := currentUserID(c)
	id := c.Param("id")
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	rows, err := database.DB.Query(`
		SELECT p.id, p.community_id, p.author_id,
		       COALESCE(u.full_name, u.username, 'Пользователь') AS author_name,
		       u.avatar_url,
		       p.title, p.content, p.image_url,
		       COALESCE(p.tags, '{}'::text[]),
		       COALESCE(p.likes_count,0), COALESCE(p.comments_count,0),
		       EXISTS(SELECT 1 FROM post_likes l WHERE l.post_id=p.id AND l.user_id=$2) AS is_liked,
		       p.created_at
		FROM posts p
		INNER JOIN users u ON u.id = p.author_id
		WHERE p.community_id=$1 AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT $3 OFFSET $4`, id, userID, limit, offset)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	posts := make([]CommunityPost, 0)
	for rows.Next() {
		var p CommunityPost
		if err := rows.Scan(
			&p.ID, &p.CommunityID, &p.AuthorID, &p.AuthorName,
			&p.AuthorAvatar, &p.Title, &p.Content, &p.ImageURL, &p.Tags,
			&p.LikesCount, &p.CommentsCount, &p.IsLiked, &p.CreatedAt,
		); err == nil {
			posts = append(posts, p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func CreateCommunityPost(c *gin.Context) {
	userID := currentUserID(c)
	communityID := c.Param("id")
	var isMember bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id=$1 AND user_id=$2)`, communityID, userID).Scan(&isMember)
	if !isMember {
		jsonError(c, http.StatusForbidden, "join the community first")
		return
	}

	var body struct {
		Title   string   `json:"title"`
		Content string   `json:"content" binding:"required,min=1"`
		Tags    []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	tags := pq.StringArray(body.Tags)
	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var postID uuid.UUID
	err = tx.QueryRow(`
		INSERT INTO posts (author_id, author_type, community_id, title, content, short_description, tags, privacy_level)
		VALUES ($1,'community',$2,NULLIF($3,''),$4,$5,$6,'public')
		RETURNING id`,
		userID, communityID, body.Title, body.Content,
		truncate(body.Content, 180), tags,
	).Scan(&postID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	_, _ = tx.Exec(`UPDATE communities SET posts_count=posts_count+1, activity_count=activity_count+3 WHERE id=$1`, communityID)
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": postID})
}

func GetCommunityMembers(c *gin.Context) {
	id := c.Param("id")
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	rows, err := database.DB.Query(`
		SELECT u.id, COALESCE(u.full_name, u.username) AS full_name,
		       COALESCE(u.avatar_url,'') AS avatar_url,
		       COALESCE(m.role, 'member') AS role,
		       COALESCE(u.is_online, false) AS is_online,
		       m.joined_at
		FROM community_members m
		INNER JOIN users u ON u.id = m.user_id
		WHERE m.community_id=$1
		ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, m.joined_at ASC
		LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	members := make([]CommunityMember, 0)
	for rows.Next() {
		var m CommunityMember
		if err := rows.Scan(&m.UserID, &m.FullName, &m.AvatarURL, &m.Role, &m.IsOnline, &m.JoinedAt); err == nil {
			members = append(members, m)
		}
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

// Compatibility endpoints kept for existing clients.
func RequestJoinCommunity(c *gin.Context) {
	jsonError(c, http.StatusBadRequest, "private requests are not enabled")
}
func GetJoinRequests(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"requests": []interface{}{}}) }
func ApproveJoinRequest(c *gin.Context) {
	jsonError(c, http.StatusBadRequest, "join requests are not enabled")
}
func RejectJoinRequest(c *gin.Context) {
	jsonError(c, http.StatusBadRequest, "join requests are not enabled")
}
func SearchCommunities(c *gin.Context) { GetCommunities(c) }

func isCommunityOwnerByUUID(communityID string, userID uuid.UUID) bool {
	var ownerID uuid.UUID
	_ = database.DB.QueryRow(`SELECT owner_id FROM communities WHERE id=$1 AND deleted_at IS NULL`, communityID).Scan(&ownerID)
	return ownerID == userID
}

func queryInt(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
