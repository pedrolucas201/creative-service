# Runbook - Contingencia Etapa 3 (Operacao Manual)

Data: 2026-03-09

## Objetivo da etapa

Adicionar operacao manual da contingencia para time operacional (owner/admin):

1. consultar incidente por UUID com historico;
2. forcar retry manual de incidente;
3. encerrar incidente manualmente.

Importante: nesta etapa ainda nao existe switch automatico da campanha entre contas.

## Endpoints novos

1. `GET /v1/contingency/incidents/{incident_uuid}`
2. `POST /v1/contingency/incidents/{incident_uuid}/retry`
3. `POST /v1/contingency/incidents/{incident_uuid}/close`

## Regras de acesso (RBAC)

1. `GET /v1/contingency/incidents/{incident_uuid}`:
   - usa a regra de leitura da ad account do incidente.
2. `POST /retry` e `POST /close`:
   - exigem role `owner` ou `admin` na ad account do incidente.

## Fluxo tecnico da etapa

1. Operador consulta o incidente e valida:
   - status atual,
   - tentativas anteriores,
   - ultimo erro.
2. Se necessario, dispara retry manual:
   - cria nova execucao `running`,
   - tenta escolher no de destino (route -> fallback node),
   - conclui como `succeeded` (incidente vai para `queued`) ou `failed/manual_required`.
3. Se o incidente nao deve mais seguir automatico, encerra manualmente:
   - status final `closed` (padrao) ou `manual_required`.

## Payloads

### Retry manual

```json
{
  "max_attempts": 3
}
```

Observacoes:

1. `max_attempts` default: `3`
2. limite maximo aceito: `10`

### Close manual

```json
{
  "status": "closed",
  "reason_detail": "Incidente tratado manualmente pelo operador."
}
```

`status` aceitos:

1. `closed` (padrao)
2. `manual_required`

## Teste rapido (PowerShell)

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$TOKEN = "<ID_TOKEN_FIREBASE>"
$INCIDENT = "<INCIDENT_UUID>"

# 1) Ver detalhe do incidente
Invoke-RestMethod -Method Get -Uri "$BASE/v1/contingency/incidents/$INCIDENT?execution_limit=20" `
  -Headers @{ Authorization = "Bearer $TOKEN" }

# 2) Retry manual (owner/admin)
$retryBody = @{
  max_attempts = 3
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/incidents/$INCIDENT/retry" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $retryBody

# 3) Encerrar incidente manualmente
$closeBody = @{
  status = "closed"
  reason_detail = "Fechado apos validacao operacional."
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/incidents/$INCIDENT/close" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $closeBody
```

## Respostas esperadas

### Retry com no elegivel

1. `incident_status = queued`
2. `execution_status = succeeded`
3. `target_node_found = true`

### Retry sem no elegivel

1. `execution_status = failed`
2. `incident_status = failed` ou `manual_required`
3. `target_node_found = false`

### Close manual

1. `incident_status = closed` (ou `manual_required`)
2. `closed_at` preenchido

## Erros de API adicionados

1. `invalid_contingency_close_status`
2. `failed_to_close_contingency_incident`
3. `contingency_incident_already_closed`
4. `contingency_incident_close_conflict`

## Limite da Etapa 3 (historico)

1. Esta etapa nao executava switch real de campanha/adset/ad.
2. Esta etapa nao pausava origem automaticamente.
3. A evolucao desses pontos foi entregue na Etapa 4.
