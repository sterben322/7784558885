package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	Email       string    `json:"email"`
	CompanyName *string   `json:"company_name,omitempty"`
	Phone       *string   `json:"phone,omitempty"`
	Position    *string   `json:"position,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RegisterRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Terms    bool   `json:"terms"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Friend struct {
	UserID       uuid.UUID `json:"user_id,omitempty"`
	FriendID     uuid.UUID `json:"friend_id,omitempty"`
	FriendName   string    `json:"friend_name"`
	FriendEmail  string    `json:"friend_email"`
	FriendAvatar *string   `json:"friend_avatar,omitempty"`
	Status       string    `json:"status,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Community struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	LogoURL      *string   `json:"logo_url,omitempty"`
	Icon         string    `json:"icon"`
	Color        string    `json:"color"`
	SearchTags   []string  `json:"search_tags"`
	IsPrivate    bool      `json:"is_private"`
	OwnerID      uuid.UUID `json:"owner_id"`
	OwnerName    string    `json:"owner_name"`
	MembersCount int       `json:"members_count"`
	PostsCount   int       `json:"posts_count"`
	Joined       bool      `json:"joined"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateCommunityRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description" binding:"required"`
	LogoURL     *string  `json:"logo_url"`
	Icon        string   `json:"icon"`
	Color       string   `json:"color"`
	SearchTags  []string `json:"search_tags"`
	IsPrivate   bool     `json:"is_private"`
}

type Company struct {
	ID             uuid.UUID `json:"id"`
	OwnerID        uuid.UUID `json:"owner_id"`
	OwnerName      string    `json:"owner_name,omitempty"`
	Name           string    `json:"name"`
	INN            *string   `json:"inn,omitempty"`
	Description    *string   `json:"description,omitempty"`
	LogoURL        *string   `json:"logo_url,omitempty"`
	EconomicSector *string   `json:"economic_sector,omitempty"`
	IsPublic       bool      `json:"is_public"`
	SearchTags     []string  `json:"search_tags"`
	Website        *string   `json:"website,omitempty"`
	Phone          *string   `json:"phone,omitempty"`
	Address        *string   `json:"address,omitempty"`
	FollowersCount int       `json:"followers_count"`
	EmployeeCount  int       `json:"employee_count"`
	IsFollowing    bool      `json:"is_following"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateCompanyRequest struct {
	Name           string   `json:"name" binding:"required"`
	INN            *string  `json:"inn"`
	Description    *string  `json:"description"`
	LogoURL        *string  `json:"logo_url"`
	EconomicSector *string  `json:"economic_sector"`
	IsPublic       bool     `json:"is_public"`
	SearchTags     []string `json:"search_tags"`
	Website        *string  `json:"website"`
	Phone          *string  `json:"phone"`
	Address        *string  `json:"address"`
}

type Post struct {
	ID               uuid.UUID  `json:"id"`
	AuthorID         uuid.UUID  `json:"author_id"`
	AuthorType       string     `json:"author_type"`
	AuthorName       string     `json:"author_name"`
	AuthorAvatar     *string    `json:"author_avatar,omitempty"`
	Title            *string    `json:"title,omitempty"`
	Content          string     `json:"content"`
	ShortDescription *string    `json:"short_description,omitempty"`
	ImageURL         *string    `json:"image_url,omitempty"`
	Tags             []string   `json:"tags"`
	PrivacyLevel     string     `json:"privacy_level"`
	TargetID         *uuid.UUID `json:"target_id,omitempty"`
	IsHidden         bool       `json:"is_hidden"`
	IsUnpublished    bool       `json:"is_unpublished"`
	LikesCount       int        `json:"likes_count"`
	CommentsCount    int        `json:"comments_count"`
	SharesCount      int        `json:"shares_count"`
	IsLiked          bool       `json:"is_liked"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CreatePostRequest struct {
	Title            *string  `json:"title"`
	Content          string   `json:"content" binding:"required"`
	ShortDescription *string  `json:"short_description"`
	ImageURL         *string  `json:"image_url"`
	Tags             []string `json:"tags"`
	PrivacyLevel     string   `json:"privacy_level"`
	TargetID         *string  `json:"target_id"`
	IsHidden         bool     `json:"is_hidden"`
	IsUnpublished    bool     `json:"is_unpublished"`
}

type Comment struct {
	ID         uuid.UUID `json:"id"`
	PostID     uuid.UUID `json:"post_id"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Content    string    `json:"content"`
	LikesCount int       `json:"likes_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type ForumTopic struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Category     string    `json:"category"`
	AuthorID     uuid.UUID `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	RepliesCount int       `json:"replies_count"`
	ViewsCount   int       `json:"views_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
}

type ForumReply struct {
	ID         uuid.UUID `json:"id"`
	TopicID    uuid.UUID `json:"topic_id"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type Chat struct {
	ID          uuid.UUID  `json:"id"`
	Name        *string    `json:"name,omitempty"`
	Type        string     `json:"type"`
	LastMessage *string    `json:"last_message,omitempty"`
	LastTime    *time.Time `json:"last_time,omitempty"`
	UnreadCount int        `json:"unread_count"`
}

type Message struct {
	ID         uuid.UUID `json:"id"`
	ChatID     uuid.UUID `json:"chat_id"`
	SenderID   uuid.UUID `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

type DashboardStats struct {
	ProjectsCount     int `json:"projects_count"`
	CommunitiesJoined int `json:"communities_joined"`
	TopicsCount       int `json:"topics_count"`
	UnreadMessages    int `json:"unread_messages"`
}

type CommunityRole struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

type CompanyRole struct {
	ID               uuid.UUID `json:"id"`
	PositionName     string    `json:"position_name"`
	Responsibilities []string  `json:"responsibilities,omitempty"`
	Permissions      []string  `json:"permissions,omitempty"`
}

type CompanyEmployee struct {
	UserID       uuid.UUID `json:"user_id"`
	UserName     string    `json:"user_name"`
	UserEmail    string    `json:"user_email"`
	UserAvatar   *string   `json:"user_avatar,omitempty"`
	PositionName string    `json:"position_name"`
	Department   *string   `json:"department,omitempty"`
	HireDate     *string   `json:"hire_date,omitempty"`
	IsActive     bool      `json:"is_active"`
	AssignedAt   time.Time `json:"assigned_at"`
}

type Resume struct {
	ID                 uuid.UUID `json:"id"`
	UserID             uuid.UUID `json:"user_id"`
	Title              string    `json:"title"`
	About              *string   `json:"about,omitempty"`
	ActivityType       *string   `json:"activity_type,omitempty"`
	Skills             []string  `json:"skills"`
	EducationLevels    []string  `json:"education_levels"`
	PreviousWorkplaces []string  `json:"previous_workplaces"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateResumeRequest struct {
	Title              string   `json:"title" binding:"required"`
	About              *string  `json:"about"`
	ActivityType       *string  `json:"activity_type"`
	Skills             []string `json:"skills"`
	EducationLevels    []string `json:"education_levels"`
	PreviousWorkplaces []string `json:"previous_workplaces"`
}

type Vacancy struct {
	ID                 uuid.UUID `json:"id"`
	PublisherType      string    `json:"publisher_type"`
	PublisherID        uuid.UUID `json:"publisher_id"`
	PublisherName      string    `json:"publisher_name"`
	Position           string    `json:"position"`
	Salary             string    `json:"salary"`
	Expectations       string    `json:"expectations"`
	RequiredSkills     []string  `json:"required_skills"`
	RequiredExperience *string   `json:"required_experience,omitempty"`
	EmploymentType     *string   `json:"employment_type,omitempty"`
	Location           *string   `json:"location,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateVacancyRequest struct {
	PublisherType      string   `json:"publisher_type" binding:"required"`
	PublisherID        string   `json:"publisher_id" binding:"required"`
	Position           string   `json:"position" binding:"required"`
	Salary             string   `json:"salary" binding:"required"`
	Expectations       string   `json:"expectations" binding:"required"`
	RequiredSkills     []string `json:"required_skills"`
	RequiredExperience *string  `json:"required_experience"`
	EmploymentType     *string  `json:"employment_type"`
	Location           *string  `json:"location"`
}
type AssignRoleRequest struct {
	UserID   uuid.UUID `json:"user_id" binding:"required"`
	RoleName string    `json:"role_name" binding:"required"`
}

type InviteToCommunityRequest struct {
	UserID   uuid.UUID `json:"user_id" binding:"required"`
	RoleName string    `json:"role_name"`
}

type InviteToCompanyRequest struct {
	UserID       uuid.UUID `json:"user_id" binding:"required"`
	PositionName string    `json:"position_name" binding:"required"`
	Department   *string   `json:"department"`
}
