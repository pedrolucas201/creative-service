CREATE TABLE IF NOT EXISTS creatives (
    creative_id TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES clients(client_id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    s3_url TEXT NOT NULL,
    s3_thumb_url TEXT,
    link TEXT,
    message TEXT,
    meta_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS creatives_client_idx ON creatives(client_id);
CREATE INDEX IF NOT EXISTS creatives_type_idx ON creatives(type);
CREATE INDEX IF NOT EXISTS creatives_created_idx ON creatives(created_at);