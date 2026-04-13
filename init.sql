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

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

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
