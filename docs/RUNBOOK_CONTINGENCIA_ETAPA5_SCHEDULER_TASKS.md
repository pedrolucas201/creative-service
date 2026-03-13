# Runbook - Contingencia Etapa 5 (Cloud Scheduler + Cloud Tasks)

Data: 2026-03-10

## Objetivo da etapa

Automatizar a execução da contingência sem intervenção manual:

1. `Cloud Scheduler` chama um endpoint interno de monitoramento.
2. O monitor detecta incidentes e enfileira execuções no `Cloud Tasks`.
3. Cada task chama um endpoint interno executor.
4. O executor roda a mesma lógica de switch já existente (Etapa 4).

## O que mudou no backend

1. Novos endpoints internos:
   - `POST /v1/internal/contingency/tick`
   - `POST /v1/internal/contingency/execute`
2. Proteção por token interno (`X-Contingency-Token`), sem depender de Firebase para automação.
3. Integração com `Cloud Tasks` para enfileirar execuções por `incident_uuid`.
4. `tick` com suporte a múltiplas contas de anúncio na mesma chamada.

## Pipeline completa (fim a fim)

1. Scheduler (a cada 5 min) envia `POST /v1/internal/contingency/tick`.
2. `tick`:
   - opcionalmente sincroniza status (`refresh_status=true`);
   - lê candidatos críticos (`DISAPPROVED`, `WITH_ISSUES`);
   - cria/recupera incidente idempotente;
   - enfileira task para incidentes em estado `detected`.
3. Task chama `POST /v1/internal/contingency/execute`.
4. `execute`:
   - inicia execução (`running`);
   - seleciona nó de contingência;
   - replica campanha/adsets/ads no destino;
   - pausa origem;
   - grava `entity_switch_map`;
   - conclui incidente/execution.

## Variáveis de ambiente novas

Obrigatórias para automação:

1. `CONTINGENCY_INTERNAL_TOKEN`
2. `CONTINGENCY_TASKS_EXECUTE_URL`
3. `CONTINGENCY_TASKS_PROJECT_ID` (default: `GCP_PROJECT_ID`)
4. `CONTINGENCY_TASKS_LOCATION` (default: `us-central1`)
5. `CONTINGENCY_TASKS_QUEUE` (default: `contingency-executor`)

Recomendadas:

1. `CONTINGENCY_MONITOR_AD_ACCOUNTS` (CSV, ex: `act_1,act_2`)
2. `CONTINGENCY_DISPATCH_VIA_TASKS` (default: `true`)
3. `CONTINGENCY_MAX_CANDIDATES` (default: `50`)
4. `CONTINGENCY_MAX_ATTEMPTS` (default: `3`)
5. `CONTINGENCY_REFRESH_STATUS` (default: `true`)

## Exemplo de update no Cloud Run

```powershell
$SERVICE_URL = "https://creative-backend-663062637696.us-central1.run.app"
$INTERNAL_TOKEN = "<TOKEN_INTERNO_FORTE>"

gcloud run services update creative-backend `
  --project rogakronos `
  --region us-central1 `
  --update-env-vars "CONTINGENCY_INTERNAL_TOKEN=$INTERNAL_TOKEN,CONTINGENCY_TASKS_PROJECT_ID=rogakronos,CONTINGENCY_TASKS_LOCATION=us-central1,CONTINGENCY_TASKS_QUEUE=contingency-executor,CONTINGENCY_TASKS_EXECUTE_URL=$SERVICE_URL/v1/internal/contingency/execute,CONTINGENCY_MONITOR_AD_ACCOUNTS=act_724102005983243,CONTINGENCY_DISPATCH_VIA_TASKS=true,CONTINGENCY_MAX_CANDIDATES=50,CONTINGENCY_MAX_ATTEMPTS=3,CONTINGENCY_REFRESH_STATUS=true"
```

## Criar fila Cloud Tasks

```powershell
gcloud tasks queues create contingency-executor `
  --project rogakronos `
  --location us-central1 `
  --max-attempts=5 `
  --max-retry-duration=3600s `
  --min-backoff=10s `
  --max-backoff=300s `
  --max-doublings=5
```

## Criar job Cloud Scheduler (5 min)

```powershell
$INTERNAL_TOKEN = "<TOKEN_INTERNO_FORTE>"
$BODY = '{"dry_run":false,"dispatch_tasks":true,"refresh_status":true,"trigger_type":"polling","max_candidates":50,"max_attempts":3}'

gcloud scheduler jobs create http contingency-monitor-5m `
  --project rogakronos `
  --location us-central1 `
  --schedule "*/5 * * * *" `
  --uri "https://creative-backend-663062637696.us-central1.run.app/v1/internal/contingency/tick" `
  --http-method POST `
  --headers "Content-Type=application/json,X-Contingency-Token=$INTERNAL_TOKEN" `
  --message-body $BODY
```

Se o job já existir:

```powershell
gcloud scheduler jobs update http contingency-monitor-5m `
  --project rogakronos `
  --location us-central1 `
  --schedule "*/5 * * * *" `
  --uri "https://creative-backend-663062637696.us-central1.run.app/v1/internal/contingency/tick" `
  --http-method POST `
  --headers "Content-Type=application/json,X-Contingency-Token=$INTERNAL_TOKEN" `
  --message-body $BODY
```

## Teste manual rápido (sem esperar 5 minutos)

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$INTERNAL_TOKEN = "<TOKEN_INTERNO_FORTE>"

Invoke-RestMethod -Method Post -Uri "$BASE/v1/internal/contingency/tick" `
  -Headers @{ "Content-Type" = "application/json"; "X-Contingency-Token" = $INTERNAL_TOKEN } `
  -Body (@{
    ad_account_ids  = @("act_724102005983243")
    dry_run         = $false
    dispatch_tasks  = $true
    refresh_status  = $true
    trigger_type    = "polling"
    max_candidates  = 50
    max_attempts    = 3
  } | ConvertTo-Json)
```

## Respostas esperadas

1. `tick`:
   - `incidents_created` / `incidents_existing`
   - `incidents_enqueued`
   - `results[*].enqueued_tasks`
2. `execute` (task):
   - sucesso lógico retorna `HTTP 200`
   - `result.original_status` indica o status da execução de negócio
   - falha 5xx retorna erro HTTP para retry automático do Cloud Tasks

## Erros comuns

1. `invalid_internal_contingency_token`:
   - token enviado no header diferente do `CONTINGENCY_INTERNAL_TOKEN`.
2. `contingency_task_queue_not_configured`:
   - `dispatch_tasks=true` sem configuração completa da fila.
3. `failed_to_list_contingency_candidates`:
   - tabela/migration ausente ou falha de leitura de status.
4. `no_target_node_for_contingency`:
   - existe incidente, mas não há nó elegível para switch.

## Arquivos principais desta etapa

1. `internal/automation/contingency_tasks.go`
2. `internal/httpapi/handlers_contingency_internal.go`
3. `internal/httpapi/handlers_contingency.go` (extração de `runMonitorContingency`)
4. `internal/httpapi/router.go`
5. `internal/config/config.go`
6. `cmd/api/main.go`
