# Runbook Meta Webhook Status (Ad Account)

## Objetivo

Documentar o fluxo de webhook da Meta para status de objetos de Ads (ad, adset, campaign, creative), desde a configuracao do app ate a confirmacao do update no banco (`entity_status_cache`).

## Escopo Implementado no Backend

- Endpoint de verificacao:
  - `GET /v1/meta/webhooks/ad-account`
- Endpoint de recebimento:
  - `POST /v1/meta/webhooks/ad-account`
- Validacoes:
  - `hub.verify_token` (GET)
  - `X-Hub-Signature-256` com HMAC SHA-256 (POST)
- Processamento:
  - interpreta payload de `ad_account`
  - resolve `entity_type/entity_id`
  - consulta status atual no Graph API
  - faz upsert em `entity_status_cache`

## Pre-requisitos

1. Meta App criado e com acesso a Ads.
2. Permissao no token da Meta:
   - `ads_management`
   - `business_management` (recomendado)
3. Backend publicado com rota de webhook.
4. Variaveis de ambiente no Cloud Run:
   - `META_WEBHOOK_VERIFY_TOKEN`
   - `META_WEBHOOK_APP_SECRET`

## Configurar Verify Token (PowerShell 5.1+)

```powershell
$bytes = New-Object 'System.Byte[]' 32
$rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create()
$rng.GetBytes($bytes)
$VERIFY = -join ($bytes | ForEach-Object { $_.ToString('x2') })
$rng.Dispose()
$VERIFY
```

## Atualizar Env no Cloud Run

```powershell
$APP_SECRET = "SUA_APP_SECRET"
gcloud run services update creative-backend `
  --region us-central1 `
  --project rogakronos `
  --update-env-vars "META_WEBHOOK_VERIFY_TOKEN=$VERIFY,META_WEBHOOK_APP_SECRET=$APP_SECRET"
```

## Smoke Test da Verificacao

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
curl.exe -i "$BASE/v1/meta/webhooks/ad-account?hub.mode=subscribe&hub.verify_token=$VERIFY&hub.challenge=12345"
```

Esperado:
- HTTP `200`
- body: `12345`

## Configurar no Meta App (UI)

1. Meta Developers -> App -> Webhooks.
2. Produto/objeto: `Ad Account`.
3. Callback URL:
   - `https://creative-backend-663062637696.us-central1.run.app/v1/meta/webhooks/ad-account`
4. Verify token:
   - mesmo valor de `META_WEBHOOK_VERIFY_TOKEN`.
5. Clique `Verificar e salvar`.

## Inscricao por Ad Account (obrigatorio por conta)

Observacao: validar webhook no app nao inscreve automaticamente todas as ad accounts.

Para cada ad account que deve enviar eventos:

```powershell
$TOKEN = "TOKEN_META_COM_ads_management"
$AD = "act_1427227328791737"
$body = @{
  subscribed_fields = "with_issues_ad_objects,in_process_ad_objects"
  access_token      = $TOKEN
}
Invoke-RestMethod -Method Post -Uri "https://graph.facebook.com/v24.0/$AD/subscribed_apps" -Body $body
```

Confirmar inscricao:

```powershell
Invoke-RestMethod -Method Get -Uri "https://graph.facebook.com/v24.0/$AD/subscribed_apps?access_token=$TOKEN"
```

Esperado:
- retorno com `app_id` do app cadastrado.

## Validacao no Banco

```sql
SELECT entity_type, entity_id, ad_account_id, status, synced_at
FROM entity_status_cache
WHERE ad_account_id = 'act_1427227328791737'
ORDER BY synced_at DESC
LIMIT 20;
```

## Resultado de Negocio Esperado

1. Operacao de ads gera mudanca de status na Meta.
2. Meta envia evento para webhook.
3. Backend atualiza `entity_status_cache`.
4. App consulta status e exibe para usuario.

## Campos enriquecidos no retorno de status

Nos endpoints:

- `GET /v1/status`
- `POST /v1/status/sync`

os itens de `statuses` agora incluem (alem dos campos originais) informacoes prontas para UI:

- `source`: origem do payload (ex.: `meta_webhook`)
- `webhook_field`: campo do webhook que disparou (ex.: `with_issues_ad_objects`)
- `webhook_level`: nivel do objeto (ex.: `AD`, `ADSET`, `CAMPAIGN`)
- `error_code`
- `error_summary`
- `error_message`
- `status_reason`: mensagem amigavel principal para exibicao
- `graph_status`: status resolvido vindo do Graph
- `graph_name`: nome do objeto no Graph
- `source_ad_id` / `source_ad_status` (quando aplicavel)

Exemplo simplificado:

```json
{
  "entity_type": "ad",
  "entity_id": "120236220628090377",
  "status": "WITH_ISSUES",
  "source": "meta_webhook",
  "webhook_field": "with_issues_ad_objects",
  "error_code": "567",
  "error_summary": "Problema de politica",
  "error_message": "Anuncio reprovado por politica X",
  "status_reason": "Anuncio reprovado por politica X"
}
```

## Limitacao Atual (importante)

Com os campos inscritos hoje:
- `with_issues_ad_objects`
- `in_process_ad_objects`

Voce recebe principalmente eventos de transicao para `WITH_ISSUES` e `IN_PROCESS`.
Para cobertura mais ampla de estados finais, manter fallback de consulta (`/v1/status?refresh=true`) e/ou ampliar estrategia de polling sob demanda.

## Como interpretar os 3 status (sem confusao)

No backend e na API existem tres visoes de status para a mesma entidade:

1. `configured_status`:
   - o que foi solicitado/configurado (ex.: ACTIVE, PAUSED).
2. `effective_status`:
   - o estado real que a Meta esta aplicando agora.
3. `status` (final):
   - status consolidado usado pela UI como status principal.

Regra pratica para produto:
- UI mostra `status` como principal.
- UI mostra `Motivo do status` quando houver (`status_reason`/erros da Meta).
- UI mostra detalhes tecnicos apenas quando `configured_status` e `effective_status` divergem.

## Troubleshooting Real (casos enfrentados)

### 1) `404 Not Found` no callback

Causa:
- imagem antiga em producao, sem rota de webhook.

Correcao:
- build/push/deploy de nova imagem do backend.

### 2) `invalid_webhook_verify_token` (403)

Causa:
- token enviado no teste diferente do token salvo na env do Cloud Run.

Correcao:
- atualizar `META_WEBHOOK_VERIFY_TOKEN` e testar com o mesmo valor literal.

### 3) PowerShell: `curl -X` com erro (`-X` / `-d` nao reconhecido)

Causa:
- alias `curl` do PowerShell chama `Invoke-WebRequest`.

Correcao:
- usar `curl.exe` ou `Invoke-RestMethod`.

### 4) Graph API erro `code 100 / subcode 33`

Causa:
- token sem acesso/permissao na ad account, ou id incorreto para aquele token.

Correcao:
- listar `me/adaccounts` com o mesmo token e usar um `act_<id>` realmente acessivel.

## Seguranca Operacional

1. Nunca versionar `APP_SECRET` e access tokens.
2. Se houve exposicao, rotacionar imediatamente:
   - App Secret (Meta)
   - Access Token
3. Preferir Secret Manager para segredos em producao.
