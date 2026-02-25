# Explicação da Arquitetura - Creative Service

## 🎯 Contexto e Problema

O projeto resolve um problema real: **criar anúncios no Facebook/Instagram Ads de forma automatizada** através da API da Meta (Facebook Marketing API).

### O Desafio
A API da Meta tem comportamentos diferentes para imagens e vídeos:
- **Imagens**: Upload rápido (segundos)
- **Vídeos**: Upload lento (pode levar minutos), com processamento assíncrono no lado da Meta

Além disso, o sistema oferece **integração completa** com a hierarquia de anúncios da Meta:
1. **Campaign** (Campanha) - Define objetivo e orçamento geral
2. **AdSet** (Conjunto de Anúncios) - Define segmentação e cronograma
3. **Creative** (Criativo) - O conteúdo visual (imagem/vídeo)
4. **Ad** (Anúncio) - Conecta AdSet + Creative

Precisamos também gerenciar múltiplos clientes, cada um com suas credenciais e contas de anúncio diferentes.

---

## 🏗️ Decisões Arquiteturais Principais

### 1. **Por que Dividir em API + Worker?**

**Problema**: Se eu fizer upload de vídeo de forma síncrona (esperar o upload completo), a requisição HTTP pode demorar 2-5 minutos. Isso causa:
- Timeout de conexão
- Cliente HTTP esperando muito tempo
- Servidor bloqueado sem poder atender outras requisições

**Solução**: Arquitetura de **dois processos independentes**:

```
Cliente → API (responde rápido) → Redis (fila)
                                      ↓
                                   Worker (processa devagar)
```

**API**: Recebe requisições, valida, salva no banco, enfileira e **responde imediatamente** com um `job_id`

**Worker**: Fica em loop infinito pegando jobs da fila e processando sem pressa

**Vantagem**: A API nunca trava. O cliente pode consultar o status do job depois.

---

### 2. **Por que Imagem é Síncrono e Vídeo é Assíncrono?**

**Imagens**:
- Upload leva ~2-5 segundos
- Usuário pode esperar esse tempo
- Resposta imediata é melhor UX
- Endpoint: `POST /v1/creatives/image` → retorna `creative_id` na hora

**Vídeos**:
- Upload pode levar 1-5 minutos (arquivos grandes)
- Não faz sentido cliente esperar tanto tempo
- Endpoint: `POST /v1/jobs/creatives/video` → retorna `job_id`
- Cliente consulta depois: `GET /v1/jobs/{job_id}` → status + resultado

**Analogia**: É como pedir comida. Sanduíche (imagem) você pega na hora. Pizza (vídeo) te dão um número e você busca depois.

---

### 3. **Mapeamento de Clientes via Database**

**Problema**: Cada cliente precisa de:
- `ad_account_id` (conta de anúncios)
- `page_id` (página do Facebook)
- `token` (credencial de acesso)

**Solução**: Tabela `clients` no PostgreSQL

```sql
client_id → ad_account_id, page_id, token_ref
```

**Fluxo**:
1. Cliente envia apenas `client_id` na requisição
2. Sistema busca no banco os dados completos
3. Token é resolvido via variável de ambiente (`token_ref = "ENV:TOKEN_FRANCISCO"`)

**Vantagens**:
- Cliente não precisa enviar credenciais toda hora
- Centralizado: mudou token? Atualiza só no servidor
- Segurança: tokens não trafegam nas requisições

---

### 4. **Por que PostgreSQL?**

Precisamos de:
- ✅ Relacionamento entre `clients` e `jobs` (foreign key)
- ✅ Transações ACID (garantir consistência)
- ✅ Queries complexas (filtrar jobs por cliente, status)
- ✅ Tipos ENUM (`job_status`: queued, running, succeeded, failed)
- ✅ JSONB para dados flexíveis (`input_json`, `result_json`)

**Redis não serve** porque é key-value simples, sem relacionamentos.

---

### 5. **Por que Redis como Fila?**

**Alternativas consideradas**:
- ❌ Polling no banco: Worker fazendo `SELECT * FROM jobs WHERE status='queued'` a cada 5s → ineficiente
- ❌ RabbitMQ/Kafka: Overhead para projeto pequeno

**Redis com LPUSH/BRPOP**:
- ✅ Simples: 2 comandos apenas
- ✅ `BRPOP`: Blocking, não desperdiça CPU (espera até ter job)
- ✅ Rápido: in-memory
- ✅ Já usado por muitos projetos Go

**Limitação (MVP)**: Sem ACK. Se worker crashar no meio, job se perde. Para produção, evoluir para `RPOPLPUSH` (move para lista de processamento).

---

### 6. **Semáforo para Controle de Concorrência**

**Problema**: Meta API tem rate limits. Se eu enviar 50 uploads simultâneos, API retorna erro 429 (too many requests).

**Solução**: Semáforo customizado (channel com buffer)

```go
type Semaphore struct { ch chan struct{} }

// API: max 6 uploads simultâneos
sem := NewSemaphore(6)

// Worker: max 3 uploads simultâneos  
sem := NewSemaphore(3)
```

**Como funciona**:
1. Antes de chamar Meta API: `sem.Acquire()` (bloqueia se já atingiu limite)
2. Depois de completar: `sem.Release()` (libera uma vaga)

**Analogia**: Fila de banco com 6 caixas. Se todos cheios, você espera. Terminou um atendimento? Próximo da fila entra.

---

### 7. **Armazenamento de Arquivos (Blob Store)**

**Problema**: Vídeos são grandes (50-500MB). Onde guardar enquanto o worker não processa?

**Opções**:
- ❌ Base64 no banco: Explode o tamanho do DB, lento
- ❌ Manter em memória: Worker pode estar em outra máquina

**Solução**: Sistema de arquivos local (`/data/blob`)

```
/data/blob/jobs/{job_id}/video.mp4
/data/blob/jobs/{job_id}/thumb.png
```

**Fluxo**:
1. API recebe upload → salva em `/data/blob`
2. Salva caminho no banco (`blob_video_path`)
3. Worker lê do disco → faz upload pra Meta → deleta (implícito)

**Docker Volume**: `blobdata:/data/blob` compartilhado entre API e Worker

**Evolução futura**: Trocar por S3/MinIO para cloud.

---

### 8. **Organização do Código (Clean Architecture Lite)**

```
cmd/               → Entrypoints (main.go da API e Worker)
internal/
  ├── config/      → Carrega variáveis de ambiente
  ├── storage/     → Camada de banco (PostgreSQL)
  ├── blob/        → Camada de arquivos
  ├── queue/       → Camada de fila (Redis)
  ├── meta/        → Client HTTP para Meta API
  ├── secrets/     → Resolve tokens (ENV:TOKEN_X)
  ├── service/     → Lógica de negócio (regras)
  └── httpapi/     → Handlers HTTP (chi router)
```

**Princípios**:
- **Separação de responsabilidades**: Cada pacote tem um propósito único
- **Testabilidade**: `service/` não depende de HTTP, posso testar sem servidor
- **Reutilização**: `meta.Client` é usado por API e Worker
- **Inversão de dependência**: Service recebe interfaces (`blob.Store`, não `blob.LocalFS`)

---

### 9. **Client HTTP Resiliente para Meta API**

**Problema**: Meta API pode dar erro temporário (500, 429)

**Solução**: Retry com exponential backoff

```go
MaxRetries: 5
Backoff: 250ms → 500ms → 1s → 2s → 4s → 8s (max)
```

**Lógica**:
- Status 429 (rate limit) ou 5xx → Retry
- Status 4xx (erro do cliente) → Falha imediata (não adianta tentar de novo)

**Vantagem**: Sistema tolerante a falhas transitórias da Meta.

---

### 10. **Por que Go?**

**Alternativas**: Python, Node.js, Java

**Escolhi Go porque**:
- ✅ Concorrência nativa (goroutines): Worker processa vários jobs em paralelo facilmente
- ✅ Binário único: `docker build` gera executável standalone (sem deps)
- ✅ Performance: HTTP server eficiente, baixo uso de memória
- ✅ Type-safe: Menos bugs em produção vs Python/JS
- ✅ Ecossistema: chi (router), pgx (postgres), go-redis

---

## 🔄 Fluxo Completo de um Vídeo Creative

```
1. Cliente POST /v1/jobs/creatives/video
   ├─ Multipart: client_id, video (MP4), thumbnail (PNG), metadados
   │
2. API (VideoJobService)
   ├─ Valida campos obrigatórios
   ├─ Salva vídeo e thumb em /data/blob/jobs/{uuid}/
   ├─ Gera job_id (UUID)
   ├─ INSERT no PostgreSQL (status='queued')
   ├─ LPUSH no Redis (enfileira job_id)
   └─ Responde 202 Accepted { "job_id": "..." }
   │
3. Worker (em loop)
   ├─ BRPOP do Redis (espera até ter job)
   ├─ UPDATE jobs SET status='running'
   ├─ Busca job no banco
   ├─ Resolve client_id → ad_account_id, page_id, token
   ├─ Lê arquivos de /data/blob
   ├─ Semaphore.Acquire() (espera vaga)
   ├─ Upload vídeo para Meta API (pode levar 2-5min)
   ├─ Upload thumbnail para Meta API
   ├─ Cria AdCreative na Meta API
   ├─ Valida creative (GET para confirmar)
   ├─ UPDATE jobs SET status='succeeded', result_json={...}
   └─ Semaphore.Release()
   │
4. Cliente consulta GET /v1/jobs/{job_id}
   └─ Retorna status + resultado ou erro
```

---

## 🛡️ Decisões de Segurança e Observabilidade

### Segurança
- **Tokens em ENV vars**: Não commitados no código
- **Token ref indireto**: Banco guarda `ENV:TOKEN_X`, não o token real
- **No credentials in logs**: Middleware não loga tokens

### Observabilidade
- **Middleware de log**: Registra método, path, status, latência
- **Panic recovery**: Se handler crashar, retorna 500 sem derrubar servidor
- **Job tracking**: Status completo no banco (queued → running → succeeded/failed)
- **Error messages**: Armazenados em `jobs.error_text` para debug

---

## 📊 Capacidade e Limites

### API
- **Concorrência**: 6 uploads simultâneos (configurável: `MAX_CONCURRENCY=6`)
- **Timeout HTTP**: 45s (uploads de imagem pequenos)

### Worker  
- **Concorrência**: 3 uploads simultâneos (menor porque vídeos são pesados)
- **Timeout HTTP**: 45s por requisição (mas upload de vídeo pode ser chunked pela Meta)

### Escalabilidade
- **Horizontal**: Posso subir vários Workers (compartilham mesma fila Redis)
- **Vertical**: Aumentar `MAX_CONCURRENCY` (respeitando rate limits da Meta)

---

## 🚀 Deploy e Infraestrutura

### Docker Compose
```yaml
services:
  api:      porta 8080, max_concurrency=6
  worker:   background, max_concurrency=3
  postgres: porta 5433 (dados)
  redis:    porta 6379 (fila)
```

**Volume compartilhado**: `blobdata` entre API e Worker

### Variáveis Críticas
```bash
DATABASE_URL=postgres://...
REDIS_ADDR=redis:6379
TOKEN_FRANCISCO=EAAB...  # Token da Meta para cliente 'francisco'
```

---

## 🎓 Lições e Trade-offs

### ✅ O que funcionou bem
1. **Separação API/Worker**: Cada um escala independente
2. **Semáforo**: Simples e eficaz para rate limiting
3. **Redis LPUSH/BRPOP**: Solução minimalista que funciona
4. **Go channels**: Concorrência sem complexidade

### ⚠️ Limitações (MVP)
1. **Redis sem ACK**: Job pode se perder se worker crashar
   - **Solução futura**: `RPOPLPUSH` + lista de processamento
2. **Blob local**: Não funciona em cluster multi-node
   - **Solução futura**: S3 / MinIO / GCS
3. **Sem metrics**: Prometheus seria bom para produção
4. **Sem dead-letter queue**: Jobs falhados ficam no banco, mas não reprocessam

### 🔮 Evolução Futura
1. **Retry automático**: Jobs falhados voltam pra fila (max 3 tentativas)
2. **Webhook callback**: Notificar cliente quando job completa
3. **Batch processing**: Criar múltiplos creatives de uma vez
4. **Cache de clients**: Evitar lookup no banco toda hora (Redis cache)

---

## 💡 Por que essa arquitetura é boa?

### 1. **Simples mas Profissional**
- Não é over-engineered (não usei Kafka para problema simples)
- Mas tem patterns de produção (retry, semaphore, async processing)

### 2. **Manutenível**
- Código organizado (clean architecture)
- Cada camada testável isoladamente
- Fácil adicionar novos tipos de creative (carousel, stories)

### 3. **Escalável**
- Workers horizontais (adiciono mais containers)
- Rate limiting por semáforo (respeita limites da Meta)

### 4. **Resiliente**
- Retry automático na Meta API
- Panic recovery
- Jobs trackáveis (não se perde histórico)

### 5. **Extensível**
- Adicionar novo endpoint? Crio handler + service
- Nova API além da Meta? Copio pacote `meta/` e adapto
- Trocar PostgreSQL por MySQL? Só mudo `storage/`

---

## 🔗 Fluxo Completo: Da Campanha ao Anúncio Publicado

O sistema oferece **dois níveis de integração**:

### Nível 1: Apenas Creatives (foco inicial)
Cliente cria campanhas/adsets manualmente na interface da Meta, e usa nossa API apenas para criar os creatives (parte visual).

### Nível 2: Automação Completa (endpoints adicionais)
Cliente pode criar **toda a estrutura programaticamente**:

```
1. POST /v1/campaigns
   ↓ retorna campaign_id
   
2. POST /v1/adsets  
   ↓ retorna adset_id (vinculado à campaign)
   
3. POST /v1/creatives/image ou /v1/jobs/creatives/video
   ↓ retorna creative_id
   
4. POST /v1/ads
   ↓ conecta adset + creative, retorna ad_id
   
5. Anúncio publicado no Facebook/Instagram! 🎉
```

### Endpoints da Estrutura Completa

**POST /v1/campaigns**
```json
{
  "client_id": "francisco",
  "name": "Black Friday 2024",
  "objective": "OUTCOME_TRAFFIC",
  "status": "PAUSED",
  "special_ad_categories": ["NONE"]
}
```
**Retorna**: `{"campaign_id": "123456"}`

---

**POST /v1/adsets**
```json
{
  "client_id": "francisco",
  "campaign_id": "123456",
  "name": "Público 18-35 SP",
  "billing_event": "IMPRESSIONS",
  "optimization_goal": "REACH",
  "bid_amount": 500,
  "daily_budget": 5000,
  "targeting": {
    "geo_locations": {"countries": ["BR"]},
    "age_min": 18,
    "age_max": 35
  },
  "status": "PAUSED"
}
```
**Retorna**: `{"adset_id": "789012"}`

---

**POST /v1/ads**
```json
{
  "client_id": "francisco",
  "adset_id": "789012",
  "creative_id": "345678",
  "name": "Anúncio Produto X",
  "status": "PAUSED"
}
```
**Retorna**: `{"ad_id": "111213"}`

### Por que todos começam com status PAUSED?

**Segurança**: Criar anúncio com `status: "ACTIVE"` já inicia cobrança imediata. O padrão seguro é criar tudo `PAUSED` e ativar manualmente após revisão.

### Implementação dos Endpoints

Cada endpoint segue o mesmo padrão arquitetural:

1. **Handler HTTP** (`internal/httpapi/handlers.go`) - Parseia JSON, valida input
2. **Service** (`internal/service/campaigns.go`, etc.) - Lógica de negócio
3. **Meta Client** (`internal/meta/client.go`) - Faz a requisição HTTP para a Meta API
4. **Semáforo** - Controla concorrência para respeitar rate limits

Todos os services (Campaign, AdSet, Ad) compartilham a mesma estrutura do Creative Service: resolvem `client_id` → credenciais, usam semáforo, e delegam para o Meta Client com retry.

---

## 🔗 Fluxo Completo: Da Campanha ao Anúncio Publicado

O sistema oferece **dois níveis de integração**:

### Nível 1: Apenas Creatives (foco atual)
Cliente cria campanhas/adsets manualmente na interface da Meta, e usa nossa API apenas para criar os creatives (parte visual).

### Nível 2: Automação Completa (endpoints adicionais)
Cliente pode criar **toda a estrutura programaticamente**:

```
1. POST /v1/campaigns
   ↓ retorna campaign_id
   
2. POST /v1/adsets  
   ↓ retorna adset_id (vinculado à campaign)
   
3. POST /v1/creatives/image ou /v1/jobs/creatives/video
   ↓ retorna creative_id
   
4. POST /v1/ads
   ↓ conecta adset + creative, retorna ad_id
   
5. Anúncio publicado no Facebook/Instagram! 🎉
```

### Endpoints da Estrutura Completa

**POST /v1/campaigns**
```json
{
  "client_id": "francisco",
  "name": "Black Friday 2024",
  "objective": "OUTCOME_TRAFFIC",
  "status": "PAUSED",
  "special_ad_categories": ["NONE"]
}
```
**Retorna**: `{"campaign_id": "123456"}`

---

**POST /v1/adsets**
```json
{
  "client_id": "francisco",
  "campaign_id": "123456",
  "name": "Público 18-35 SP",
  "billing_event": "IMPRESSIONS",
  "optimization_goal": "REACH",
  "bid_amount": 500,
  "daily_budget": 5000,
  "targeting": {
    "geo_locations": {"countries": ["BR"]},
    "age_min": 18,
    "age_max": 35
  },
  "status": "PAUSED"
}
```
**Retorna**: `{"adset_id": "789012"}`

---

**POST /v1/ads**
```json
{
  "client_id": "francisco",
  "adset_id": "789012",
  "creative_id": "345678",
  "name": "Anúncio Produto X",
  "status": "PAUSED"
}
```
**Retorna**: `{"ad_id": "111213"}`

### Por que todos começam com status PAUSED?

**Segurança**: Criar anúncio com `status: "ACTIVE"` já inicia cobrança imediata. O padrão seguro é criar tudo `PAUSED` e ativar manualmente após revisão.

---

## 📝 Resumo Executivo (para seu chefe)

**Problema**: Integração com Meta Ads API para criar anúncios automaticamente.

**Desafio**: Vídeos demoram muito (2-5min), imagens são rápidas (2-5s).

**Solução**: 
- **API síncrona** para imagens (responde na hora)
- **API + Worker assíncrono** para vídeos (retorna job_id, processa depois)
- **Integração completa**: Campaigns → AdSets → Creatives → Ads (toda hierarquia da Meta)
- **PostgreSQL** para dados relacionais (clientes, jobs)
- **Redis** para fila de trabalho (simples e eficiente)
- **Semáforo** para respeitar rate limits da Meta
- **Go** para performance e concorrência nativa

**Resultado**: Sistema rápido, escalável e profissional, com automação completa do fluxo de anúncios da Meta.

**Métricas**:
- Latência da API: <100ms (exceto uploads)
- Throughput: 6 imagens ou 3 vídeos simultâneos (configurável)
- Resiliência: Retry automático em 429/5xx da Meta
- Observabilidade: Logs estruturados + tracking de jobs
