# 🚀 GUIA COMPLETO: MIGRAÇÃO PARA GCP (FAÇA SOZINHO)

## ✅ CÓDIGO MODIFICADO (JÁ ESTÁ PRONTO!)

Arquivos que eu criei/modifiquei:
- ✅ `internal/storage/gcs_client.go` - Cliente GCS (novo)
- ✅ `internal/storage/storage_interface.go` - Interface comum S3/GCS (novo)
- ✅ `internal/config/config.go` - Suporte a GCS (atualizado)
- ✅ `cmd/api/main.go` - Detecta S3 ou GCS automaticamente (atualizado)
- ✅ `internal/service/creatives_sync.go` - Usa interface genérica (atualizado)
- ✅ `go.mod` - Dependências GCP (atualizado)
- ✅ `docker-compose.yml` - Postgres removido (atualizado)

---

## 📋 FASE 1: INSTALAR DEPENDÊNCIAS

### **1. Instalar Google Cloud SDK no Go:**

```powershell
cd C:\Users\PC\Downloads\creative-service

# Baixar dependências novas (GCS)
go get cloud.google.com/go/storage
go get google.golang.org/api

# Atualizar go.mod
go mod tidy

# Verificar se compila
go build ./cmd/api
```

**Esperado:** Compila sem erros ✅

---

## 📋 FASE 2: CONFIGURAR GCP (Console Web)

### **2.1 Criar Projeto GCP:**

1. Acesse: https://console.cloud.google.com
2. **Criar Projeto:**
   - Nome: `creative-service-prod`
   - Localização: Sem organização
3. **Anotar Project ID** (ex: `creative-service-prod-123456`)

---

### **2.2 Habilitar APIs:**

Console → APIs & Services → Enable APIs and Services

Habilitar:
- ✅ Cloud Storage API
- ✅ Cloud SQL Admin API
- ✅ Cloud Run API
- ✅ Artifact Registry API

---

### **2.3 Criar Service Account:**

IAM & Admin → Service Accounts → Create Service Account

```
Service account name: creative-service-sa
Role: 
  - Storage Object Admin
  - Cloud SQL Client
```

**Criar chave JSON:**
1. Service Account criada → Keys → Add Key → Create new key → JSON
2. **Baixar arquivo** (ex: `creative-service-sa-key.json`)
3. **Guardar em local seguro!**

---

### **2.4 Criar Cloud Storage Bucket:**

Cloud Storage → Buckets → Create

```
Name: creative-service-prod-gcs (único globalmente)
Location type: Region
Location: us-east1 (ou outra região)
Storage class: Standard
Access control: Uniform
Public access: Allow public access (pra servir arquivos)

⚠️ Importante: Deixar público!
```

**Configurar permissões públicas:**
```
Bucket → Permissions → Add principal
  Não adicionar allUsers por padrão (deixe privado)
```

---

### **2.5 Criar Cloud SQL (PostgreSQL):**

SQL → Create Instance → Choose PostgreSQL

```
Instance ID: creative-service-db
Password: [ANOTAR! Ex: SenhaForte123!]
Database version: PostgreSQL 16
Configuration: Sandbox (pra teste, barato)
  - Shared core
  - 0.6 GB RAM
  - 10 GB storage

Region: us-east1 (mesma do bucket!)

Connections:
  ✅ Public IP (mais fácil pra começar)
  ✅ Add network: seu-ip-publico/32 (temporário para setup)
  
  ⚠️ Em produção real, limitaria IPs específicos
```

**Vai levar 5-10 minutos pra criar!**

Enquanto isso, próximo passo...

---

### **2.6 Criar Artifact Registry:**

Artifact Registry → Repositories → Create Repository

```
Name: creative-service
Format: Docker
Location: us-east1 (mesma região!)
Encryption: Google-managed
```

---

## 📋 FASE 3: RODAR MIGRATIONS NO CLOUD SQL

### **3.1 Conectar no Cloud SQL:**

**Opção A - Cloud Shell (mais fácil):**
```bash
# No console GCP → Cloud Shell (ícone >_)
gcloud sql connect creative-service-db --user=postgres

# Vai pedir senha (a que você definiu)
```

**Opção B - Local (precisa Cloud SQL Proxy):**
```powershell
# Baixar Cloud SQL Proxy
# https://cloud.google.com/sql/docs/postgres/sql-proxy

# Rodar proxy
cloud-sql-proxy creative-service-prod:us-east1:creative-service-db

# Em outro terminal:
psql "host=127.0.0.1 port=5432 sslmode=disable user=postgres dbname=postgres"
```

---

### **3.2 Criar banco e rodar migrations:**

```sql
-- Criar banco
CREATE DATABASE creatives;

-- Conectar nele
\c creatives

-- Rodar migrations (uma por uma):
```

**Agora copie/cole conteúdo de cada arquivo:**

1. `internal/storage/migrations/001_init.sql`
2. `internal/storage/migrations/002_creatives.sql`
3. `internal/storage/migrations/003_refactor_multi_account.sql`
4. `internal/storage/migrations/004_simplify_ad_account_id.sql`
5. `internal/storage/migrations/005_remove_client_id_from_creatives.sql`

**Verificar:**
```sql
\dt
-- Deve mostrar: campaigns, adsets, ads, creatives, ad_accounts
```

✅ **Migrations OK!**

---

## 📋 FASE 4: TESTAR LOCAL COM GCS

### **4.1 Criar .env.gcp (teste local com GCP):**

```powershell
# C:\Users\PC\Downloads\creative-service\.env.gcp
```

Conteúdo:
```bash
# Storage
STORAGE_PROVIDER=gcs
GCS_BUCKET=creative-service-prod-gcs
GCP_PROJECT_ID=creative-service-prod-123456
GCS_CREDENTIALS_JSON={"type":"service_account",...}
# ↑ Cole TODO o conteúdo do arquivo JSON aqui (em uma linha)

# Banco (Cloud SQL via IP público)
DATABASE_URL=postgres://postgres:SenhaForte123!@34.XXX.XXX.XXX:5432/creatives?sslmode=require
# ↑ Pegar IP no console: SQL → creative-service-db → Overview → Public IP address

# Tokens Meta
TOKEN_FRANCISCO=EAAMcbt...
TOKEN_CONTINGENCIA01=EAAMcbt...

# Meta API
META_BASE_URL=https://graph.facebook.com
META_API_VERSION=v24.0
```

---

### **4.2 Testar localmente:**

```powershell
# Carregar .env.gcp
$env:STORAGE_PROVIDER="gcs"
$env:GCS_BUCKET="creative-service-prod-gcs"
$env:GCP_PROJECT_ID="creative-service-prod-123456"
# ... etc (ou usar start.ps1 modificado)

# Rodar backend
go run ./cmd/api
```

**Logs esperados:**
```
Initializing GCS client...
GCS client initialized for bucket: creative-service-prod-gcs
```

✅ **Se subiu sem erro: PRONTO PRA DEPLOY!**

---

## 📋 FASE 5: BUILD E PUSH DOCKER

### **5.1 Autenticar no Artifact Registry:**

```powershell
# Configurar Docker pra autenticar no GCP
gcloud auth configure-docker us-east1-docker.pkg.dev
```

---

### **5.2 Build da imagem:**

```powershell
cd C:\Users\PC\Downloads\creative-service

# Build (vai demorar 2-5 min)
docker build -t us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest .
```

**Verificar:**
```powershell
docker images | findstr backend
```

---

### **5.3 Push pro Artifact Registry:**

```powershell
docker push us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest
```

**Vai demorar 5-10 minutos** (fazendo upload)

---

## 📋 FASE 6: DEPLOY NO CLOUD RUN

### **6.1 Deploy via gcloud (linha de comando):**

```powershell
gcloud run deploy creative-service-backend `
  --image=us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest `
  --platform=managed `
  --region=us-east1 `
  --allow-unauthenticated `
  --port=8080 `
  --memory=512Mi `
  --cpu=1 `
  --min-instances=0 `
  --max-instances=10 `
  --set-env-vars="^@^STORAGE_PROVIDER=gcs@GCS_BUCKET=creative-service-prod-gcs@GCP_PROJECT_ID=creative-service-prod-123456@TOKEN_FRANCISCO=EAAMcbt...@TOKEN_CONTINGENCIA01=EAAMcbt...@META_BASE_URL=https://graph.facebook.com@META_API_VERSION=v24.0" `
  --set-cloudsql-instances=creative-service-prod:us-east1:creative-service-db
```

⚠️ **ATENÇÃO:** Substitua todas as variáveis!

---

### **6.2 OU deploy via Console (MAIS FÁCIL!):**

Cloud Run → Create Service

**Container:**
```
Container image URL: 
us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest

Container port: 8080
```

**Variables & Secrets → Add Variable:**
```
STORAGE_PROVIDER = gcs
GCS_BUCKET = creative-service-prod-gcs
GCP_PROJECT_ID = creative-service-prod-123456
DATABASE_URL = postgres://postgres:SenhaForte123!@/creatives?host=/cloudsql/creative-service-prod:us-east1:creative-service-db
TOKEN_FRANCISCO = EAAMcbt...
TOKEN_CONTINGENCIA01 = EAAMcbt...
META_BASE_URL = https://graph.facebook.com
META_API_VERSION = v24.0
```

**Connections → Cloud SQL:**
```
✅ Add connection
Instance: creative-service-prod:us-east1:creative-service-db
```

**Security → Service account:**
```
Service account: creative-service-sa@...iam.gserviceaccount.com
```

**Authentication:**
```
Allow unauthenticated invocations
```

**CREATE!**

Vai levar 2-3 minutos.

---

### **6.3 Pegar URL do Cloud Run:**

Depois do deploy:
```
Service URL: https://creative-service-backend-xxxxx-ue.a.run.app
```

**ANOTE ESSA URL!**

---

## 📋 FASE 7: TESTAR PRODUÇÃO

### **7.1 Testar API:**

```powershell
# Listar campanhas (deve retornar [] ou dados)
curl https://creative-service-backend-xxxxx-ue.a.run.app/v1/health

# Health check (se tiver)
curl https://creative-service-backend-xxxxx-ue.a.run.app/health
```

---

### **7.2 Ver logs:**

Cloud Run → creative-service-backend → Logs

Procurar por:
- ✅ "GCS client initialized"
- ✅ "Server listening on :8080"
- ❌ Erros (se tiver)

---

## 📋 FASE 8: ATUALIZAR FLUTTER

### **8.1 Modificar URL:**

```dart
// C:\Users\PC\StudioProjects\untitled\lib\config\api_config.dart

class ApiConfig {
  static const String baseUrl = 'https://creative-service-backend-xxxxx-ue.a.run.app';
  // ↑ Sua URL do Cloud Run
}
```

---

### **8.2 Rebuild Flutter:**

```powershell
cd C:\Users\PC\StudioProjects\untitled

flutter build windows --release
```

**Output:** `build\windows\x64\runner\Release\`

---

### **8.3 Testar:**

1. Abrir `untitled.exe`
2. Listar campanhas
3. Criar criativo
4. Verificar se arquivo aparece no GCS bucket

✅ **SUCESSO!**

---

## 🎯 CHECKLIST FINAL

- [ ] Código modificado (eu fiz!)
- [ ] Dependências instaladas (`go mod tidy`)
- [ ] Projeto GCP criado
- [ ] APIs habilitadas
- [ ] Service account criado (JSON baixado)
- [ ] Cloud Storage bucket criado (público)
- [ ] Cloud SQL criado
- [ ] Migrations rodadas no Cloud SQL
- [ ] Artifact Registry criado
- [ ] Imagem Docker buildada
- [ ] Imagem enviada pro Artifact Registry
- [ ] Cloud Run deployado
- [ ] Variáveis de ambiente configuradas
- [ ] Cloud SQL conectado
- [ ] API testada (curl)
- [ ] Flutter atualizado (URL)
- [ ] Flutter rebuildado
- [ ] Testado end-to-end

---

## 💰 CUSTO MENSAL ESTIMADO

```
Cloud Run:              $0-5/mês (pay-per-use, primeiros 2M requests grátis)
Cloud SQL (Sandbox):    $7-10/mês
Cloud Storage:          $1-3/mês (50GB = ~$1)
Artifact Registry:      $0.10/mês (storage)
Networking:             $1-5/mês

TOTAL: ~$10-25/mês
```

Muito mais barato que AWS! 🎉

---

## 🚨 PROBLEMAS COMUNS

### ❌ "Failed to create GCS client"
**Causa:** Credenciais erradas ou permissões faltando  
**Solução:** Verificar service account tem roles corretas

### ❌ "Connection refused" (Cloud SQL)
**Causa:** Cloud SQL não conectado ao Cloud Run  
**Solução:** Adicionar connection no Cloud Run config

### ❌ "Permission denied" (GCS upload)
**Causa:** Service account sem permissão  
**Solução:** IAM → creative-service-sa → Add role "Storage Object Admin"

### ❌ Container crash após 30s
**Causa:** Variável faltando (provavelmente DATABASE_URL)  
**Solução:** Cloud Run → Edit & Deploy → Variables → Conferir todas

---

## 📞 ORDEM RECOMENDADA DE EXECUÇÃO

**DIA 1 (Preparação - 2h):**
1. ✅ Instalar dependências (`go mod tidy`)
2. ✅ Criar projeto GCP
3. ✅ Habilitar APIs
4. ✅ Criar service account
5. ✅ Criar Cloud Storage
6. ✅ Criar Cloud SQL (vai demorar!)

**DIA 2 (Deploy - 3h):**
1. ✅ Rodar migrations
2. ✅ Testar local com GCS
3. ✅ Build Docker
4. ✅ Push Artifact Registry
5. ✅ Deploy Cloud Run
6. ✅ Testar produção
7. ✅ Atualizar Flutter
8. ✅ Testar end-to-end

---

## 🎉 PRONTO!

Depois de tudo funcionando:
- ✅ Backend: Cloud Run (serverless, escalável)
- ✅ Banco: Cloud SQL (gerenciado, backups automáticos)
- ✅ Storage: Cloud Storage (arquivos públicos)
- ✅ Flutter: Conecta na URL de prod

**VOCÊ CONSEGUIU! 🚀**
