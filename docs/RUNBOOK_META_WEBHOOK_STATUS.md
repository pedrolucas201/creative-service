# Runbook Meta Webhook + Status no App

## Objetivo

Documentar o estado final da implementacao de status da Meta no projeto:

- webhook de Ad Account
- sincronizacao de status em `entity_status_cache`
- consulta sob demanda de status da conta na Meta
- como isso aparece na UI
- troubleshooting real dos erros que aconteceram

## Escopo implementado

### 1) Webhook Meta (Ad Account)

- Verificacao:
  - `GET /v1/meta/webhooks/ad-account`
- Recebimento:
  - `POST /v1/meta/webhooks/ad-account`
- Validacoes:
  - `hub.verify_token`
  - `X-Hub-Signature-256` (HMAC SHA-256)
- Processamento:
  - interpreta mudancas de `ad_account`
  - resolve `entity_type`/`entity_id`
  - consulta status atual no Graph
  - upsert em `entity_status_cache`

### 2) Snapshot de status para UI

- Endpoints:
  - `GET /v1/status`
  - `POST /v1/status/sync`
- Retorno enriquecido para UI:
  - `status`, `status_reason`
  - `graph_status`, `graph_name`
  - `error_code`, `error_summary`, `error_message`
  - `source`, `webhook_field`, `webhook_level`
  - `source_ad_id`, `source_ad_status` (quando aplicavel)

### 3) Consulta de status da conta na Meta (novo)

- Endpoint:
  - `GET /v1/ad-accounts/{ad_account_id}/meta-status`
- Protegido por auth + autorizacao por ad account (`requireAdAccountAccess`).
- Retorna 2 blocos:
  - `system`: estado interno no banco
  - `meta`: estado consultado no Graph API

Arquivos principais no backend:

- `internal/httpapi/handlers_meta_webhook.go`
- `internal/httpapi/handlers_meta_status.go`
- `internal/httpapi/status_view.go`
- `internal/httpapi/router.go`
- `internal/httpapi/responses.go`

## Fluxo end-to-end

1. Usuario abre app e seleciona conta.
2. Front lista entidades (criativo/campanha/conjunto/anuncio).
3. Front pode sincronizar snapshot via `/v1/status?refresh=true` (ou endpoint de sync).
4. Backend consulta Graph, atualiza cache e devolve status consolidado.
5. UI mostra status em cada card:
   - Criativo: status do criativo + status no anuncio (quando houver)
   - Campanha: efetivo + configurado
   - Conjunto: efetivo + configurado
   - Anuncio: efetivo + configurado
6. Barra de contexto da conta mostra:
   - `Conta`
   - `Conta no sistema`
   - `Conta na Meta`
   - `BM no sistema`

## Decisao de UX aplicada (pedido do produto)

Removido da UI principal:

- `BM na Meta`
- exibicao de `BM ID`

Motivo:

- evitar informacao tecnica desnecessaria para usuario final
- reduzir ruido visual
- manter foco no que realmente muda operacao diaria

Obs.: o backend ainda consegue consultar BM na Meta internamente, mas isso foi ocultado da tela principal por decisao de produto.

## "Nao consultado" / "Indisponivel" - explicacao real

Causa encontrada no caso real:

- Front estava apontando para Cloud Run (`ApiConfig.url` em producao).
- Endpoint novo ainda nao estava deployado na revisao ativa.
- Chamada retornava `404`, entao a UI nao recebia payload de Meta.

Correcao aplicada:

1. deploy de nova imagem no Cloud Run com o endpoint:
   - revisao: `creative-backend-00041-kw8`
2. ajuste de UX no front:
   - quando a consulta falha, mostra `Indisponivel` (em vez de texto ambiguo)

## Deploy executado (backend)

Imagem:

- `us-central1-docker.pkg.dev/rogakronos/titan-repo/backend:meta-account-status-20260305-1300`

Servico:

- `creative-backend`
- URL: `https://creative-backend-663062637696.us-central1.run.app`
- Revisao ativa: `creative-backend-00041-kw8`

Validacao pos-deploy:

- `GET /v1/ad-accounts/{id}/meta-status` sem token -> `401` (esperado)
- importante: nao retorna mais `404`

## Configuracao de Webhook (mantida)

Variaveis de ambiente (Cloud Run):

- `META_WEBHOOK_VERIFY_TOKEN`
- `META_WEBHOOK_APP_SECRET`

Teste de verificacao:

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
curl.exe -i "$BASE/v1/meta/webhooks/ad-account?hub.mode=subscribe&hub.verify_token=$VERIFY&hub.challenge=12345"
```

Esperado:

- HTTP `200`
- body: `12345`

## Inscricao por Ad Account (obrigatorio por conta)

Cada ad account precisa ser inscrita no webhook:

```powershell
$TOKEN = "TOKEN_META_COM_ads_management"
$AD = "act_1427227328791737"
$body = @{
  subscribed_fields = "with_issues_ad_objects,in_process_ad_objects"
  access_token      = $TOKEN
}
Invoke-RestMethod -Method Post -Uri "https://graph.facebook.com/v24.0/$AD/subscribed_apps" -Body $body
Invoke-RestMethod -Method Get -Uri "https://graph.facebook.com/v24.0/$AD/subscribed_apps?access_token=$TOKEN"
```

## Troubleshooting rapido

### 1) `404` no endpoint novo

- backend sem deploy da revisao com rota nova.

### 2) `invalid_webhook_verify_token` (403)

- token usado no teste diferente do token salvo em env.

### 3) PowerShell quebrando `curl -X/-d`

- usar `curl.exe` ou `Invoke-RestMethod`.

### 4) `Invalid OAuth access token - Cannot parse`

- token com BOM/caractere invisivel; aplicar trim/sanitize.

### 5) Conta sem status util na UI

- confirmar:
  - endpoint `/v1/status` funcionando
  - webhook inscrito para a ad account correta
  - token com permissao na conta

## Seguranca operacional

- Nao versionar `APP_SECRET` nem access tokens.
- Se houve exposicao, rotacionar imediatamente.
- Manter segredos no Secret Manager.
