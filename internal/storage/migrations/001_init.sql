CREATE TABLE IF NOT EXISTS clients (
  client_id TEXT PRIMARY KEY,
  ad_account_id TEXT NOT NULL,
  page_id TEXT NOT NULL,
  token_ref TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'job_status') THEN
    CREATE TYPE job_status AS ENUM ('queued','running','succeeded','failed');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS jobs (
  job_id TEXT PRIMARY KEY,
  client_id TEXT NOT NULL REFERENCES clients(client_id),
  job_type TEXT NOT NULL,
  status job_status NOT NULL,
  input_json JSONB NOT NULL,
  blob_video_path TEXT,
  blob_thumb_path TEXT,
  result_json JSONB,
  error_text TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS jobs_client_idx ON jobs(client_id);
CREATE INDEX IF NOT EXISTS jobs_status_idx ON jobs(status);
