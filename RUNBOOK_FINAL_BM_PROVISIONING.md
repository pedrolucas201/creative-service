# Runbook Final - Provisionamento BM + AuthZ

Objetivo: cadastrar uma nova BM (ex.: Danilo) ponta a ponta, ligar `ad_account_id` a `bm_uuid`, configurar Secret Manager e validar 401/200/403.

## 1) Pré-requisitos

- Projeto GCP: `rogakronos`
- Banco já com migrations `007` e `008`
- Backend com `AUTH_REQUIRED=true` em produção
- Você já possui um `ID_TOKEN` Firebase válido para testes

## 2) SQL - criar BM e vincular ad account

Use no Cloud SQL (Postgres):

```sql
BEGIN;

-- 1) Criar BM (secret_name no padrão bm_uuid, ajustado depois)
WITH new_bm AS (
  INSERT INTO business_managers (
    client_uuid,
    bm_id,
    project_id,
    secret_name,
    is_active
  )
  VALUES (
    '<CLIENT_UUID_DANILO>',
    '<BM_ID_DANILO>',
    'rogakronos',
    'TEMP_SECRET_NAME',
    TRUE
  )
  RETURNING bm_uuid
)
UPDATE business_managers bm
SET secret_name = new_bm.bm_uuid::text
FROM new_bm
WHERE bm.bm_uuid = new_bm.bm_uuid;

-- 2) Vincular ad account da Meta na BM
UPDATE ad_accounts
SET bm_uuid = (
  SELECT bm_uuid
  FROM business_managers
  WHERE bm_id = '<BM_ID_DANILO>'
)
WHERE ad_account_id = 'act_1279XXXXXXXXX'
  AND deleted_at IS NULL;

COMMIT;
```

Conferência:

```sql
SELECT bm_uuid, bm_id, project_id, secret_name, is_active
FROM business_managers
WHERE bm_id = '<BM_ID_DANILO>';

SELECT ad_account_id, bm_uuid
FROM ad_accounts
WHERE ad_account_id = 'act_1279XXXXXXXXX';
```

## 3) Secret Manager - criar secret da BM

Descubra o `bm_uuid` criado:

```sql
SELECT bm_uuid::text
FROM business_managers
WHERE bm_id = '<BM_ID_DANILO>';
```

No terminal:

```powershell
$PROJECT_ID = "rogakronos"
$BM_UUID = "<BM_UUID_AQUI>"
$SA = "creative-service-runtime@rogakronos.iam.gserviceaccount.com"
```

Criar secret com nome = `bm_uuid`:

```powershell
gcloud secrets create $BM_UUID --replication-policy="automatic" --project=$PROJECT_ID
```

Criar arquivo JSON (exemplo `bm-config.json`):

```json
{
  "token_ref": "SM:rogakronos/meta-prod-token-danilo-01",
  "ad_account_id": "act_1279XXXXXXXXX",
  "page_id": "123456789012345",
  "bm_id": "<BM_ID_DANILO>"
}
```

Adicionar versão:

```powershell
gcloud secrets versions add $BM_UUID --data-file="bm-config.json" --project=$PROJECT_ID
```

Dar acesso apenas a esse secret para a service account do backend:

```powershell
gcloud secrets add-iam-policy-binding $BM_UUID `
  --member="serviceAccount:$SA" `
  --role="roles/secretmanager.secretAccessor" `
  --project=$PROJECT_ID
```

## 4) Seed de acesso do usuário (uid -> bm_uuid)

```sql
BEGIN;

INSERT INTO app_users(uid, email, is_active)
VALUES ('<FIREBASE_UID>', '<EMAIL_USUARIO>', TRUE)
ON CONFLICT (uid)
DO UPDATE SET email = EXCLUDED.email, updated_at = now();

INSERT INTO user_bm_access(uid, bm_uuid, role, is_active)
VALUES (
  '<FIREBASE_UID>',
  '<BM_UUID_AQUI>'::uuid,
  'admin',
  TRUE
)
ON CONFLICT (uid, bm_uuid)
DO UPDATE SET role = EXCLUDED.role, is_active = EXCLUDED.is_active, updated_at = now();

COMMIT;
```

## 5) Testes PowerShell (401/200/403)

Variáveis:

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$TOKEN = "<ID_TOKEN_FIREBASE>"
```

Teste 401 (sem token):

```powershell
Invoke-RestMethod -Method GET -Uri "$BASE/v1/me"
```

Esperado: erro `401` com `missing_authorization_header`.

Teste 200 (`/v1/me` com token válido):

```powershell
Invoke-RestMethod -Method GET -Uri "$BASE/v1/me" -Headers @{ Authorization = "Bearer $TOKEN" }
```

Esperado: `uid` e `email`.

Teste 200 em ad account permitida:

```powershell
$bodyAllowed = @{
  ad_account_id = "act_1279XXXXXXXXX"
  name = "RBAC Allowed Test"
  objective = "OUTCOME_TRAFFIC"
  status = "PAUSED"
  special_ad_categories = @()
  buying_type = "AUCTION"
  is_adset_budget_sharing_enabled = $false
} | ConvertTo-Json -Depth 6

Invoke-RestMethod -Method POST -Uri "$BASE/v1/campaigns" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $bodyAllowed
```

Esperado: `200` com `campaign_id`.

Teste 403 em ad account sem vínculo:

```powershell
$bodyDenied = @{
  ad_account_id = "act_999999999999999"
  name = "RBAC Denied Test"
  objective = "OUTCOME_TRAFFIC"
  status = "PAUSED"
  special_ad_categories = @()
  buying_type = "AUCTION"
  is_adset_budget_sharing_enabled = $false
} | ConvertTo-Json -Depth 6

Invoke-RestMethod -Method POST -Uri "$BASE/v1/campaigns" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $bodyDenied
```

Esperado: `403` com `forbidden_for_ad_account`.

## 6) Critério de pronto

- BM criada e `is_active=true`
- `ad_accounts.bm_uuid` vinculado corretamente
- Secret com nome `bm_uuid` criado com versão `latest`
- SA do backend com `secretAccessor` no secret
- `user_bm_access` seedado para o UID alvo
- Testes 401/200/403 passando

## 7) Alerta antes de carimbar produção

No código atual, `PATCH /v1/ads/{ad_id}` e `DELETE /v1/ads/{ad_id}` ainda não aplicam `requireAdAccountAccess` como os demais endpoints. Corrigir isso evita bypass de autorização por ad account nessas duas rotas.

