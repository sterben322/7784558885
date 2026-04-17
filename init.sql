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
    is_private_profile BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_private_profile BOOLEAN NOT NULL DEFAULT false;
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_friends_pair_expr_unique
ON user_friends ((LEAST(user_id, friend_id)), (GREATEST(user_id, friend_id)));
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
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_url TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_name TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_size BIGINT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_type TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS image_url TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_messages_reply_to_id ON messages(reply_to_id);

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

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_moderator BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS forum_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    title VARCHAR(120) NOT NULL DEFAULT '',
    creator_id UUID REFERENCES users(id) ON DELETE SET NULL,
    description TEXT NOT NULL DEFAULT '',
    color_idx SMALLINT NOT NULL DEFAULT 0 CHECK (color_idx BETWEEN 0 AND 5),
    sort_order INT NOT NULL DEFAULT 0,
    topics_count INT NOT NULL DEFAULT 0,
    messages_count INT NOT NULL DEFAULT 0,
    last_author VARCHAR(200),
    last_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
ALTER TABLE forum_sections ADD COLUMN IF NOT EXISTS title VARCHAR(120);
ALTER TABLE forum_sections ADD COLUMN IF NOT EXISTS creator_id UUID REFERENCES users(id) ON DELETE SET NULL;
UPDATE forum_sections
SET name = COALESCE(NULLIF(name, ''), title, 'Без названия'),
    title = COALESCE(NULLIF(title, ''), name, 'Без названия');
UPDATE forum_sections
SET creator_id = (SELECT id FROM users ORDER BY created_at ASC LIMIT 1)
WHERE creator_id IS NULL
  AND EXISTS (SELECT 1 FROM users);
ALTER TABLE forum_sections
    ALTER COLUMN name SET NOT NULL,
    ALTER COLUMN title SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_forum_sections_sort ON forum_sections(sort_order, id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS forum_topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    section_id UUID NOT NULL REFERENCES forum_sections(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(300) NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    replies_count INT NOT NULL DEFAULT 0,
    views_count INT NOT NULL DEFAULT 0,
    is_hot BOOLEAN NOT NULL DEFAULT false,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_forum_topics_section ON forum_topics(section_id, is_pinned DESC, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_topics_author ON forum_topics(author_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS forum_discussions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(240) NOT NULL,
    messages_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_forum_discussions_topic ON forum_discussions(topic_id, created_at ASC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS forum_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    discussion_id UUID REFERENCES forum_discussions(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES forum_messages(id),
    author_id UUID NOT NULL REFERENCES users(id),
    text TEXT NOT NULL,
    likes_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS discussion_id UUID REFERENCES forum_discussions(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_forum_messages_topic ON forum_messages(topic_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_messages_discussion ON forum_messages(discussion_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_messages_parent ON forum_messages(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_forum_messages_author ON forum_messages(author_id) WHERE deleted_at IS NULL;

INSERT INTO forum_discussions (topic_id, author_id, title, created_at, updated_at)
SELECT t.id, t.author_id, 'Общее обсуждение', t.created_at, t.updated_at
FROM forum_topics t
WHERE NOT EXISTS (SELECT 1 FROM forum_discussions d WHERE d.topic_id = t.id);

UPDATE forum_messages m
SET discussion_id = sub.id
FROM (
    SELECT DISTINCT ON (d.topic_id) d.topic_id, d.id
    FROM forum_discussions d
    WHERE d.deleted_at IS NULL
    ORDER BY d.topic_id, d.created_at ASC
) sub
WHERE m.topic_id = sub.topic_id
  AND m.discussion_id IS NULL;

ALTER TABLE forum_messages ALTER COLUMN discussion_id SET NOT NULL;

UPDATE forum_discussions d
SET messages_count = sub.cnt,
    updated_at = NOW()
FROM (
    SELECT discussion_id, COUNT(*)::int AS cnt
    FROM forum_messages
    WHERE deleted_at IS NULL
    GROUP BY discussion_id
) sub
WHERE d.id = sub.discussion_id;

CREATE TABLE IF NOT EXISTS forum_message_likes (
    message_id UUID NOT NULL REFERENCES forum_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_forum_likes_user ON forum_message_likes(user_id);

CREATE TABLE IF NOT EXISTS projects (
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
);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_projects_author_type ON projects(author_type);
CREATE INDEX IF NOT EXISTS idx_projects_goal ON projects(goal);

CREATE TABLE IF NOT EXISTS project_needs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    amount_text VARCHAR(120),
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_project_needs_project_id ON project_needs(project_id, sort_order);

CREATE TABLE IF NOT EXISTS project_responses (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_project_responses_user_id ON project_responses(user_id);
