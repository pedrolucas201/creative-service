# 🚀 MIGRAÇÃO PARA GCP - PLANO COMPLETO

## 📊 ARQUITETURA ATUAL vs NOVA

### **ANTES (Local):**
```
Flutter App (Desktop)
    ↓
Backend Go (go run ./cmd/api)
    ↓
PostgreSQL (Docker container)
    ↓
AWS S3 (creatives/videos)
```

### **DEPOIS (GCP Production):**
```
Flutter App (Desktop)
    ↓
Backend Go (Cloud Run - Docker)
    ↙         ↓         ↘
Meta API  Cloud SQL  Cloud Storage
(Externa) (PostgreSQL) (Arquivos)
```

---

## ✅ CHECKLIST DO CHEFE (5 tarefas)

- [ ] **1. Fazer falar com GCP Cloud Storage** (trocar S3)
- [ ] **2. Fazer falar com GCP Cloud SQL** (trocar PostgreSQL)
- [ ] **3. Remover Postgres do docker-compose**
- [ ] **4. Subir Docker no GCP Cloud Run**
- [ ] **5. Alterar path da API no Flutter App**

---

## 📋 TAREFA 1: MIGRAR S3 → CLOUD STORAGE

### **🔧 Mudanças no código:**

**Arquivo:** `internal/storage/gcs_client.go` (NOVO)

**O que fazer:**
1. Criar novo cliente GCS (igual ao S3 mas pra Google Cloud)
2. Substituir AWS SDK por GCP SDK
3. Manter interface igual (Upload, Download, GetURL)

**Dependências novas:**
```bash
go get cloud.google.com/go/storage
```

**Variáveis de ambiente:**
```bash
# ANTES (AWS):
S3_BUCKET=creative-service-prod
S3_REGION=us-east-2
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...

# DEPOIS (GCP):
GCS_BUCKET=creative-service-prod
GCP_PROJECT_ID=seu-projeto-gcp
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
# OU usar Application Default Credentials no Cloud Run (mais fácil)
```

**Código novo necessário:** ~100 linhas (similar ao s3/client.go)

---

## 📋 TAREFA 2: MIGRAR POSTGRESQL → CLOUD SQL

### **🔧 Mudanças no código:**

**NENHUMA mudança de código! Só mudar variável de ambiente.**

PostgreSQL é PostgreSQL. O driver `pgx` funciona igual.

**DATABASE_URL antiga:**
```
postgres://postgres:postgres@localhost:5433/creatives?sslmode=disable
```

**DATABASE_URL nova (Cloud SQL):**
```
postgres://postgres:SENHA@/creatives?host=/cloudsql/PROJETO:REGIAO:INSTANCIA&sslmode=disable
```

OU (com IP público do Cloud SQL):
```
postgres://postgres:SENHA@IP_PUBLICO:5432/creatives?sslmode=require
```

**Migrations:**
Rodar as mesmas 6 migrations no Cloud SQL antes do deploy.

---

## 📋 TAREFA 3: REMOVER POSTGRES DO DOCKER-COMPOSE

### **🔧 Mudanças no docker-compose.yml:**

**ANTES:**
```yaml
services:
  api:
    build: .
    ports: ["8080:8080"]
    depends_on: [postgres]
    
  postgres:  # ← REMOVER ISSO
    image: postgres:16
    ports: ["5433:5432"]
```

**DEPOIS:**
```yaml
services:
  api:
    build: .
    ports: ["8080:8080"]
    # SEM depends_on
    # SEM service postgres
```

**Motivo:** PostgreSQL agora é Cloud SQL (gerenciado pelo GCP)

---

## 📋 TAREFA 4: SUBIR DOCKER NO CLOUD RUN

### **🚀 Passos:**

**A. Criar projeto no GCP (se não tiver)**
```bash
gcloud projects create creative-service-prod --name="Creative Service"
gcloud config set project creative-service-prod
```

**B. Habilitar APIs necessárias:**
```bash
gcloud services enable run.googleapis.com
gcloud services enable sqladmin.googleapis.com
gcloud services enable storage.googleapis.com
gcloud services enable artifactregistry.googleapis.com
```

**C. Criar Artifact Registry (registry de imagens Docker):**
```bash
gcloud artifacts repositories create creative-service \
  --repository-format=docker \
  --location=us-east1 \
  --description="Creative Service Docker images"
```

**D. Build e push da imagem:**
```bash
# Autenticar
gcloud auth configure-docker us-east1-docker.pkg.dev

# Build
docker build -t us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest .

# Push
docker push us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest
```

**E. Deploy no Cloud Run:**
```bash
gcloud run deploy creative-service-backend \
  --image=us-east1-docker.pkg.dev/creative-service-prod/creative-service/backend:latest \
  --platform=managed \
  --region=us-east1 \
  --allow-unauthenticated \
  --port=8080 \
  --set-env-vars="DATABASE_URL=postgres://...,GCS_BUCKET=...,TOKEN_FRANCISCO=..." \
  --add-cloudsql-instances=PROJETO:REGIAO:INSTANCIA
```

**OU fazer pela interface web (mais fácil):**
1. Cloud Console → Cloud Run → Create Service
2. Container image: [URL da imagem]
3. Variables: Adicionar todas do .env
4. Connections: Cloud SQL instance
5. Deploy

---

## 📋 TAREFA 5: ALTERAR PATH DA API NO FLUTTER

### **🔧 Mudança no Flutter:**

**Arquivo:** `lib/config/api_config.dart`

**ANTES:**
```dart
class ApiConfig {
  static const String baseUrl = 'http://localhost:8080';
}
```

**DEPOIS:**
```dart
class ApiConfig {
  static const String baseUrl = 'https://creative-service-backend-xxxxx-ue.a.run.app';
  // ↑ URL que o Cloud Run vai gerar
}
```

**Rebuild Flutter:**
```bash
cd C:\Users\PC\StudioProjects\flutter
flutter build windows --release
```

**Distribuir novo .exe** para os usuários.

---

## 🎯 ORDEM DE EXECUÇÃO (IMPORTANTE!)

### **Fase 1: Preparação (Você faz antes)**
1. ✅ Modificar código S3 → GCS
2. ✅ Testar localmente (com Cloud Storage de teste)
3. ✅ Remover Postgres do docker-compose
4. ✅ Atualizar Dockerfile se necessário

### **Fase 2: Infraestrutura GCP (Você + Chefe)**
1. ✅ Criar Cloud SQL instance
2. ✅ Criar Cloud Storage bucket
3. ✅ Rodar migrations no Cloud SQL
4. ✅ Configurar service account (permissões)

### **Fase 3: Deploy (Você + Chefe)**
1. ✅ Build da imagem Docker
2. ✅ Push pro Artifact Registry
3. ✅ Deploy no Cloud Run
4. ✅ Configurar variáveis de ambiente
5. ✅ Testar endpoints (curl)

### **Fase 4: Frontend (Você)**
1. ✅ Atualizar URL no Flutter
2. ✅ Rebuild aplicação
3. ✅ Testar tudo funcionando
4. ✅ Distribuir novo .exe

---

## 🔥 DIFERENÇAS IMPORTANTES: AWS vs GCP

| Item | AWS | GCP |
|------|-----|-----|
| **Storage** | S3 | Cloud Storage |
| **Banco** | RDS | Cloud SQL |
| **Container** | ECS/Fargate | Cloud Run |
| **Registry** | ECR | Artifact Registry |
| **CLI** | aws-cli | gcloud |
| **Credenciais** | Access Key + Secret | Service Account JSON |

---

## 💰 CUSTOS ESTIMADOS (GCP)

```
Cloud Run (backend):        $5-15/mês
  - Pay-per-use (só quando recebe request)
  - Primeiros 2M requests grátis

Cloud SQL (PostgreSQL):     $10-25/mês
  - db-f1-micro (shared core, 0.6GB RAM)
  - Pode pausar quando não usar

Cloud Storage:              $1-5/mês
  - $0.020/GB/mês (Standard)
  - 50GB = ~$1

Total estimado:             $15-45/mês

⚠️ Mais barato que AWS porque Cloud Run é serverless!
```

---

## 🚨 PONTOS DE ATENÇÃO

### **1. Service Account (Permissões)**
Cloud Run precisa de permissões para:
- ✅ Cloud SQL (conectar)
- ✅ Cloud Storage (upload/download)
- ❌ Meta API (não precisa, é externa)

### **2. Cloud SQL Proxy**
Duas formas de conectar:
- **A) Unix Socket** (recomendado no Cloud Run)
- **B) IP público** (mais simples pra testar)

### **3. Migrations**
Rodar ANTES do primeiro deploy:
```bash
# Conectar no Cloud SQL
gcloud sql connect INSTANCIA --user=postgres

# Rodar migrations
\i internal/storage/migrations/001_init.sql
# ... etc
```

### **4. Cold Start**
Cloud Run pode demorar 2-5s pra "acordar" se ficar sem requests.
- Solução: Min instances = 1 (mas custa mais)
- Ou aceitar latência inicial

---

## 📞 O QUE VOCÊ PRECISA DO CHEFE

### **Informações necessárias:**
1. ✅ Nome do projeto GCP
2. ✅ Região (us-east1, us-central1, etc)
3. ✅ Credenciais (service account JSON)
4. ✅ Nome da instância Cloud SQL (quando criar)
5. ✅ Nome do bucket Cloud Storage

### **Acessos necessários:**
1. ✅ Cloud Run Admin
2. ✅ Cloud SQL Admin
3. ✅ Storage Admin
4. ✅ Artifact Registry Writer

---

## 🎯 PRÓXIMOS PASSOS IMEDIATOS

### **HOJE (Preparação):**
1. [ ] Criar `internal/storage/gcs_client.go`
2. [ ] Atualizar `go.mod` (adicionar GCP SDK)
3. [ ] Testar Cloud Storage localmente
4. [ ] Remover Postgres do docker-compose.yml

### **AMANHÃ (Com o chefe):**
1. [ ] Criar Cloud SQL instance
2. [ ] Criar Cloud Storage bucket
3. [ ] Rodar migrations
4. [ ] Deploy no Cloud Run
5. [ ] Testar tudo

### **DEPOIS (Finalização):**
1. [ ] Atualizar Flutter
2. [ ] Rebuild .exe
3. [ ] Documentar URLs de produção
4. [ ] Monitorar logs

---

## 📚 DOCUMENTAÇÃO ÚTIL

- Cloud Run: https://cloud.google.com/run/docs
- Cloud SQL: https://cloud.google.com/sql/docs
- Cloud Storage Go: https://cloud.google.com/storage/docs/reference/libraries#client-libraries-install-go

---

**QUER QUE EU COMECE CRIANDO O CÓDIGO DO GCS_CLIENT AGORA?** 🚀
