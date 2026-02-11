-- Migration 003: Refatoração para Multi-Account com Soft Delete
-- Descrição: Adiciona UUID aos clients, cria tabela ad_accounts, adiciona soft delete

-- ============================================================================
-- STEP 1: Backup (caso precise reverter)
-- ============================================================================
-- Não fazemos backup em SQL, mas é bom fazer pg_dump antes de rodar

-- ============================================================================
-- STEP 2: Adicionar colunas novas na tabela clients
-- ============================================================================

-- Adiciona UUID (temporariamente nullable até preencher)
ALTER TABLE clients ADD COLUMN IF NOT EXISTS client_uuid UUID;

-- Adiciona campos novos
ALTER TABLE clients ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Preenche UUID para registros existentes
UPDATE clients SET client_uuid = gen_random_uuid() WHERE client_uuid IS NULL;

-- Preenche name baseado no client_id (temporário, você pode ajustar depois)
UPDATE clients SET name = INITCAP(client_id) WHERE name IS NULL;

-- Agora torna UUID NOT NULL e adiciona constraint unique
ALTER TABLE clients ALTER COLUMN client_uuid SET NOT NULL;
ALTER TABLE clients ADD CONSTRAINT clients_uuid_unique UNIQUE (client_uuid);

-- ============================================================================
-- STEP 3: Criar nova tabela ad_accounts
-- ============================================================================

CREATE TABLE IF NOT EXISTS ad_accounts (
  ad_account_uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_uuid UUID NOT NULL,
  ad_account_id TEXT NOT NULL,
  ad_account_name TEXT NOT NULL,
  page_id TEXT NOT NULL,
  token_ref TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT fk_ad_accounts_client 
    FOREIGN KEY (client_uuid) 
    REFERENCES clients(client_uuid) 
    ON DELETE CASCADE,
  
  CONSTRAINT unique_client_ad_account 
    UNIQUE(client_uuid, ad_account_id)
);

-- Índices para performance
CREATE INDEX IF NOT EXISTS ad_accounts_client_idx ON ad_accounts(client_uuid);
CREATE INDEX IF NOT EXISTS ad_accounts_active_idx ON ad_accounts(is_active);
CREATE INDEX IF NOT EXISTS ad_accounts_deleted_idx ON ad_accounts(deleted_at) WHERE deleted_at IS NULL;

-- ============================================================================
-- STEP 4: Migrar dados de clients para ad_accounts
-- ============================================================================

-- Para cada client existente, cria um ad_account
INSERT INTO ad_accounts (
  client_uuid, 
  ad_account_id, 
  ad_account_name, 
  page_id, 
  token_ref
)
SELECT 
  c.client_uuid,
  c.ad_account_id,
  'Conta Principal',  -- Nome padrão (você pode ajustar depois)
  c.page_id,
  c.token_ref
FROM clients c
WHERE NOT EXISTS (
  SELECT 1 FROM ad_accounts aa 
  WHERE aa.client_uuid = c.client_uuid 
  AND aa.ad_account_id = c.ad_account_id
);

-- ============================================================================
-- STEP 5: Atualizar tabela creatives
-- ============================================================================

-- Adiciona novas colunas
ALTER TABLE creatives ADD COLUMN IF NOT EXISTS client_uuid UUID;
ALTER TABLE creatives ADD COLUMN IF NOT EXISTS ad_account_uuid UUID;
ALTER TABLE creatives ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Preenche client_uuid baseado no client_id existente
UPDATE creatives cr
SET client_uuid = c.client_uuid
FROM clients c
WHERE cr.client_id = c.client_id
AND cr.client_uuid IS NULL;

-- Preenche ad_account_uuid (pega primeira conta do cliente)
UPDATE creatives cr
SET ad_account_uuid = (
  SELECT aa.ad_account_uuid 
  FROM ad_accounts aa 
  WHERE aa.client_uuid = cr.client_uuid 
  LIMIT 1
)
WHERE cr.ad_account_uuid IS NULL;

-- Torna colunas NOT NULL
ALTER TABLE creatives ALTER COLUMN client_uuid SET NOT NULL;
ALTER TABLE creatives ALTER COLUMN ad_account_uuid SET NOT NULL;

-- Adiciona foreign keys
ALTER TABLE creatives ADD CONSTRAINT fk_creatives_client 
  FOREIGN KEY (client_uuid) 
  REFERENCES clients(client_uuid) 
  ON DELETE CASCADE;

ALTER TABLE creatives ADD CONSTRAINT fk_creatives_ad_account 
  FOREIGN KEY (ad_account_uuid) 
  REFERENCES ad_accounts(ad_account_uuid) 
  ON DELETE CASCADE;

-- Índices para performance
CREATE INDEX IF NOT EXISTS creatives_client_uuid_idx ON creatives(client_uuid);
CREATE INDEX IF NOT EXISTS creatives_ad_account_uuid_idx ON creatives(ad_account_uuid);
CREATE INDEX IF NOT EXISTS creatives_deleted_idx ON creatives(deleted_at) WHERE deleted_at IS NULL;

-- ============================================================================
-- STEP 6: Limpar colunas antigas (CUIDADO! Só faça depois de testar)
-- ============================================================================

-- COMENTADO POR SEGURANÇA - Descomente apenas quando testar tudo
-- ALTER TABLE clients DROP COLUMN IF EXISTS ad_account_id;
-- ALTER TABLE clients DROP COLUMN IF EXISTS page_id;
-- ALTER TABLE clients DROP COLUMN IF EXISTS token_ref;
-- ALTER TABLE creatives DROP COLUMN IF EXISTS client_id;

-- ============================================================================
-- STEP 7: Criar constraint para impedir client_uuid duplicado como PK
-- ============================================================================

-- Adiciona client_uuid como nova PK (remove client_id como PK)
-- COMENTADO - Isso é complexo e pode quebrar FKs existentes
-- Deixe client_id como PK por enquanto, client_uuid é UNIQUE

-- ============================================================================
-- VERIFICAÇÕES FINAIS
-- ============================================================================

-- Verifica se todos os clients têm UUID
DO $$
DECLARE
  missing_uuid INTEGER;
BEGIN
  SELECT COUNT(*) INTO missing_uuid FROM clients WHERE client_uuid IS NULL;
  IF missing_uuid > 0 THEN
    RAISE EXCEPTION 'Existem % clients sem UUID!', missing_uuid;
  END IF;
END $$;

-- Verifica se todos os creatives têm client_uuid e ad_account_uuid
DO $$
DECLARE
  missing_client INTEGER;
  missing_account INTEGER;
BEGIN
  SELECT COUNT(*) INTO missing_client FROM creatives WHERE client_uuid IS NULL;
  SELECT COUNT(*) INTO missing_account FROM creatives WHERE ad_account_uuid IS NULL;
  
  IF missing_client > 0 THEN
    RAISE EXCEPTION 'Existem % creatives sem client_uuid!', missing_client;
  END IF;
  
  IF missing_account > 0 THEN
    RAISE EXCEPTION 'Existem % creatives sem ad_account_uuid!', missing_account;
  END IF;
END $$;

-- ============================================================================
-- SUCESSO!
-- ============================================================================
-- Migration concluída com sucesso!
-- Próximos passos:
-- 1. Testar aplicação
-- 2. Ajustar nomes de ad_accounts manualmente se necessário
-- 3. Após 1 semana de testes, descomentar STEP 6 para remover colunas antigas
