-- Bounded persistent metadata snapshots. Never persist signed playback URLs.
CREATE TABLE catalog_snapshots (
    source_id uuid NOT NULL REFERENCES source_configs(id) ON DELETE CASCADE,
    content_type text NOT NULL CHECK (content_type IN ('movie', 'series')),
    catalog_id text NOT NULL CHECK (length(catalog_id) BETWEEN 1 AND 100),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'array'),
    refreshed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source_id,content_type,catalog_id)
);
CREATE INDEX catalog_snapshots_refreshed_idx ON catalog_snapshots(refreshed_at);
