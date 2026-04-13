CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Базовые таблицы, необходимые для регистрации и авторизации,
-- чтобы фронтенд мог работать сразу после `docker compose up`.
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

-- Тестовый пользователь для быстрого входа в систему после первого запуска PostgreSQL.
-- Пароль: TestPass123!
INSERT INTO users (
    id,
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
    'Тестовый Пользователь',
    'test.user@lastop.local',
    crypt('TestPass123!', gen_salt('bf')),
    'Lastop QA',
    '+1-202-555-0188',
    'QA Engineer',
    'https://i.pravatar.cc/300?img=12'
)
ON CONFLICT (email) DO NOTHING;
