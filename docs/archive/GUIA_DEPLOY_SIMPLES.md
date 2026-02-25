# 🚀 GUIA SIMPLIFICADO - DEPLOY GCP (PEDIDO DO CHEFE)

## 📋 O QUE VAMOS FAZER

1. ✅ Subir Docker com backend no GCP Cloud Run
2. ✅ Criar Cloud Storage (arquivos)
3. ✅ Subir PostgreSQL no Cloud SQL
4. ✅ Atualizar Flutter para apontar pro novo servidor

**Tempo total:** 2-3 horas

---

## 🎯 PASSO 1: CRIAR PROJETO GCP (5 min)

### 1.1 Acesse: https://console.cloud.google.com

### 1.2 Criar Novo Projeto
- Clique no seletor de projetos (topo) → **New Project**
- Nome: `creative-service`
- Clique **Create**
- **ANOTE O PROJECT ID** (ex: `creative-service-123456`)

### 1.3 Habilitar APIs
- Menu ☰ → **APIs & Services** → **Library**
- Pesquise e habilite (clique ENABLE):
  - `Cloud Run API`
  - `Cloud SQL Admin API`
  - `Cloud Storage API`
  - `Artifact Registry API`

### 1.4 Criar Service Account (credenciais)
- Menu → **IAM & Admin** → **Service Accounts**
- **Create Service Account**
- Nome: `creative-service`
- Grant roles:
  - `Cloud SQL Client`
  - `Storage Object Admin`
- **Create and Continue** → **Done**

`Obs:` para o serviço rodando no Cloud Run, prefira anexar essa service account ao serviço e **não** usar chave JSON.

---

## 🪣 PASSO 2: CRIAR CLOUD STORAGE (10 min)

### 2.1 Criar Bucket
- Menu → **Cloud Storage** → **Buckets** → **Create**
- Nome: `creative-service-storage` (pode mudar se já existir)
- Location type: **Region**
- Location: **us-east1** (ou sua preferência)
- Mantenha "Enforce public access prevention" habilitado (recomendado)
- **Create**

### 2.2 Permissões do bucket
- Não adicionar `allUsers` por padrão
- O backend grava/lê com a service account do Cloud Run

### 2.3 Anotar Nome
```
BUCKET: creative-service-storage
```

---

## 🗄️ PASSO 3: CRIAR CLOUD SQL (PostgreSQL) (15 min)

### 3.1 Criar Instância
- Menu → **SQL** → **Create Instance**
- Escolha: **PostgreSQL**
- Instance ID: `creative-db`
- Password: **[CRIE E ANOTE UMA SENHA FORTE]**
- Database version: **PostgreSQL 16**
- Preset: **Sandbox** (mais barato)
- Region: **us-east1** (mesma do bucket!)

### 3.2 Configurar Conexões
- **Show Configuration Options** → **Connections**
- **Public IP**: ✅ Marque
- **Authorized networks**: Add network
  - Name: `local-dev-temporario`
  - Network: `SEU_IP/32`
- **Create** (vai demorar 5-10 minutos!)

### 3.3 Pegar IP Público
Após criar:
- Clique na instância → **Overview**
- **ANOTE O PUBLIC IP ADDRESS** (ex: `34.123.45.67`)

### 3.4 Rodar Migrations
**Opção A - Cloud Shell (mais fácil):**
- No console, clique no ícone **Cloud Shell** `>_` (topo direito)

```bash
# Conectar
gcloud sql connect creative-db --user=postgres

# Digite a senha que você criou

# Criar banco
CREATE DATABASE creatives;

# Conectar no banco
\c creatives

# Agora copie e cole cada migration (na ordem):
```

**Cole uma por vez** (do seu Windows, abra cada arquivo em `internal\storage\migrations\`):
1. `001_init.sql`
2. `002_creatives.sql`
3. `003_refactor_multi_account.sql`
4. `004_simplify_ad_account_id.sql`
5. `005_remove_client_id_from_creatives.sql`

Verifique:
```sql
\dt
-- Deve mostrar: clients, ad_accounts, creatives
```

Saia: `\q`

---

## 🐳 PASSO 4: SUBIR DOCKER NO CLOUD RUN (30 min)

### 4.1 Preparar no seu PC (Windows)

**Instalar Google Cloud SDK:**
- Baixe: https://cloud.google.com/sdk/docs/install
- Execute o instalador
- Abra **novo** PowerShell (depois da instalação)

**Autenticar:**
```powershell
gcloud auth login
# Vai abrir navegador, faça login

gcloud config set project creative-service-123456
# ↑ Use seu PROJECT ID
```

**Configurar Docker:**
```powershell
gcloud auth configure-docker us-east1-docker.pkg.dev
```

### 4.2 Criar Artifact Registry
```powershell
gcloud artifacts repositories create docker-repo `
  --repository-format=docker `
  --location=us-east1 `
  --description="Docker images"
```

### 4.3 Build da Imagem

```powershell
cd C:\Users\PC\Downloads\creative-service

# Build (vai demorar 3-5 min)
docker build -t us-east1-docker.pkg.dev/creative-service-123456/docker-repo/backend:v1 .
```

### 4.4 Push para GCP

```powershell
# Push (vai demorar 5-10 min)
docker push us-east1-docker.pkg.dev/creative-service-123456/docker-repo/backend:v1
```

### 4.5 Deploy no Cloud Run (CONSOLE)

**Via Console (mais fácil):**

1. Menu → **Cloud Run** → **Create Service**

2. **Container:**
   - Container image URL: **SELECT** → Artifact Registry
   - Escolha sua imagem: `backend:v1`
   - Container port: `8080`

3. **Service name:** `creative-backend`

4. **Region:** `us-east1`

5. **CPU allocation:** CPU is only allocated during request processing

6. **Autoscaling:**
   - Minimum: `0`
   - Maximum: `10`

7. **Authentication:** ✅ **Allow unauthenticated invocations**

8. **Container(s), Volumes, Networking, Security:**
   - Clique para expandir

9. **Variables & Secrets** → ADD VARIABLE (para cada):

```
Nome                    | Valor
------------------------|----------------------------------------------
STORAGE_PROVIDER        | gcs
GCS_BUCKET              | creative-service-storage
GCP_PROJECT_ID          | creative-service-123456 (seu project ID)
DATABASE_URL            | postgres://postgres:SUA_SENHA@/creatives?host=/cloudsql/creative-service-123456:us-east1:creative-db
TOKEN_FRANCISCO         | [seu token Meta]
TOKEN_CONTIGENCIA01     | [seu token Meta]
META_BASE_URL           | https://graph.facebook.com
META_API_VERSION        | v24.0
MAX_CONCURRENCY         | 6
HTTP_TIMEOUT            | 45s
RUN_MIGRATIONS          | false
```

10. **Cloud SQL Connections:**
    - **Add Connection**
    - Selecione: `creative-service-123456:us-east1:creative-db`

11. **Service Account:**
    - Selecione: `creative-service@creative-service-123456.iam.gserviceaccount.com`

12. **Clique CREATE!**

Vai levar 2-3 minutos...

---

## ✅ PASSO 5: TESTAR (5 min)

### 5.1 Pegar URL do Serviço

Após deploy concluir:
- Cloud Run → `creative-backend`
- Copie a **URL** (ex: `https://creative-backend-xxxxx-ue.a.run.app`)

### 5.2 Testar API

```powershell
# Substituir pela sua URL
curl https://creative-backend-xxxxx-ue.a.run.app/v1/health
```

**Esperado:** `{"ok":true}` → **SUCESSO!** ✅

Se der erro, veja **Logs**:
- Cloud Run → creative-backend → **Logs**
- Procure por erros vermelhos

---

## 📱 PASSO 6: ATUALIZAR FLUTTER (10 min)

### 6.1 Modificar URL da API

**Arquivo:** `C:\Users\PC\StudioProjects\flutter\lib\config\api_config.dart`

**ANTES:**
```dart
class ApiConfig {
  static const String baseUrl = 'http://localhost:8080';
}
```

**DEPOIS:**
```dart
class ApiConfig {
  static const String baseUrl = 'https://creative-backend-xxxxx-ue.a.run.app';
  // ↑ Cole sua URL do Cloud Run (sem / no final)
}
```

### 6.2 Rebuild Flutter

```powershell
cd C:\Users\PC\StudioProjects\flutter

flutter build windows --release
```

### 6.3 Testar Aplicação

1. Abra o `.exe` em `build\windows\x64\runner\Release\`
2. Tente listar campanhas
3. Tente criar um creative
4. Verifique se funciona! ✅

---

## 🎉 PRONTO!

Arquitetura final:

```
Flutter App (Desktop)
        ↓
Cloud Run (Backend Docker)
    ↙      ↓      ↘
Meta API  Cloud SQL  Cloud Storage
          (Postgres) (Arquivos)
```

---

## 🚨 PROBLEMAS COMUNS

### ❌ Erro ao fazer build Docker
**Solução:** Certifique-se que Docker Desktop está rodando

### ❌ "Permission denied" ao fazer push
**Solução:** Rode novamente:
```powershell
gcloud auth configure-docker us-east1-docker.pkg.dev
```

### ❌ Container não inicia (Cloud Run)
**Solução:** 
1. Cloud Run → Logs
2. Procure erro vermelho
3. 90% das vezes: variável de ambiente errada

### ❌ "Connection refused" no banco
**Solução:**
1. Cloud Run → Edit & Deploy new revision
2. Connections → Verificar se Cloud SQL está selecionado
3. DATABASE_URL está correto?

### ❌ "Permission denied" no Storage
**Solução:**
1. IAM & Admin → Service Accounts
2. Encontre `creative-service`
3. Verificar se tem role "Storage Object Admin"

---

## 💰 CUSTO MENSAL ESTIMADO

```
Cloud Run (backend):     $5-10/mês (pay-per-use)
Cloud SQL (Sandbox):     $7/mês
Cloud Storage:           $1-3/mês
Total:                   ~$15-20/mês
```

**Muito mais barato que manter servidor próprio!**

---

## 📝 CHECKLIST FINAL

- [ ] Projeto GCP criado
- [ ] APIs habilitadas
- [ ] Service Account criado (JSON baixado)
- [ ] Cloud Storage criado (público)
- [ ] Cloud SQL criado
- [ ] Migrations rodadas
- [ ] Artifact Registry criado
- [ ] Docker buildado
- [ ] Docker enviado (push)
- [ ] Cloud Run deployado
- [ ] Variáveis configuradas
- [ ] Cloud SQL conectado
- [ ] API testada (curl)
- [ ] Flutter atualizado
- [ ] App testado end-to-end

---

## 📞 SUPORTE

Se precisar de ajuda:
1. Leia mensagem de erro com atenção
2. Verifique Cloud Run → Logs
3. Confirme todas variáveis de ambiente
4. Google: "Cloud Run [mensagem de erro]"

**Você consegue! 🚀**
