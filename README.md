# Creative Service - Meta Ads API Integration

Serviço de automação para criação de anúncios no Facebook e Instagram através da Meta Marketing API. Oferece integração completa com a hierarquia de anúncios da Meta (Campaigns, AdSets, Creatives, Ads) com processamento síncrono para imagens e assíncrono para vídeos.

## Visão Geral

O Creative Service resolve o desafio de criar anúncios programaticamente na plataforma Meta Ads. Implementa uma arquitetura híbrida que processa uploads de imagens de forma síncrona (resposta imediata) e vídeos de forma assíncrona (job queue com worker), respeitando os diferentes comportamentos e tempos de processamento da API da Meta.

**Documentação Completa:** [Explicação da Arquitetura](explicacao_arquitetura.md)

## Funcionalidades

- **Upload Síncrono de Imagens**: Criação de ad creatives com imagens em segundos
- **Upload Assíncrono de Vídeos**: Sistema de filas para processamento de vídeos longos
- **Gerenciamento de Campanhas**: Criação completa de Campaigns, AdSets e Ads
- **Multi-tenancy**: Suporte para múltiplos clientes com credenciais isoladas
- **Retry Inteligente**: Backoff exponencial para resiliência contra falhas da Meta API
- **Rate Limiting**: Semáforo para controlar concorrência e respeitar limites da Meta

## Tecnologias

- **Go 1.22** - Backend performático com concorrência nativa
- **PostgreSQL** - Armazenamento de clientes e jobs
- **Redis** - Fila de trabalho para processamento assíncrono
- **Docker** - Containerização e orquestração
- **Meta Marketing API v24.0** - Integração com Facebook/Instagram Ads

## Arquitetura

```
┌─────────────┐
│   Cliente   │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────┐
│          API REST (Go)              │
│  ┌──────────────┬─────────────────┐ │
│  │   Imagens    │     Vídeos      │ │
│  │   (Sync)     │    (Async)      │ │
│  └──────┬───────┴────────┬────────┘ │
└─────────┼────────────────┼──────────┘
          │                │
          ▼                ▼
    ┌─────────┐      ┌──────────┐
    │  Meta   │      │  Redis   │
    │   API   │      │  Queue   │
    └─────────┘      └────┬─────┘
                          │
                          ▼
                   ┌─────────────┐
                   │   Worker    │
                   │    (Go)     │
                   └──────┬──────┘
                          │
                          ▼
                     ┌─────────┐
                     │  Meta   │
                     │   API   │
                     └─────────┘
```

## Estrutura do Projeto

```
creative-service/
├── cmd/
│   ├── api/          # Entrypoint da API REST
│   └── worker/       # Entrypoint do Worker assíncrono
├── internal/
│   ├── blob/         # Armazenamento de arquivos
│   ├── config/       # Configuração e variáveis de ambiente
│   ├── httpapi/      # Handlers HTTP e middleware
│   ├── meta/         # Client para Meta Marketing API
│   ├── queue/        # Abstração de fila Redis
│   ├── secrets/      # Resolução de tokens
│   ├── service/      # Lógica de negócio
│   └── storage/      # Camada de persistência PostgreSQL
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

## Endpoints

### Health Check
```
GET /v1/health
```

### Creatives (Conteúdo Visual)

**Criar Creative com Imagem (Síncrono)**
```
POST /v1/creatives/image
Content-Type: multipart/form-data

Parâmetros:
- client_id (string)
- name (string)
- link (string)
- message (string)
- image (file)

Resposta: { "creative_id": "123456789" }
```

**Criar Creative com Vídeo (Assíncrono)**
```
POST /v1/jobs/creatives/video
Content-Type: multipart/form-data

Parâmetros:
- client_id (string)
- name (string)
- link (string)
- message (string)
- video (file)
- thumbnail (file)

Resposta: { "job_id": "uuid-v4" }
```

**Consultar Status de Job**
```
GET /v1/jobs/{job_id}

Resposta: {
  "job_id": "uuid",
  "status": "succeeded|failed|running|queued",
  "result": { "creative_id": "123456789" },
  "error": null
}
```

### Campaign Management (Estrutura Completa)

**Criar Campanha**
```
POST /v1/campaigns
Content-Type: application/json

{
  "client_id": "francisco",
  "name": "Black Friday 2024",
  "objective": "OUTCOME_TRAFFIC",
  "status": "PAUSED",
  "special_ad_categories": ["NONE"]
}

Resposta: { "campaign_id": "123456" }
```

**Criar AdSet (Conjunto de Anúncios)**
```
POST /v1/adsets
Content-Type: application/json

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

Resposta: { "adset_id": "789012" }
```

**Criar Ad (Anúncio Final)**
```
POST /v1/ads
Content-Type: application/json

{
  "client_id": "francisco",
  "adset_id": "789012",
  "creative_id": "345678",
  "name": "Anúncio Produto X",
  "status": "PAUSED"
}

Resposta: { "ad_id": "111213" }
```

## Instalação e Execução

### Pré-requisitos

- Docker & Docker Compose
- Go 1.22+ (para desenvolvimento local)
- PostgreSQL 16 (incluído no docker-compose)
- Redis 7 (incluído no docker-compose)

### Setup Rápido

1. **Clone o repositório**
```bash
git clone https://github.com/seu-usuario/creative-service.git
cd creative-service
```

2. **Configure variáveis de ambiente**
```bash
cp .env.example .env
# Edite .env com seus tokens da Meta
```

3. **Inicie os serviços**
```bash
docker compose up --build
```

4. **Execute as migrations**
```bash
psql 'postgres://postgres:postgres@localhost:5432/creatives?sslmode=disable' \
  -f internal/storage/migrations/001_init.sql
```

5. **Insira um cliente de teste**
```sql
INSERT INTO clients(client_id, ad_account_id, page_id, token_ref)
VALUES ('francisco', 'act_123456789', '987654321', 'ENV:TOKEN_FRANCISCO');
```

A API estará disponível em `http://localhost:8080`

### Desenvolvimento Local (sem Docker)

```bash
# Instalar dependências
go mod download

# Executar API
export DATABASE_URL="postgres://..."
export REDIS_ADDR="localhost:6379"
export TOKEN_FRANCISCO="EAAB..."
go run cmd/api/main.go

# Executar Worker (em outro terminal)
go run cmd/worker/main.go
```

## Configuração

### Variáveis de Ambiente

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `ADDR` | Endereço da API | `:8080` |
| `DATABASE_URL` | Connection string PostgreSQL | - |
| `REDIS_ADDR` | Endereço do Redis | `localhost:6379` |
| `REDIS_QUEUE` | Nome da fila | `creative_jobs` |
| `BLOB_DIR` | Diretório para arquivos temporários | `/data/blob` |
| `MAX_CONCURRENCY` | Uploads simultâneos (API: 6, Worker: 3) | `6` |
| `META_BASE_URL` | URL base da Meta API | `https://graph.facebook.com` |
| `META_API_VERSION` | Versão da API | `v24.0` |
| `TOKEN_*` | Tokens de acesso dos clientes | - |

### Mapeamento de Clientes

O sistema usa a tabela `clients` para mapear `client_id` para credenciais:

```sql
CREATE TABLE clients (
    client_id TEXT PRIMARY KEY,
    ad_account_id TEXT NOT NULL,
    page_id TEXT NOT NULL,
    token_ref TEXT NOT NULL
);
```

O campo `token_ref` usa o formato `ENV:NOME_DA_VARIAVEL` para referenciar tokens armazenados em variáveis de ambiente.

## Fluxo de Trabalho

### Imagens (Sync)
```
1. Cliente envia POST /v1/creatives/image
2. API valida dados e imagem
3. API faz upload para Meta API (2-5 segundos)
4. Retorna creative_id imediatamente
```

### Vídeos (Async)
```
1. Cliente envia POST /v1/jobs/creatives/video
2. API salva vídeo/thumbnail no blob storage
3. API cria job no banco (status: queued)
4. API enfileira job_id no Redis
5. API retorna job_id imediatamente
6. Worker consome job da fila
7. Worker processa upload (2-5 minutos)
8. Worker atualiza status do job (succeeded/failed)
9. Cliente consulta GET /v1/jobs/{job_id}
```

## Decisões de Arquitetura

- **Imagem Síncrona**: Upload rápido permite resposta imediata, melhor UX
- **Vídeo Assíncrono**: Evita timeout HTTP em uploads longos
- **Redis Simples**: LPUSH/BRPOP suficiente para MVP, sem overhead de RabbitMQ/Kafka
- **Semáforo**: Controla concorrência sem bibliotecas externas
- **Blob Storage Local**: Solução MVP, evoluir para S3 em produção
- **PostgreSQL**: Dados relacionais (clients ↔ jobs) e transações ACID

## Limitações (MVP)

- **Fila sem ACK**: Redis LPUSH/BRPOP não garante entrega. Jobs podem se perder se worker crashar. Evoluir para RPOPLPUSH.
- **Blob local**: Não funciona em cluster multi-node. Migrar para S3/MinIO.
- **Sem retry automático**: Jobs falhados não reprocessam. Implementar dead-letter queue.
- **Sem métricas**: Adicionar Prometheus/Grafana para observabilidade.

## Roadmap

- [ ] Implementar RPOPLPUSH para fila confiável
- [ ] Migrar blob storage para S3/MinIO
- [ ] Sistema de retry automático para jobs falhados
- [ ] Webhooks para notificação de conclusão
- [ ] Cache de clientes em Redis
- [ ] Métricas com Prometheus
- [ ] Batch creation de múltiplos creatives
- [ ] Suporte a mais formatos (carousel, stories)

## Troubleshooting

**Worker não processa jobs**
- Verifique se Redis está rodando: `redis-cli ping`
- Verifique logs do worker: `docker compose logs worker`
- Confirme que a fila está configurada: `REDIS_QUEUE=creative_jobs`

**Erro 401 na Meta API**
- Verifique se o token está válido e não expirou
- Confirme que `token_ref` no banco aponta para variável correta
- Teste token manualmente: `curl -X GET "https://graph.facebook.com/v24.0/me?access_token=TOKEN"`

**Upload de vídeo falha**
- Confirme que vídeo atende requisitos da Meta (formato, tamanho, codec)
- Verifique se thumbnail foi enviado (obrigatório para vídeos)
- Aumente `MAX_CONCURRENCY` se houver timeout

**Database connection refused**
- Aguarde PostgreSQL inicializar completamente (~10s após docker compose up)
- Verifique URL: `postgres://user:pass@host:port/dbname?sslmode=disable`

## Contribuindo

1. Fork o projeto
2. Crie uma branch: `git checkout -b feature/nova-funcionalidade`
3. Commit com conventional commits: `git commit -m "feat: adicionar suporte a carousel"`
4. Push: `git push origin feature/nova-funcionalidade`
5. Abra um Pull Request

## Licença

MIT

## Contato

Para dúvidas ou suporte, consulte a [documentação de arquitetura](explicacao_arquitetura.md) ou abra uma issue.
