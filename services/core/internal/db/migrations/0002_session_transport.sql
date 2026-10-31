-- Bind sessions to transport; previously created sessions stay HTTPS-only.
ALTER TABLE sessions ADD COLUMN transport text NOT NULL DEFAULT 'https'
    CONSTRAINT sessions_transport_valid CHECK (transport IN ('http','https'));
CREATE INDEX sessions_user_transport_idx ON sessions (user_id, transport);
