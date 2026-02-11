-- Migration 004: Simplificar - usar ad_account_id como PK direta (sem UUID interno)
-- 
-- Motivação: ad_account_id da Meta já é único globalmente (act_123456789)
-- Não precisamos de um UUID interno adicional, isso adiciona complexidade sem benefício
--
-- Mudanças:
-- 1. Remover ad_account_uuid da tabela ad_accounts
-- 2. Tornar ad_account_id a PRIMARY KEY
-- 3. Atualizar creatives para usar ad_account_id como FK
-- 4. Remover ad_account_uuid de creatives

BEGIN;

-- ====================
-- STEP 1: Backup tables (safety measure)
-- ====================
-- Caso precise reverter, dados estão preservados

-- ====================
-- STEP 2: Recriar tabela ad_accounts com ad_account_id como PK
-- ====================

-- Primeiro, remover FK de creatives
ALTER TABLE creatives DROP CONSTRAINT IF EXISTS fk_creatives_ad_account;

-- Dropar tabela antiga ad_accounts
DROP TABLE IF EXISTS ad_accounts CASCADE;

-- Recriar com estrutura simplificada
CREATE TABLE ad_accounts (
    ad_account_id   TEXT PRIMARY KEY,                    -- act_123456789 (PK direto!)
    client_uuid     UUID NOT NULL,
    ad_account_name TEXT NOT NULL,
    page_id         TEXT NOT NULL,
    token_ref       TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    CONSTRAINT fk_ad_accounts_client 
        FOREIGN KEY (client_uuid) 
        REFERENCES clients(client_uuid) 
        ON DELETE CASCADE
);

-- Index para buscar ad accounts por cliente
CREATE INDEX idx_ad_accounts_client_uuid ON ad_accounts(client_uuid) WHERE deleted_at IS NULL;

-- Index para soft delete
CREATE INDEX idx_ad_accounts_deleted_at ON ad_accounts(deleted_at) WHERE deleted_at IS NULL;

-- ====================
-- STEP 3: Migrar dados existentes da migration 003
-- ====================

-- Inserir ad accounts baseado nos dados da tabela clients
-- (cada client virou 1 ad account na migration 003)
INSERT INTO ad_accounts (
    ad_account_id, 
    client_uuid, 
    ad_account_name, 
    page_id, 
    token_ref, 
    is_active
)
SELECT 
    c.ad_account_id,              -- Usar o ad_account_id que estava no clients
    c.client_uuid,                -- FK para o cliente
    COALESCE(c.name, 'Default Account') AS ad_account_name,
    c.page_id,
    c.token_ref,
    true AS is_active
FROM clients c
WHERE c.ad_account_id IS NOT NULL
  AND c.deleted_at IS NULL
ON CONFLICT (ad_account_id) DO NOTHING;  -- Evitar duplicatas

-- ====================
-- STEP 4: Atualizar tabela creatives
-- ====================

-- Remover coluna ad_account_uuid (não precisamos mais)
ALTER TABLE creatives DROP COLUMN IF EXISTS ad_account_uuid;

-- Adicionar coluna ad_account_id se não existir
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'creatives' 
        AND column_name = 'ad_account_id'
    ) THEN
        ALTER TABLE creatives ADD COLUMN ad_account_id TEXT;
    END IF;
END $$;

-- Migrar dados: pegar ad_account_id do client relacionado
UPDATE creatives cr
SET ad_account_id = c.ad_account_id
FROM clients c
WHERE cr.client_uuid = c.client_uuid
  AND cr.ad_account_id IS NULL
  AND c.ad_account_id IS NOT NULL;

-- Tornar NOT NULL após popular
ALTER TABLE creatives ALTER COLUMN ad_account_id SET NOT NULL;

-- Adicionar FK para ad_accounts
ALTER TABLE creatives 
    ADD CONSTRAINT fk_creatives_ad_account 
    FOREIGN KEY (ad_account_id) 
    REFERENCES ad_accounts(ad_account_id) 
    ON DELETE CASCADE;

-- Index para performance
CREATE INDEX idx_creatives_ad_account_id ON creatives(ad_account_id) WHERE deleted_at IS NULL;

-- ====================
-- STEP 5: Verificações de integridade
-- ====================

DO $$
DECLARE
    v_ad_account_count INT;
    v_creative_orphans INT;
BEGIN
    -- Verificar se há ad accounts criadas
    SELECT COUNT(*) INTO v_ad_account_count FROM ad_accounts;
    IF v_ad_account_count = 0 THEN
        RAISE EXCEPTION 'MIGRATION FAILED: No ad accounts created';
    END IF;
    
    -- Verificar se há creatives órfãos (sem ad account)
    SELECT COUNT(*) INTO v_creative_orphans 
    FROM creatives cr
    LEFT JOIN ad_accounts aa ON cr.ad_account_id = aa.ad_account_id
    WHERE aa.ad_account_id IS NULL;
    
    IF v_creative_orphans > 0 THEN
        RAISE EXCEPTION 'MIGRATION FAILED: % orphan creatives found', v_creative_orphans;
    END IF;
    
    RAISE NOTICE 'Migration 004 successful: % ad accounts created', v_ad_account_count;
END $$;

COMMIT;

-- ====================
-- NOTAS PÓS-MIGRATION:
-- ====================
--
-- Estrutura final:
--
-- ad_accounts:
--   PK: ad_account_id (TEXT) - act_123456789 da Meta
--   FK: client_uuid → clients(client_uuid)
--
-- creatives:
--   FK: ad_account_id → ad_accounts(ad_account_id)
--   FK: client_uuid → clients(client_uuid)
--
-- Path S3 será:
--   creatives/{type}/{client_uuid}-{client_name}/{ad_account_id}-{ad_account_name}/{creative_uuid}-{filename}
--   Exemplo: creatives/images/abc-123-Francisco/act_123456-Main_Campaign/def-456-produto.jpg
--
-- CLEANUP FUTURO (após 1 semana de testes):
--   - Pode remover client_id, ad_account_id, page_id, token_ref da tabela clients
--   - Essas colunas agora estão em ad_accounts
