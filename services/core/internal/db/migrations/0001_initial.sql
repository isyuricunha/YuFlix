CREATE TABLE IF NOT EXISTS schema_migrations (
    version integer PRIMARY KEY,
    checksum text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT username_valid CHECK (username ~ '^[a-z][a-z0-9_]{2,31}$')
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expiry_idx ON sessions (expires_at);

CREATE TABLE settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE watch_progress (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type text NOT NULL CHECK (content_type IN ('movie', 'series', 'anime')),
    content_id text NOT NULL,
    episode_id text NOT NULL DEFAULT '',
    position_seconds integer NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
    duration_seconds integer CHECK (duration_seconds >= 0),
    completed boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, content_type, content_id, episode_id),
    CONSTRAINT progress_content_id_valid CHECK (length(content_id) BETWEEN 1 AND 200),
    CONSTRAINT progress_episode_id_valid CHECK (length(episode_id) <= 200)
);
CREATE INDEX progress_recent_idx ON watch_progress (user_id, updated_at DESC);

CREATE TABLE source_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    type text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE source_configs IS 'Future shared source configuration. No secret-bearing endpoints exist yet.';
