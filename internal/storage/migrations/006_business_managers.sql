-- Migration 006: tabela business_managers para mapear BM -> Secret Manager
--
-- Fluxo:
-- - DB guarda bm_uuid, project_id e secret_name
-- - secret_name aponta para um secret no Secret Manager com JSON da BM

BEGIN;

CREATE TABLE IF NOT EXISTS business_managers (
  bm_uuid      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_uuid  UUID NOT NULL REFERENCES clients(client_uuid) ON DELETE CASCADE,
  bm_id        TEXT NOT NULL,
  project_id   TEXT NOT NULL,
  secret_name  TEXT NOT NULL,
  is_active    BOOLEAN NOT NULL DEFAULT TRUE,
  deleted_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_business_managers_bm_id UNIQUE (bm_id),
  CONSTRAINT uq_business_managers_secret_name UNIQUE (secret_name)
);

CREATE INDEX IF NOT EXISTS idx_bm_client_uuid
  ON business_managers(client_uuid)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_bm_is_active
  ON business_managers(is_active)
  WHERE deleted_at IS NULL;

COMMIT;
