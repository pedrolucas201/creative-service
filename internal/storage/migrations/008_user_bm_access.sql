-- Migration 008: autenticação/autorização por usuário Firebase (uid) e BM

BEGIN;

CREATE TABLE IF NOT EXISTS app_users (
  uid         TEXT PRIMARY KEY,
  email       TEXT,
  is_active   BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_bm_access (
  uid         TEXT NOT NULL REFERENCES app_users(uid) ON DELETE CASCADE,
  bm_uuid     UUID NOT NULL REFERENCES business_managers(bm_uuid) ON DELETE CASCADE,
  role        TEXT NOT NULL DEFAULT 'viewer',
  is_active   BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (uid, bm_uuid),
  CONSTRAINT chk_user_bm_access_role CHECK (role IN ('owner','admin','operator','viewer'))
);

CREATE INDEX IF NOT EXISTS idx_user_bm_access_bm_uuid
  ON user_bm_access(bm_uuid)
  WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_user_bm_access_uid
  ON user_bm_access(uid)
  WHERE is_active = TRUE;

COMMIT;
