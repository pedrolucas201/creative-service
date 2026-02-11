-- Migration 005: Remover client_id legado da tabela creatives
-- 
-- Motivação: client_id é redundante - creatives já tem client_uuid como FK
-- Isso elimina:
--   1. Redundância de dados (violação de normalização)
--   2. Risco de inconsistência (duas FKs para o mesmo relacionamento)
--   3. Query extra no código (buscar client_id do banco)
--   4. Complexidade desnecessária
--
-- Impacto: 
--   - Remove coluna client_id de creatives
--   - Remove FK constraint
--   - Remove index
--   - Código deve usar apenas client_uuid

BEGIN;

-- ====================
-- STEP 1: Verificações de segurança
-- ====================

DO $$
DECLARE
    v_creatives_count INT;
BEGIN
    -- Verificar se há creatives na tabela
    SELECT COUNT(*) INTO v_creatives_count FROM creatives;
    
    RAISE NOTICE 'Migration 005: % creatives encontrados', v_creatives_count;
    
    -- Verificar se todos os creatives têm client_uuid válido
    IF EXISTS (
        SELECT 1 FROM creatives 
        WHERE client_uuid IS NULL 
        OR NOT EXISTS (
            SELECT 1 FROM clients 
            WHERE clients.client_uuid = creatives.client_uuid
        )
    ) THEN
        RAISE EXCEPTION 'MIGRATION ABORTED: Existem creatives sem client_uuid válido!';
    END IF;
    
    RAISE NOTICE 'Todas as verificações de segurança passaram ✓';
END $$;

-- ====================
-- STEP 2: Remover FK constraint
-- ====================

ALTER TABLE creatives DROP CONSTRAINT IF EXISTS creatives_client_id_fkey;

-- ====================
-- STEP 3: Remover índice
-- ====================

DROP INDEX IF EXISTS creatives_client_idx;
DROP INDEX IF EXISTS idx_creatives_client_id;

-- ====================
-- STEP 4: Dropar coluna client_id
-- ====================

ALTER TABLE creatives DROP COLUMN IF EXISTS client_id;

-- ====================
-- STEP 5: Verificação final
-- ====================

DO $$
BEGIN
    -- Verificar se coluna foi realmente removida
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'creatives' 
        AND column_name = 'client_id'
    ) THEN
        RAISE EXCEPTION 'MIGRATION FAILED: Coluna client_id ainda existe!';
    END IF;
    
    RAISE NOTICE 'Migration 005 concluída com sucesso! ✓';
    RAISE NOTICE '';
    RAISE NOTICE 'Estrutura final da tabela creatives:';
    RAISE NOTICE '  - creative_id (PK)';
    RAISE NOTICE '  - client_uuid (FK) ← Única referência ao cliente';
    RAISE NOTICE '  - ad_account_id (FK)';
    RAISE NOTICE '  - + campos de dados';
END $$;

COMMIT;

-- ====================
-- NOTAS PÓS-MIGRATION:
-- ====================
--
-- Próximos passos no código:
-- 1. Remover campo ClientID do struct Creative (postgres.go)
-- 2. Atualizar queries SQL (CreateCreative, GetCreative, ListCreatives)
-- 3. Remover ClientID dos service structs (ImageCreativeInput, etc)
-- 4. Remover fallback client_id dos handlers HTTP
-- 5. Atualizar testes
--
-- Benefícios:
-- ✅ 1 query a menos por creative criado
-- ✅ Impossível ter FKs inconsistentes
-- ✅ Código mais limpo e manutenível
-- ✅ Banco normalizado corretamente
