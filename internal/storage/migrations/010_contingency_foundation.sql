-- Migration 010: Fundacao da contingencia automatica
--
-- Objetivo:
-- - registrar nos e rotas de contingencia
-- - registrar incidentes com idempotencia por campanha
-- - registrar execucoes e mapeamento de switch

BEGIN;

CREATE TABLE IF NOT EXISTS contingency_nodes (
  node_uuid        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bm_uuid          UUID NOT NULL REFERENCES business_managers(bm_uuid) ON DELETE CASCADE,
  ad_account_id    TEXT NOT NULL REFERENCES ad_accounts(ad_account_id) ON DELETE CASCADE,
  node_name        TEXT NOT NULL DEFAULT '',
  priority         INT NOT NULL DEFAULT 100,
  weight           INT NOT NULL DEFAULT 100,
  is_active        BOOLEAN NOT NULL DEFAULT TRUE,
  cooldown_until   TIMESTAMPTZ,
  last_used_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_contingency_nodes_account UNIQUE(ad_account_id),
  CONSTRAINT chk_contingency_nodes_weight CHECK (weight > 0)
);

CREATE INDEX IF NOT EXISTS idx_contingency_nodes_bm
  ON contingency_nodes(bm_uuid, is_active, priority);

CREATE TABLE IF NOT EXISTS contingency_routes (
  route_uuid             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_ad_account_id   TEXT NOT NULL REFERENCES ad_accounts(ad_account_id) ON DELETE CASCADE,
  target_node_uuid       UUID NOT NULL REFERENCES contingency_nodes(node_uuid) ON DELETE CASCADE,
  order_index            INT NOT NULL DEFAULT 1,
  is_active              BOOLEAN NOT NULL DEFAULT TRUE,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_contingency_routes UNIQUE(source_ad_account_id, target_node_uuid)
);

CREATE INDEX IF NOT EXISTS idx_contingency_routes_source
  ON contingency_routes(source_ad_account_id, is_active, order_index);

CREATE TABLE IF NOT EXISTS contingency_incidents (
  incident_uuid          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_campaign_id     TEXT NOT NULL,
  source_ad_account_id   TEXT NOT NULL REFERENCES ad_accounts(ad_account_id) ON DELETE CASCADE,
  trigger_type           TEXT NOT NULL,
  reason_code            TEXT NOT NULL,
  reason_detail          TEXT,
  evidence               JSONB NOT NULL DEFAULT '{}'::jsonb,
  status                 TEXT NOT NULL DEFAULT 'detected',
  attempt_count          INT NOT NULL DEFAULT 0,
  opened_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at              TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_contingency_incidents_trigger_type
    CHECK (trigger_type IN ('webhook', 'polling', 'manual')),
  CONSTRAINT chk_contingency_incidents_status
    CHECK (status IN ('detected', 'queued', 'executing', 'switched', 'failed', 'manual_required', 'rolled_back', 'closed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contingency_incident_open_unique
  ON contingency_incidents(source_campaign_id, source_ad_account_id)
  WHERE status IN ('detected', 'queued', 'executing');

CREATE INDEX IF NOT EXISTS idx_contingency_incidents_account_status
  ON contingency_incidents(source_ad_account_id, status, opened_at DESC);

CREATE INDEX IF NOT EXISTS idx_contingency_incidents_campaign
  ON contingency_incidents(source_campaign_id, opened_at DESC);

CREATE TABLE IF NOT EXISTS contingency_executions (
  execution_uuid         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_uuid          UUID NOT NULL REFERENCES contingency_incidents(incident_uuid) ON DELETE CASCADE,
  attempt                INT NOT NULL,
  target_node_uuid       UUID REFERENCES contingency_nodes(node_uuid) ON DELETE SET NULL,
  status                 TEXT NOT NULL,
  error_code             TEXT,
  error_message          TEXT,
  started_at             TIMESTAMPTZ,
  finished_at            TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_contingency_executions_attempt UNIQUE(incident_uuid, attempt),
  CONSTRAINT chk_contingency_executions_status
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_contingency_executions_incident
  ON contingency_executions(incident_uuid, created_at DESC);

CREATE TABLE IF NOT EXISTS entity_switch_map (
  switch_uuid            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incident_uuid          UUID NOT NULL REFERENCES contingency_incidents(incident_uuid) ON DELETE CASCADE,
  source_campaign_id     TEXT NOT NULL,
  target_campaign_id     TEXT,
  source_adset_ids       JSONB NOT NULL DEFAULT '[]'::jsonb,
  target_adset_ids       JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_ad_ids          JSONB NOT NULL DEFAULT '[]'::jsonb,
  target_ad_ids          JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_entity_switch_map_incident
  ON entity_switch_map(incident_uuid, created_at DESC);

COMMIT;
