package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB
var stateMu sync.RWMutex
var dbReady bool
var lastConnectErr string
var schemaInitialized bool

func InitDB(databaseURL string) error {
	if os.Getenv("TEST_MODE") == "1" {
		log.Println("TEST_MODE=1: database initialization skipped")
		setReadyState(true, "")
		return nil
	}

	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}

	if databaseURL == "" {
		err := fmt.Errorf("DATABASE_URL is not set")
		setReadyState(false, err.Error())
		return err
	}

	connStrings := []string{withDefaultSSLMode(databaseURL)}

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
		setSchemaInitialized(false)
		setReadyState(false, "connected, schema is not initialized")
		log.Println("database connected")
		return nil
	}

	err := fmt.Errorf("database is unavailable: %w", lastErr)
	setReadyState(false, err.Error())
	return err
}

func Startup(databaseURL string) {
	if os.Getenv("TEST_MODE") == "1" {
		return
	}
	go retryConnectLoop(databaseURL)
}

func retryConnectLoop(databaseURL string) {
	initialDelay := envDuration("DB_RETRY_INITIAL_DELAY", time.Second)
	interval := envDuration("DB_RETRY_INTERVAL", 3*time.Second)
	pingTimeout := envDuration("DB_PING_TIMEOUT", 5*time.Second)

	if initialDelay > 0 {
		time.Sleep(initialDelay)
	}

	for {
		if IsReady() {
			time.Sleep(interval)
			continue
		}

		if err := InitDB(databaseURL); err != nil {
			log.Printf("database connection attempt failed: %v", err)
			time.Sleep(interval)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		err := Ping(ctx)
		cancel()
		if err != nil {
			setReadyState(false, fmt.Sprintf("database ping failed after connect: %v", err))
			log.Printf("database ping failed after connect: %v", err)
			CloseDB()
			time.Sleep(interval)
			continue
		}

		if err := CreateTables(); err != nil {
			setReadyState(false, fmt.Sprintf("database schema init failed: %v", err))
			log.Printf("database schema init failed: %v", err)
			CloseDB()
			time.Sleep(interval)
			continue
		}

		setReadyState(true, "")
		log.Println("database is ready")
	}
}

func withDefaultSSLMode(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	if query.Get("sslmode") != "" {
		return rawURL
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "", "localhost", "127.0.0.1":
		query.Set("sslmode", "disable")
	default:
		query.Set("sslmode", "require")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func IsConfigured() bool {
	return os.Getenv("DATABASE_URL") != ""
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
		DB = nil
		setSchemaInitialized(false)
		setReadyState(false, "database connection is closed")
	}
}

func CreateTables() error {
	if DB == nil {
		log.Println("database not initialized: skipping table creation")
		setReadyState(false, "database is not initialized")
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
            is_private_profile BOOLEAN NOT NULL DEFAULT false,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR(100)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_private_profile BOOLEAN NOT NULL DEFAULT false`,
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
            role_code VARCHAR(50),
            position_name VARCHAR(100) NOT NULL,
            responsibilities TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE (company_id, position_name),
            UNIQUE (company_id, role_code)
        )`,
		`CREATE TABLE IF NOT EXISTS company_employees (
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            position_name VARCHAR(100) NOT NULL,
            role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL,
            department VARCHAR(100),
            hire_date DATE,
            is_active BOOLEAN NOT NULL DEFAULT true,
            assigned_by UUID REFERENCES users(id),
            assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (company_id, user_id)
        )`,
		`CREATE TABLE IF NOT EXISTS company_user_roles (
            company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            role_id UUID NOT NULL REFERENCES company_roles(id) ON DELETE CASCADE,
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
            role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL,
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
		`ALTER TABLE company_invites ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL`,
		`ALTER TABLE company_roles ADD COLUMN IF NOT EXISTS role_code VARCHAR(50)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_company_roles_position ON company_roles(company_id, position_name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_company_roles_role_code ON company_roles(company_id, role_code) WHERE role_code IS NOT NULL`,
		`ALTER TABLE company_employees ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL`,
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
		`CREATE TABLE IF NOT EXISTS projects (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			author_type VARCHAR(20) NOT NULL CHECK (author_type IN ('user', 'company', 'community')),
			author_id UUID,
			author_name VARCHAR(255) NOT NULL,
			title VARCHAR(300) NOT NULL,
			category VARCHAR(120),
			goal VARCHAR(20) NOT NULL DEFAULT 'partner',
			city VARCHAR(120),
			tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			goals TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			description TEXT,
			images TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			progress INT,
			views_count INT NOT NULL DEFAULT 0,
			responses_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_author_type ON projects(author_type)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_goal ON projects(goal)`,
		`CREATE TABLE IF NOT EXISTS project_needs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name VARCHAR(200) NOT NULL,
			amount_text VARCHAR(120),
			status VARCHAR(20) NOT NULL DEFAULT 'open',
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_needs_project_id ON project_needs(project_id, sort_order)`,
		`CREATE TABLE IF NOT EXISTS project_responses (
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_responses_user_id ON project_responses(user_id)`,
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
		`CREATE TABLE IF NOT EXISTS forum_sections (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title VARCHAR(180) NOT NULL UNIQUE,
            description TEXT NOT NULL DEFAULT '',
            creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            topics_count INT NOT NULL DEFAULT 0,
            posts_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS forum_topics (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            section_id UUID REFERENCES forum_sections(id) ON DELETE CASCADE,
            title VARCHAR(200) NOT NULL,
            content TEXT,
            category VARCHAR(50) NOT NULL DEFAULT '',
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            replies_count INT NOT NULL DEFAULT 0,
            posts_count INT NOT NULL DEFAULT 0,
            views_count INT NOT NULL DEFAULT 0,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS forum_posts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
            author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
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
		`CREATE INDEX IF NOT EXISTS idx_forum_topics_section_updated ON forum_topics(section_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_forum_posts_topic_created ON forum_posts(topic_id, created_at ASC)`,
		`CREATE TABLE IF NOT EXISTS chats (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(100),
            type VARCHAR(20) NOT NULL DEFAULT 'dialog',
            direct_user_low UUID REFERENCES users(id) ON DELETE SET NULL,
            direct_user_high UUID REFERENCES users(id) ON DELETE SET NULL,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            last_message_at TIMESTAMP
        )`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS direct_user_low UUID REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS direct_user_high UUID REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMP`,
		`ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_direct_pair_check`,
		`ALTER TABLE chats ADD CONSTRAINT chats_direct_pair_check CHECK (
            type <> 'direct'
            OR (
                direct_user_low IS NOT NULL
                AND direct_user_high IS NOT NULL
                AND direct_user_low <> direct_user_high
            )
        )`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_direct_pair_unique
            ON chats(direct_user_low, direct_user_high)
            WHERE type = 'direct'`,
		`CREATE TABLE IF NOT EXISTS chat_participants (
            chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            unread_count INT NOT NULL DEFAULT 0,
            joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            last_read_at TIMESTAMP,
            PRIMARY KEY (chat_id, user_id)
        )`,
		`ALTER TABLE chat_participants ADD COLUMN IF NOT EXISTS joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE chat_participants ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMP`,
		`CREATE INDEX IF NOT EXISTS idx_chat_participants_user ON chat_participants(user_id)`,
		`CREATE TABLE IF NOT EXISTS messages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            read BOOLEAN NOT NULL DEFAULT false,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_url TEXT`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_name TEXT`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_size BIGINT`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_type TEXT`,
		`ALTER TABLE messages ADD COLUMN IF NOT EXISTS image_url TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat_created_at ON messages(chat_id, created_at DESC)`,
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
			setReadyState(false, fmt.Sprintf("error creating tables: %v", err))
			return fmt.Errorf("error creating tables: %w", err)
		}
	}

	_, _ = DB.Exec(`
		UPDATE company_employees ce
		SET role_id = cr.id
		FROM company_roles cr
		WHERE ce.company_id = cr.company_id
		  AND ce.position_name = cr.position_name
		  AND ce.role_id IS NULL
	`)

	_, _ = DB.Exec(`
		INSERT INTO forum_sections (title, description, creator_id)
		SELECT 'Общий раздел', 'Раздел по умолчанию', id
		FROM users
		ORDER BY created_at ASC
		LIMIT 1
		ON CONFLICT (title) DO NOTHING
	`)

	_, _ = DB.Exec(`
		UPDATE forum_topics
		SET section_id = (SELECT id FROM forum_sections ORDER BY created_at ASC LIMIT 1)
		WHERE section_id IS NULL
	`)

	_, _ = DB.Exec(`
		INSERT INTO forum_posts (id, topic_id, author_id, content, created_at, updated_at)
		SELECT gen_random_uuid(), t.id, t.author_id, t.content, t.created_at, COALESCE(t.updated_at, t.created_at)
		FROM forum_topics t
		WHERE t.content IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM forum_posts p WHERE p.topic_id = t.id)
	`)

	_, _ = DB.Exec(`
		INSERT INTO forum_posts (id, topic_id, author_id, content, created_at, updated_at)
		SELECT fr.id, fr.topic_id, fr.author_id, fr.content, fr.created_at, fr.created_at
		FROM forum_replies fr
		WHERE NOT EXISTS (SELECT 1 FROM forum_posts fp WHERE fp.id = fr.id)
	`)

	_, _ = DB.Exec(`
		UPDATE forum_topics t
		SET posts_count = sub.cnt,
		    updated_at = COALESCE(sub.last_at, t.updated_at)
		FROM (
			SELECT topic_id, COUNT(*)::int AS cnt, MAX(created_at) AS last_at
			FROM forum_posts
			GROUP BY topic_id
		) sub
		WHERE t.id = sub.topic_id
	`)

	_, _ = DB.Exec(`
		UPDATE forum_sections s
		SET topics_count = sub.topics_count,
		    posts_count = sub.posts_count,
		    updated_at = COALESCE(sub.last_at, s.updated_at)
		FROM (
			SELECT t.section_id,
			       COUNT(DISTINCT t.id)::int AS topics_count,
			       COUNT(p.id)::int AS posts_count,
			       MAX(COALESCE(p.created_at, t.updated_at)) AS last_at
			FROM forum_topics t
			LEFT JOIN forum_posts p ON p.topic_id = t.id
			GROUP BY t.section_id
		) sub
		WHERE s.id = sub.section_id
	`)

	_, _ = DB.Exec(`ALTER TABLE forum_topics ALTER COLUMN section_id SET NOT NULL`)
	_, _ = DB.Exec(`
		CREATE TABLE IF NOT EXISTS forum_discussions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
			author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(240) NOT NULL,
			messages_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		)
	`)
	_, _ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_forum_discussions_topic ON forum_discussions(topic_id, created_at ASC)`)
	_, _ = DB.Exec(`ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS discussion_id UUID REFERENCES forum_discussions(id) ON DELETE CASCADE`)
	_, _ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_forum_messages_discussion ON forum_messages(discussion_id, created_at)`)
	_, _ = DB.Exec(`
		INSERT INTO forum_discussions (topic_id, author_id, title, created_at, updated_at)
		SELECT t.id, t.author_id, 'Общее обсуждение', t.created_at, t.updated_at
		FROM forum_topics t
		WHERE NOT EXISTS (SELECT 1 FROM forum_discussions d WHERE d.topic_id = t.id)
	`)
	_, _ = DB.Exec(`
		UPDATE forum_messages m
		SET discussion_id = sub.id
		FROM (
			SELECT DISTINCT ON (d.topic_id) d.topic_id, d.id
			FROM forum_discussions d
			WHERE d.deleted_at IS NULL
			ORDER BY d.topic_id, d.created_at ASC
		) sub
		WHERE m.topic_id = sub.topic_id
		  AND m.discussion_id IS NULL
	`)
	_, _ = DB.Exec(`ALTER TABLE forum_messages ALTER COLUMN discussion_id SET NOT NULL`)
	_, _ = DB.Exec(`
		UPDATE forum_discussions d
		SET messages_count = sub.cnt,
		    updated_at = CURRENT_TIMESTAMP
		FROM (
			SELECT discussion_id, COUNT(*)::int AS cnt
			FROM forum_messages
			WHERE deleted_at IS NULL
			GROUP BY discussion_id
		) sub
		WHERE d.id = sub.discussion_id
	`)

	companyRows, err := DB.Query(`SELECT id, owner_id FROM companies`)
	if err != nil {
		return err
	}
	defer companyRows.Close()

	for companyRows.Next() {
		var companyID, ownerID string
		if err := companyRows.Scan(&companyID, &ownerID); err != nil {
			return err
		}
		if err := EnsureDefaultCompanyRoles(companyID); err != nil {
			return err
		}
		_, _ = DB.Exec(`
			INSERT INTO company_user_roles (company_id, user_id, role_id, assigned_by)
			SELECT $1, $2, id, $2 FROM company_roles
			WHERE company_id = $1 AND role_code = 'owner'
			ON CONFLICT (company_id, user_id) DO NOTHING
		`, companyID, ownerID)
	}

	setSchemaInitialized(true)
	setReadyState(true, "")
	return nil
}

func setReadyState(ready bool, errMsg string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	dbReady = ready
	lastConnectErr = strings.TrimSpace(errMsg)
}

func IsReady() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return DB != nil && dbReady && schemaInitialized
}

func LastError() string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return lastConnectErr
}

func setSchemaInitialized(value bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	schemaInitialized = value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
		return parsed
	}

	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	return fallback
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

func EnsureDefaultCompanyRoles(companyID string) error {
	if DB == nil {
		return nil
	}

	statements := []string{
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'owner', 'Owner', ARRAY['Полный доступ'], ARRAY['*'])
		 ON CONFLICT (company_id, role_code) DO NOTHING`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'admin', 'Admin', ARRAY['Операционное управление'], ARRAY['invite_employees','manage_roles','edit_company_profile','publish_news','manage_employees'])
		 ON CONFLICT (company_id, role_code) DO NOTHING`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'editor', 'Editor', ARRAY['Редактирование профиля и новостей'], ARRAY['edit_company_profile','publish_news'])
		 ON CONFLICT (company_id, role_code) DO NOTHING`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'member', 'Member', ARRAY['Базовый доступ'], ARRAY[]::TEXT[])
		 ON CONFLICT (company_id, role_code) DO NOTHING`,
	}
	for _, stmt := range statements {
		if _, err := DB.Exec(stmt, companyID); err != nil {
			return err
		}
	}
	return nil
}
