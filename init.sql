CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Базовые таблицы, необходимые для регистрации и авторизации,
-- чтобы фронтенд мог работать сразу после `docker compose up`.
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    full_name VARCHAR(100) NOT NULL,
    name VARCHAR(100),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    company_name VARCHAR(200),
    phone VARCHAR(20),
    position VARCHAR(100),
    avatar_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users(email);

CREATE TABLE IF NOT EXISTS user_friends (
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
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_friends_pair_unique ON user_friends (user_low, user_high);
CREATE INDEX IF NOT EXISTS idx_user_friends_status ON user_friends(status);
CREATE INDEX IF NOT EXISTS idx_user_friends_requester_status ON user_friends(requester_id, status);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100),
    type VARCHAR(20) NOT NULL DEFAULT 'dialog',
    direct_user_low UUID REFERENCES users(id) ON DELETE SET NULL,
    direct_user_high UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_message_at TIMESTAMP,
    CHECK (
        type <> 'direct'
        OR (
            direct_user_low IS NOT NULL
            AND direct_user_high IS NOT NULL
            AND direct_user_low <> direct_user_high
        )
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_direct_pair_unique
ON chats (direct_user_low, direct_user_high)
WHERE type = 'direct';

CREATE TABLE IF NOT EXISTS chat_participants (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    unread_count INT NOT NULL DEFAULT 0,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_read_at TIMESTAMP,
    PRIMARY KEY (chat_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_chat_participants_user ON chat_participants(user_id);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_messages_chat_created_at ON messages(chat_id, created_at DESC);

CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    is_public BOOLEAN NOT NULL DEFAULT true,
    followers_count INT NOT NULL DEFAULT 0,
    employee_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS company_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    role_code VARCHAR(50),
    position_name VARCHAR(100) NOT NULL,
    permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    UNIQUE (company_id, position_name),
    UNIQUE (company_id, role_code)
);

CREATE TABLE IF NOT EXISTS company_employees (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position_name VARCHAR(100) NOT NULL,
    role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (company_id, user_id)
);

CREATE TABLE IF NOT EXISTS company_user_roles (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES company_roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (company_id, user_id)
);

CREATE TABLE IF NOT EXISTS company_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    inviter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position_name VARCHAR(100) NOT NULL,
    role_id UUID REFERENCES company_roles(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP NOT NULL
);

-- Тестовый пользователь для быстрого входа в систему после первого запуска PostgreSQL.
-- Пароль: TestPass123!
INSERT INTO users (
    id,
    first_name,
    last_name,
    full_name,
    email,
    password_hash,
    company_name,
    phone,
    position,
    avatar_url
)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Тестовый',
    'Пользователь',
    'Тестовый Пользователь',
    'test.user@lastop.local',
    crypt('TestPass123!', gen_salt('bf')),
    'Lastop QA',
    '+1-202-555-0188',
    'QA Engineer',
    'https://i.pravatar.cc/300?img=12'
)
ON CONFLICT (email) DO NOTHING;

CREATE TABLE IF NOT EXISTS forum_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(180) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topics_count INT NOT NULL DEFAULT 0,
    posts_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS forum_topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    section_id UUID NOT NULL REFERENCES forum_sections(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    posts_count INT NOT NULL DEFAULT 0,
    views_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_forum_topics_section_updated ON forum_topics(section_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS forum_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_forum_posts_topic_created ON forum_posts(topic_id, created_at ASC);
