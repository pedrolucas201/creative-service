# Relatorio de Avancos - Meta Webhook + Status + UI (2026-03-04)

## Resumo executivo

Foi concluida a base tecnica para status de anuncios via Meta com dois caminhos complementares:

1. Atualizacao por evento (webhook da Meta para `ad_account`).
2. Atualizacao sob demanda (consulta direta no Graph API via endpoints de status).

No frontend, a exibicao de status foi simplificada para usuario final (status principal + motivo), mantendo detalhes tecnicos disponiveis somente quando necessario.

## Entregas no backend

### 1) Webhook da Meta (Ad Account)

- `GET /v1/meta/webhooks/ad-account`
  - validacao do handshake (`hub.mode`, `hub.verify_token`, `hub.challenge`).
- `POST /v1/meta/webhooks/ad-account`
  - validacao de assinatura `X-Hub-Signature-256` (HMAC SHA-256 com app secret).
  - parse do payload `ad_account`.
  - enriquecimento dos dados e persistencia no cache de status.

Arquivos principais:
- `internal/httpapi/handlers_meta_webhook.go`
- `internal/httpapi/router.go`
- `cmd/api/main.go`
- `internal/config/config.go`

### 2) Cache de status por entidade

- Migration criada: `009_entity_status_cache.sql`.
- Tabela `entity_status_cache` para armazenar status consolidado de:
  - `creative`
  - `campaign`
  - `adset`
  - `ad`

Arquivo principal:
- `internal/storage/migrations/009_entity_status_cache.sql`

### 3) Endpoints de status para consumo da UI

- `POST /v1/status/sync`
  - sincroniza status no momento da chamada (sem cron/job).
- `GET /v1/status`
  - retorna status atual do cache (com opcao de refresh conforme implementacao vigente).

Arquivos principais:
- `internal/httpapi/status_view.go`
- `internal/service/status.go`
- `internal/storage/postgres.go`

### 4) Payload enriquecido para UX

Os retornos de status incluem campos adicionais (origem, erro e motivo) para mensagens mais claras no app:

- `status_reason`
- `error_message`
- `error_summary`
- `error_code`
- `webhook_field`
- `webhook_level`
- `graph_status`
- `source_ad_id`, etc.

## Entregas no frontend (impacto funcional)

### 1) Exibicao simplificada de status

Para Campanha, Conjunto, Anuncio e Criativo:

- mostra `Status` principal;
- mostra `Motivo` quando houver;
- mostra `Status atualizado` com data/hora.

Detalhes tecnicos (configurado x efetivo) so aparecem no modal quando ha divergencia real.

### 2) Campos traduzidos e mensagens mais claras

- labels de status em portugues;
- textos de erro orientados ao usuario final;
- diagnostico tecnico permanece no log para debug.

## Decisao de deploy (estado atual)

Frontend em Firebase Hosting foi pausado intencionalmente neste momento para evitar expor a tela de criacao de usuario sem funcionalidade completa em producao.

Motivo:
- endpoint de criacao depende de permissao de IAM na Service Account de runtime (`firebaseauth.admin`) para criar usuario no Firebase Auth via backend.

## Pendencia objetiva

Conceder na SA de runtime:
- `roles/firebaseauth.admin`

Apos isso:
1. validar criacao de usuario fim a fim;
2. liberar deploy final no Hosting.

## Referencias relacionadas

- `docs/RUNBOOK_META_WEBHOOK_STATUS.md`
- `docs/CONTRATO_ERROS_API.md`
- `docs/FAQ_REUNIAO_FIREBASE_SM.md`
