-- ══════════════════════════════════════════════════════════════════
--  EVENTS — мероприятия платформы LASTOP
--  Добавить в конец init.sql (или выполнить отдельно при миграции)
-- ══════════════════════════════════════════════════════════════════

-- Мероприятия
CREATE TABLE IF NOT EXISTS events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL,
    type             TEXT NOT NULL DEFAULT 'webinar',   -- webinar|workshop|conference|networking|roundtable
    format           TEXT NOT NULL DEFAULT 'online',    -- online|offline|hybrid
    category         TEXT NOT NULL DEFAULT '',
    city             TEXT,
    date             DATE NOT NULL,
    time_start       TIME,
    time_end         TIME,
    duration_min     INT  DEFAULT 0,
    fee              INT  NOT NULL DEFAULT 0,
    seats_total      INT  NOT NULL DEFAULT 0,
    description      TEXT NOT NULL DEFAULT '',
    cover            TEXT,                              -- base64 или URL
    organizer_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tags             JSONB NOT NULL DEFAULT '[]',
    speakers         JSONB NOT NULL DEFAULT '[]',
    registered_count INT  NOT NULL DEFAULT 0,
    views_count      INT  NOT NULL DEFAULT 0,
    comments_count   INT  NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_date        ON events(date);
CREATE INDEX IF NOT EXISTS idx_events_type        ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_format      ON events(format);
CREATE INDEX IF NOT EXISTS idx_events_organizer   ON events(organizer_id);
CREATE INDEX IF NOT EXISTS idx_events_created_at  ON events(created_at DESC);

-- Регистрации на мероприятия
CREATE TABLE IF NOT EXISTS event_registrations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_reg_event ON event_registrations(event_id);
CREATE INDEX IF NOT EXISTS idx_event_reg_user  ON event_registrations(user_id);

-- Триггер: пересчёт registered_count после вставки/удаления регистраций
CREATE OR REPLACE FUNCTION update_event_registered_count()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE events SET registered_count = registered_count + 1 WHERE id = NEW.event_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE events SET registered_count = GREATEST(0, registered_count - 1) WHERE id = OLD.event_id;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_event_registered_count ON event_registrations;
CREATE TRIGGER trg_event_registered_count
AFTER INSERT OR DELETE ON event_registrations
FOR EACH ROW EXECUTE FUNCTION update_event_registered_count();

-- Маршрут /events.html — уже обслуживается Go как статика из web/
-- Маршрут /exhibitions.html — аналогично
