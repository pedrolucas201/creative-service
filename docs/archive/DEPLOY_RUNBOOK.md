# Deploy Runbook (Cloud Run + Cloud SQL + GCS)

Projeto: `rogakronos`  
Região: `us-central1`  
Serviço: `creative-backend`  
Repo de imagem: `us-central1-docker.pkg.dev/rogakronos/titan-repo/backend`  
Cloud SQL: `rogakronos:us-central1:febe`  
Bucket GCS: `meta-service-storage`

---

## 1) Pré-check

```powershell
gcloud auth login
gcloud config set project rogakronos
gcloud auth configure-docker us-central1-docker.pkg.dev
```

Validar imagem atual em produção:

```powershell
gcloud run services describe creative-backend --region=us-central1 --format="value(spec.template.spec.containers[0].image)"
```

---

## 2) Build e Push da nova imagem

Escolha uma tag única por deploy:

```powershell
$TAG="devgomesss-YYYYMMDD-01"
cd C:\Users\PC\Downloads\creative-service
docker build -t us-central1-docker.pkg.dev/rogakronos/titan-repo/backend:$TAG .
docker push us-central1-docker.pkg.dev/rogakronos/titan-repo/backend:$TAG
```

---

## 3) Deploy no Cloud Run

### 3.1 Preparar DATABASE_URL com socket (recomendado)

```powershell
$pgPass = [uri]::EscapeDataString("Postgres@2026!")
$dbUrl = "postgres://postgres:$pgPass@/creatives?host=/cloudsql/rogakronos:us-central1:febe&sslmode=disable&connect_timeout=5"
```

### 3.2 Deploy completo

```powershell
gcloud run deploy creative-backend `
  --image=us-central1-docker.pkg.dev/rogakronos/titan-repo/backend:$TAG `
  --region=us-central1 `
  --platform=managed `
  --allow-unauthenticated `
  --port=8080 `
  --service-account=creative-service-runtime@rogakronos.iam.gserviceaccount.com `
  --add-cloudsql-instances=rogakronos:us-central1:febe `
  --set-env-vars="ADDR=:8080,STORAGE_PROVIDER=gcs,GCS_BUCKET=meta-service-storage,GCP_PROJECT_ID=rogakronos,DATABASE_URL=$dbUrl,META_BASE_URL=https://graph.facebook.com,META_API_VERSION=v24.0,HTTP_TIMEOUT=45s,MAX_CONCURRENCY=6,RUN_MIGRATIONS=false,TOKEN_FRANCISCO=SEU_TOKEN,TOKEN_CONTINGENCIA01=SEU_TOKEN"
```

---

## 4) Verificação pós-deploy

```powershell
curl.exe --max-time 20 https://creative-backend-663062637696.us-central1.run.app/v1/health
curl.exe --max-time 20 https://creative-backend-663062637696.us-central1.run.app/v1/clients
```

Teste funcional de criação de imagem:

```powershell
curl.exe -X POST "https://creative-backend-663062637696.us-central1.run.app/v1/creatives/image" `
  -F "ad_account_id=act_1427227328791737" `
  -F "name=Teste Producao Image" `
  -F "link=https://example.com" `
  -F "message=teste producao cloud run" `
  -F "headline=Teste" `
  -F "description=Teste" `
  -F "image=@C:\Users\PC\Downloads\creative-service\testdata\fixtures\test.jpg"
```

Esperado:
- `creative_id` e `image_hash`
- `validated: true`
- URL com `storage.googleapis.com/meta-service-storage/...`

---

## 5) Checks de configuração (quando der problema)

Service Account usada pelo serviço:

```powershell
gcloud run services describe creative-backend --region=us-central1 --format="value(spec.template.spec.serviceAccountName)"
```

Cloud SQL attachment:

```powershell
gcloud run services describe creative-backend --region=us-central1 --format="yaml(spec.template.metadata.annotations)"
```

Deve conter:
`run.googleapis.com/cloudsql-instances: rogakronos:us-central1:febe`

---

## 6) Rollback rápido

Listar revisões:

```powershell
gcloud run revisions list --service=creative-backend --region=us-central1
```

Redirecionar tráfego para revisão anterior:

```powershell
gcloud run services update-traffic creative-backend `
  --region=us-central1 `
  --to-revisions REVISAO_ANTERIOR=100
```

---

## 7) Atualização do Flutter (quando backend aprovado)

Base URL:

```text
https://creative-backend-663062637696.us-central1.run.app
```

Depois:
1. rebuild
2. teste fim a fim

---

## 8) Boas práticas pós-go-live

1. mover tokens/senha para Secret Manager  
2. reduzir permissões IAM para mínimo necessário  
3. manter tag única por deploy  
4. registrar revisão + imagem ativa em changelog

---

## 9) Incidente registrado (Meta `object_story_spec`)

Sintoma:
- erro Meta API `code=100` com subcodes de `Invalid parameter` durante criação de creative para uma conta específica.

Diagnóstico:
- Infra GCP estava saudável (Cloud Run, Cloud SQL, GCS).
- Token e ad account válidos.
- O bloqueio estava no lado Meta App/permissão efetiva para esse fluxo.

Resolução:
- publicar o app da Meta destravou o fluxo de criação.

Lição:
- para alguns fluxos de creative/story da Meta, o app em modo não publicado pode gerar erro de parâmetro inválido mesmo com token aparentemente válido.

---

## 10) Modelagem BM x Cliente (próximo passo de banco)

Estado atual:
- não existe identificação explícita de BM (`business_id`) no schema.
- hoje o sistema trata "cliente" como entidade principal e vincula várias ad accounts por `client_uuid`.

Problema:
- um cliente pode ter várias BMs.

Recomendação:
1. Criar tabela `business_managers`.
2. Relacionar `clients` <-> `business_managers` (1:N) ou (N:N) conforme regra real.
3. Adicionar FK de BM em `ad_accounts`.

Modelo sugerido (simples e escalável):

```sql
CREATE TABLE IF NOT EXISTS business_managers (
  bm_uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_uuid UUID NOT NULL REFERENCES clients(client_uuid) ON DELETE CASCADE,
  bm_id TEXT NOT NULL,        -- id da BM no Meta
  bm_name TEXT,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT bm_unique_per_client UNIQUE (client_uuid, bm_id)
);

ALTER TABLE ad_accounts
  ADD COLUMN IF NOT EXISTS bm_uuid UUID REFERENCES business_managers(bm_uuid);
```

Se precisar de cliente com múltiplas BMs e BM compartilhada, usar tabela de vínculo `client_business_managers` (N:N).

---

## 11) Secret Manager (dev/prod) - criar e buscar segredos

Objetivo:
- parar de manter token/senha em texto puro em env vars.

Padrão de nomes sugerido:
- `creative-dev-token-francisco`
- `creative-dev-token-danilo-01`
- `creative-dev-database-url`
- `creative-prod-token-francisco`
- `creative-prod-token-danilo-01`
- `creative-prod-database-url`

### 11.1 Criar segredo (primeira vez)

```powershell
gcloud secrets create creative-dev-token-danilo-01 --replication-policy=automatic
```

Adicionar valor:

```powershell
echo "VALOR_DO_TOKEN" | gcloud secrets versions add creative-dev-token-danilo-01 --data-file=-
```

### 11.2 Buscar/retornar segredo (o que seu chefe pediu)

```powershell
gcloud secrets versions access latest --secret=creative-dev-token-danilo-01
```

Para prod:

```powershell
gcloud secrets versions access latest --secret=creative-prod-token-danilo-01
```

### 11.3 Permissão para Cloud Run ler segredos

Dar role para a service account de runtime:

```powershell
gcloud projects add-iam-policy-binding rogakronos `
  --member="serviceAccount:creative-service-runtime@rogakronos.iam.gserviceaccount.com" `
  --role="roles/secretmanager.secretAccessor"
```

### 11.4 Injetar segredos no Cloud Run (sem expor valores)

```powershell
gcloud run services update creative-backend `
  --region=us-central1 `
  --set-secrets "TOKEN_DANILO_01=creative-prod-token-danilo-01:latest,DATABASE_URL=creative-prod-database-url:latest"
```

### 11.5 Estratégia Dev x Prod

- ambiente dev usa segredos `creative-dev-*`
- ambiente prod usa segredos `creative-prod-*`
- separar também por serviço Cloud Run (`creative-backend-dev` / `creative-backend-prod`) é recomendável.

---

## 12) Migration 006 + endpoint BM

Objetivo:
- garantir que ambientes novos tenham a tabela `business_managers`.
- validar o endpoint `GET /v1/bm/{bm_uuid}/config`.

### 12.1 Aplicar migration 006

Arquivo da migration:
- `internal/storage/migrations/006_business_managers.sql`

Script pronto (PowerShell):

```powershell
cd C:\Users\PC\Downloads\creative-service
.\ops\run_migration_006.ps1 -DatabaseUrl "postgres://USER:PASS@HOST:5432/creatives?sslmode=disable"
```

Se estiver em Cloud SQL Studio, copie e execute o SQL do arquivo diretamente.

### 12.2 Seed mínimo de BM (exemplo)

```sql
WITH x AS (SELECT gen_random_uuid() AS id)
INSERT INTO business_managers (bm_uuid, client_uuid, bm_id, project_id, secret_name)
SELECT
  id,
  'COLE_AQUI_O_CLIENT_UUID',
  '1025052152797587',
  'rogakronos',
  id::text
FROM x
RETURNING bm_uuid, secret_name;
```

### 12.3 Validar endpoint BM

```powershell
cd C:\Users\PC\Downloads\creative-service
.\ops\smoke_test_bm.ps1 `
  -BaseUrl "https://creative-backend-663062637696.us-central1.run.app" `
  -BMUUID "COLE_AQUI_O_BM_UUID"
```

Esperado em `bm config`:
- `token_ref`
- `ad_account_id`
- `page_id`
- `bm_id`

---

## 13) BM -> Secret Manager como fonte real (sem depender de env legado)

Objetivo:
- parar de usar `ad_accounts.token_ref` como fonte principal.
- usar `ad_account_id -> bm_uuid -> secret_name -> Secret Manager JSON`.
- manter Flutter sem mudanças.

### 13.1 Arquitetura final

Fluxo em runtime:
1. request chega com `ad_account_id`.
2. backend busca em `ad_accounts` o `bm_uuid`.
3. backend lê `business_managers` (`project_id`, `secret_name`).
4. backend lê o Secret Manager `projects/<project_id>/secrets/<secret_name>/versions/latest`.
5. backend resolve `token_ref` do JSON da BM.
6. backend chama Meta API.

### 13.2 Mudanças de banco

Migration criada para vínculo:
- `internal/storage/migrations/007_link_ad_accounts_to_bm.sql`

SQL principal:
```sql
ALTER TABLE ad_accounts ADD COLUMN IF NOT EXISTS bm_uuid UUID;
ALTER TABLE ad_accounts
  ADD CONSTRAINT fk_ad_accounts_bm
  FOREIGN KEY (bm_uuid) REFERENCES business_managers(bm_uuid) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_ad_accounts_bm_uuid
  ON ad_accounts(bm_uuid) WHERE deleted_at IS NULL;
```

### 13.3 Mudanças de código

Arquivos alterados:
- `internal/bm/service.go`
  - novo método `GetBMConfigByAdAccountID(ad_account_id)`.
  - valida consistência de `ad_account_id` entre request e secret.
- `internal/service/creatives_sync.go`
- `internal/service/campaigns.go`
- `internal/service/adsets.go`
- `internal/service/ads.go`
  - todos passaram a resolver token via BM/SM.
- `cmd/api/main.go`
  - injeção de `bmService` em todos os services.
- `internal/storage/postgres.go`
  - struct/query de `ad_accounts` com `bm_uuid`.

### 13.4 Prova prática (SM no lugar de env legado)

Pré-requisito:
- `ad_accounts.bm_uuid` preenchido para a conta.

Teste:
1. confirmar endpoint BM:
```powershell
curl.exe "https://creative-backend-663062637696.us-central1.run.app/v1/bms/<BM_UUID>/config"
```
2. em SQL, forçar valor inválido em `ad_accounts.token_ref`:
```sql
UPDATE ad_accounts
SET token_ref = 'ENV:TOKEN_INVALIDO_PARA_PROVA'
WHERE ad_account_id = 'act_1427227328791737'
  AND deleted_at IS NULL;
```
3. criar campanha via API.
4. se criar com sucesso, token veio do Secret Manager via BM (não do campo legado).

### 13.5 Erros encontrados e correções

1) `Secret Version ... is in DESTROYED state`
- causa: versão mais recente do secret destruída.
- correção: adicionar nova versão ativa no mesmo secret.

2) `invalid header field value for "Authorization"`
- causa: token do SM com BOM/newline/whitespace.
- correção em código:
  - `internal/secrets/sm_resolver.go`
  - aplicar `TrimPrefix("\\uFEFF")` + `TrimSpace()` antes de usar token.

3) `invalid_json` em teste de campanha via PowerShell
- causa: escape quebrado com `curl -d`.
- correção: usar `Invoke-RestMethod` + `ConvertTo-Json`.

### 13.6 Comandos de validação (padrão)

```powershell
$base = "https://creative-backend-663062637696.us-central1.run.app"
$ad   = "act_1427227328791737"

$bodyObj = @{
  ad_account_id = $ad
  name = "SM PROOF $(Get-Date -Format HHmmss)"
  objective = "OUTCOME_TRAFFIC"
  status = "PAUSED"
  special_ad_categories = @()
  buying_type = "AUCTION"
  is_adset_budget_sharing_enabled = $false
}
$body = $bodyObj | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post -Uri "$base/v1/campaigns" -ContentType "application/json" -Body $body
```

Resultado esperado:
- resposta com `campaign_id` válido.

### 13.7 Estado atual (concluído)

Concluído:
- rota BM funcionando: `GET /v1/bms/{bm_uuid}/config`
- backend usando BM/SM para creatives/campaigns/adsets/ads
- campanha criada com sucesso após integração

Pendente operacional:
- aplicar migration `007_link_ad_accounts_to_bm.sql` em todos os ambientes.
- preencher `bm_uuid` para todas as `ad_accounts` ativas.

---

## 14) Firebase Auth (Magic Link) no backend Go

Objetivo:
- exigir `Authorization: Bearer <Firebase ID Token>` nas rotas de negócio.
- manter `/v1/health` público.

### 14.1 Implementação aplicada no código

Arquivos novos:
- `internal/auth/firebase.go`
  - inicializa Firebase Admin SDK
  - valida ID token e extrai `uid/email`
- `internal/auth/context.go`
  - injeta identidade autenticada no `context.Context`
- `internal/httpapi/middleware_auth.go`
  - middleware Bearer token (401 para header/token inválido)

Arquivos alterados:
- `internal/httpapi/router.go`
  - novo `RouterOptions` com:
    - `RequireAuth`
    - `AuthVerifier`
  - rotas protegidas por grupo `chi.With(...)`
  - `GET /v1/health` segue público
- `cmd/api/main.go`
  - inicializa `FirebaseVerifier` quando `AUTH_REQUIRED=true`
  - injeta no router
- `internal/config/config.go`
  - novas env vars:
    - `AUTH_REQUIRED` (default: false)
    - `FIREBASE_PROJECT_ID` (fallback: `GCP_PROJECT_ID`)

### 14.2 Variáveis de ambiente

Obrigatórias para auth:
- `AUTH_REQUIRED=true`
- `FIREBASE_PROJECT_ID=glineui` (ou o project id Firebase correto)

Para desligar autenticação (dev local rápido):
- `AUTH_REQUIRED=false`

### 14.3 Comportamento das rotas

Pública:
- `GET /v1/health`

Protegidas por token Firebase:
- `clients`, `ad-accounts`
- `creatives` (image/video/list/get/delete)
- `campaigns`
- `adsets`
- `ads`
- `bms/{bm_uuid}/config`

### 14.4 Teste rápido

Sem token (esperado 401):
```powershell
curl.exe -i "https://creative-backend-663062637696.us-central1.run.app/v1/clients"
```

Com token:
```powershell
$TOKEN="ID_TOKEN_FIREBASE_AQUI"
curl.exe -i `
  -H "Authorization: Bearer $TOKEN" `
  "https://creative-backend-663062637696.us-central1.run.app/v1/clients"
```

### 14.5 Deploy com auth ligado

```powershell
gcloud run services update creative-backend `
  --region=us-central1 `
  --update-env-vars "AUTH_REQUIRED=true,FIREBASE_PROJECT_ID=glineui"
```

### 14.6 Observação importante

Este patch implementa autenticação (quem é o usuário).  
A autorização fina por recurso está documentada e implementada na seção 15 (`uid` -> `bm_uuid`).

---

## 15) Autorização fina por BM/usuário (uid -> bm_uuid)

Objetivo:
- usuário autenticado só pode operar `ad_account_id` vinculadas às BMs que ele possui acesso.

### 15.1 Mudanças de banco

Migration:
- `internal/storage/migrations/008_user_bm_access.sql`

Tabelas:
- `app_users(uid, email, is_active, created_at, updated_at)`
- `user_bm_access(uid, bm_uuid, role, is_active, ...)`

### 15.2 Mudanças de código

Arquivos:
- `internal/storage/postgres.go`
  - `EnsureAppUser(uid, email)` upsert do usuário autenticado
  - `UserCanAccessAdAccount(uid, ad_account_id)` valida vínculo
- `internal/httpapi/handlers.go`
  - novo endpoint `GET /v1/me`
  - helper `requireAdAccountAccess(...)`
  - aplicado em rotas com `ad_account_id`:
    - creatives (create/list/get/delete)
    - campaigns (create/list/update/delete)
    - adsets (create/list/update/delete)
    - ads (create/list/update/delete)
- `internal/httpapi/router.go`
  - rota protegida `GET /v1/me`

### 15.3 Seed mínimo para testar

1) Criar vínculo do usuário Firebase com BM:
```sql
INSERT INTO app_users(uid, email)
VALUES ('FIREBASE_UID_AQUI', 'usuario@empresa.com')
ON CONFLICT (uid) DO UPDATE SET email = EXCLUDED.email, updated_at = now();

INSERT INTO user_bm_access(uid, bm_uuid, role, is_active)
VALUES ('FIREBASE_UID_AQUI', 'e312d632-249a-43d1-8957-b5c1bedb9223', 'admin', true)
ON CONFLICT (uid, bm_uuid) DO UPDATE SET is_active = EXCLUDED.is_active, updated_at = now();
```

### 15.4 Testes esperados

1. `/v1/me` com token válido:
- retorna `uid` e `email`.

2. Endpoint de negócio com usuário autorizado para BM:
- `200` e operação normal.

3. Endpoint de negócio com usuário sem vínculo:
- `403` com `forbidden_for_ad_account`.

4. Sem token:
- `401`.

### 15.5 Observações

- `ListClients` e `ListAdAccountsByClient` continuam protegidos por auth, mas sem filtro de tenant por usuário neste patch.
- Próxima melhoria recomendada: filtrar listagens por escopo de BM/usuário.
