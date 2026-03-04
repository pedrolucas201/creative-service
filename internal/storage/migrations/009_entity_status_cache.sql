-- Migration 009: cache de status por entidade de mídia
--
-- Objetivo:
-- - guardar último status consultado na Meta API sem job/cron
-- - permitir retorno rápido para o cliente e histórico de sincronização

BEGIN;

CREATE TABLE IF NOT EXISTS entity_status_cache (
  entity_type    TEXT NOT NULL,
  entity_id      TEXT NOT NULL,
  ad_account_id  TEXT NOT NULL REFERENCES ad_accounts(ad_account_id) ON DELETE CASCADE,
  status         TEXT,
  raw_payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
  synced_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (entity_type, entity_id),
  CONSTRAINT chk_entity_status_cache_type
    CHECK (entity_type IN ('creative', 'campaign', 'adset', 'ad'))
);

CREATE INDEX IF NOT EXISTS idx_entity_status_cache_account_type
  ON entity_status_cache(ad_account_id, entity_type, synced_at DESC);

CREATE INDEX IF NOT EXISTS idx_entity_status_cache_account
  ON entity_status_cache(ad_account_id, synced_at DESC);

COMMIT;
