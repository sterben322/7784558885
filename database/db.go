package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB(databaseURL string) error {
	if os.Getenv("TEST_MODE") == "1" {
		log.Println("TEST_MODE=1: database initialization skipped")
		return nil
	}

	host := getenv("DB_HOST", getenv("PGHOST", ""))
	port := getenv("DB_PORT", getenv("PGPORT", "5432"))
	user := getenv("DB_USER", getenv("PGUSER", "postgres"))
	password := getenv("DB_PASSWORD", getenv("PGPASSWORD", "postgres"))
	dbname := getenv("DB_NAME", getenv("PGDATABASE", "lastop_db"))

	if databaseURL == "" && host == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	connStrings := make([]string, 0, 1)
	if databaseURL != "" {
		connStrings = append(connStrings, databaseURL)
	} else {
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
		connStrings = append(connStrings, connStr)
	}

	var lastErr error
	for _, connStr := range connStrings {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			continue
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err != nil {
			lastErr = err
			_ = db.Close()
			continue
		}

		DB = db
		log.Println("database connected")
		return nil
	}

	return fmt.Errorf("database is unavailable: %w", lastErr)
}

func IsConfigured() bool {
	return os.Getenv("DATABASE_URL") != "" ||
		os.Getenv("DB_HOST") != "" ||
		os.Getenv("PGHOST") != ""
}

func Ping(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	return DB.PingContext(ctx)
}

func CloseDB() {
	if DB != nil {
		_ = DB.Close()
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func CreateTables() error {
	if DB == nil {
		log.Println("database not initialized: skipping table creation")
		return fmt.Errorf("database is not initialized")
	}

	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            first_name VARCHAR(100),
            last_name VARCHAR(100),
            full_name VARCHAR(100) NOT NULL,
            email VARCHAR(255) UNIQUE NOT NULL,
            password_hash VARCHAR(255) NOT NULL,
            name VARCHAR(100),
            company_name VARCHAR(200),
            phone VARCHAR(20),
            position VARCHAR(100),
            avatar_url TEXT,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR(100)`,
		`UPDATE users
		 SET first_name = COALESCE(NULLIF(first_name, ''), split_part(full_name, ' ', 1)),
		     last_name = COALESCE(NULLIF(last_name, ''), NULLIF(trim(substr(full_name, length(split_part(full_name, ' ', 1)) + 1)), ''))
		 WHERE first_name IS NULL OR first_name = '' OR last_name IS NULL OR last_name = ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users (email)`,
		`CREATE TABLE IF NOT EXISTS user_friends (
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            friend_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            requester_id UUID REFERENCES users(id) ON DELETE SET NULL,
            user_low UUID GENERATED ALWAYS AS (LEAST(user_id, friend_id)) STORED,
            user_high UUID GENERATED ALWAYS AS (GREATEST(user_id, friend_id)) STORED,
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (user_id, friend_id),
            CHECK (user_id <> friend_id),
            CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled'))
        )`,
		`ALTER TABLE user_friends ADD COLUMN IF NOT EXISTS requester_id UUID REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE user_friends ADD COLUMN IF NOT EXISTS user_low UUID GENERATED ALWAYS AS (LEAST(user_id, friend_id)) STORED`,
		`ALTER TABLE user_friends ADD COLUMN IF NOT EXISTS user_high UUID GENERATED ALWAYS AS (GREATEST(user_id, friend_id)) STORED`,
		`UPDATE user_friends SET requester_id = COALESCE(requester_id, user_id)`,
		`ALTER TABLE user_friends DROP CONSTRAINT IF EXISTS user_friends_status_check`,
		`ALTER TABLE user_friends ADD CONSTRAINT user_friends_status_check CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled'))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_friends_pair_unique ON user_friends ((LEAST(user_id, friend_id)), (GREATEST(user_id, friend_id)))`,
		`CREATE INDEX IF NOT EXISTS idx_user_friends_status ON user_friends (status)`,
		`CREATE INDEX IF NOT EXISTS idx_user_friends_requester_status ON user_friends (requester_id, status)`,
		`CREATE TABLE IF NOT EXISTS communities (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(100) NOT NULL UNIQUE,
            description TEXT NOT NULL,
            logo_url TEXT,
            icon VARCHAR(50) NOT NULL DEFAULT 'fa-users',
            color VARCHAR(50) NOT NULL DEFAULT 'blue',
            search_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            is_private BOOLEAN NOT NULL DEFAULT false,
            owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            members_count INT NOT NULL DEFAULT 0,
            posts_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS community_members (
            community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (community_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS community_roles (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
            name VARCHAR(50) NOT NULL,
            display_name VARCHAR(100) NOT NULL,
            permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE (community_id, name)
        )`,
		`CREATE TABLE IF NOT EXISTS community_user_roles (
            community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            role_name VARCHAR(50) NOT NULL,
            assigned_by UUID REFERENCES users(id),
            assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (community_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS community_invites (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
            inviter_id UUID REFERENCES users(id) ON DELETE SET NULL,
            invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            role_name VARCHAR(50) NOT NULL DEFAULT 'member',
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            expires_at TIMESTAMP NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS community_join_requests (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            message TEXT,
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE (community_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS companies (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            owner_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
            name VARCHAR(200) NOT NULL,
            inn VARCHAR(20),
            description TEXT,
            logo_url TEXT,
            economic_sector VARCHAR(100),
            is_public BOOLEAN NOT NULL DEFAULT true,
            search_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            website VARCHAR(255),
            phone VARCHAR(20),
            address TEXT,
            followers_count INT NOT NULL DEFAULT 0,
            employee_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS company_followers (
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (company_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS company_roles (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            position_name VARCHAR(100) NOT NULL,
            responsibilities TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS company_employees (
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            position_name VARCHAR(100) NOT NULL,
            department VARCHAR(100),
            hire_date DATE,
            is_active BOOLEAN NOT NULL DEFAULT true,
            assigned_by UUID REFERENCES users(id),
            assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (company_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS company_invites (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            inviter_id UUID REFERENCES users(id) ON DELETE SET NULL,
            invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            position_name VARCHAR(100) NOT NULL,
            department VARCHAR(100),
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            expires_at TIMESTAMP NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS corporate_profiles (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
            company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
            created_by UUID REFERENCES users(id) ON DELETE SET NULL,
            position_name VARCHAR(100),
            permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            employment_status VARCHAR(20) NOT NULL DEFAULT 'independent',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`ALTER TABLE company_invites ADD COLUMN IF NOT EXISTS corporate_profile_id UUID REFERENCES corporate_profiles(id) ON DELETE SET NULL`,
		`CREATE TABLE IF NOT EXISTS posts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            author_type VARCHAR(20) NOT NULL,
            author_name VARCHAR(255) NOT NULL,
            author_avatar TEXT,
            title VARCHAR(255),
            content TEXT NOT NULL,
            short_description TEXT,
            image_url TEXT,
            tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            privacy_level VARCHAR(20) NOT NULL DEFAULT 'public',
            target_id UUID,
            likes_count INT NOT NULL DEFAULT 0,
            comments_count INT NOT NULL DEFAULT 0,
            shares_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,

		`ALTER TABLE posts ADD COLUMN IF NOT EXISTS is_hidden BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE posts ADD COLUMN IF NOT EXISTS is_unpublished BOOLEAN NOT NULL DEFAULT false`,
		`CREATE TABLE IF NOT EXISTS company_members (
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (company_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS company_join_requests (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            message TEXT,
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE (company_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS resumes (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
            title VARCHAR(200) NOT NULL,
            about TEXT,
            activity_type VARCHAR(100),
            skills TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            education_levels TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            previous_workplaces TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS vacancies (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            publisher_type VARCHAR(20) NOT NULL,
            publisher_id UUID NOT NULL,
            publisher_name VARCHAR(255) NOT NULL,
            position VARCHAR(200) NOT NULL,
            salary VARCHAR(100) NOT NULL,
            expectations TEXT NOT NULL,
            required_skills TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            required_experience VARCHAR(100),
            employment_type VARCHAR(100),
            location VARCHAR(255),
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS post_likes (
            post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (post_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS post_comments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            author_name VARCHAR(255) NOT NULL,
            content TEXT NOT NULL,
            likes_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS forum_topics (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title VARCHAR(200) NOT NULL,
            content TEXT NOT NULL,
            category VARCHAR(50) NOT NULL DEFAULT '',
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            replies_count INT NOT NULL DEFAULT 0,
            views_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS forum_replies (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS chats (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(100),
            type VARCHAR(20) NOT NULL DEFAULT 'dialog',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS chat_participants (
            chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            unread_count INT NOT NULL DEFAULT 0,
            PRIMARY KEY (chat_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS messages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            read BOOLEAN NOT NULL DEFAULT false,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS sessions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token VARCHAR(500) NOT NULL UNIQUE,
            expires_at TIMESTAMP NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return fmt.Errorf("error creating tables: %w", err)
		}
	}

	return nil
}

func EnsureDefaultCommunityRoles(communityID string) error {
	if DB == nil {
		return nil
	}

	statements := []string{
		`INSERT INTO community_roles (community_id, name, display_name, permissions)
         VALUES ($1, 'admin', 'Администратор', ARRAY['manage_members','manage_posts','manage_settings','delete_posts','pin_posts','manage_roles'])
         ON CONFLICT (community_id, name) DO NOTHING`,
		`INSERT INTO community_roles (community_id, name, display_name, permissions)
         VALUES ($1, 'moderator', 'Модератор', ARRAY['manage_posts','delete_posts','pin_posts'])
         ON CONFLICT (community_id, name) DO NOTHING`,
		`INSERT INTO community_roles (community_id, name, display_name, permissions)
         VALUES ($1, 'editor', 'Редактор', ARRAY['manage_posts','pin_posts'])
         ON CONFLICT (community_id, name) DO NOTHING`,
		`INSERT INTO community_roles (community_id, name, display_name, permissions)
         VALUES ($1, 'member', 'Участник', ARRAY[]::TEXT[])
         ON CONFLICT (community_id, name) DO NOTHING`,
	}
	for _, stmt := range statements {
		if _, err := DB.Exec(stmt, communityID); err != nil {
			return err
		}
	}
	return nil
}
