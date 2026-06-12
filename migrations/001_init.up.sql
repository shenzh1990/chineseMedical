CREATE TABLE IF NOT EXISTS app_health_events (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_health_events_created_at
    ON app_health_events (created_at DESC);

CREATE TABLE IF NOT EXISTS app_users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_medicated_food (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    food TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    effect TEXT NOT NULL DEFAULT '',
    create_by TEXT,
    create_time TIMESTAMPTZ,
    update_by TEXT,
    update_time TIMESTAMPTZ
);
