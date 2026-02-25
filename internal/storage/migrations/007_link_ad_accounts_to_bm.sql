-- Migration 007: vincular ad_accounts -> business_managers
--
-- Objetivo:
-- - mapear ad_account_id para bm_uuid no banco
-- - permitir que o backend resolva config/token via Secret Manager por BM

BEGIN;

ALTER TABLE ad_accounts
  ADD COLUMN IF NOT EXISTS bm_uuid UUID;

ALTER TABLE ad_accounts
  DROP CONSTRAINT IF EXISTS fk_ad_accounts_bm;

ALTER TABLE ad_accounts
  ADD CONSTRAINT fk_ad_accounts_bm
  FOREIGN KEY (bm_uuid)
  REFERENCES business_managers(bm_uuid)
  ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_ad_accounts_bm_uuid
  ON ad_accounts(bm_uuid)
  WHERE deleted_at IS NULL;

COMMIT;
