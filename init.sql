CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Тестовый пользователь для быстрого входа в систему после первого запуска PostgreSQL.
-- Важно: password_hash для текущей реализации хранится в формате "hashed:<пароль>".
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
    'hashed:TestPass123!',
    'Lastop QA',
    '+1-202-555-0188',
    'QA Engineer',
    'https://i.pravatar.cc/300?img=12'
)
ON CONFLICT (email) DO NOTHING;
