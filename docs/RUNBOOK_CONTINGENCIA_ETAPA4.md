# Runbook - Contingencia Etapa 4 (Switch Real)

Data: 2026-03-10

## Objetivo da etapa

Executar o switch real quando houver no de contingencia elegivel:

1. clonar campanha/adsets/ads para a conta de destino;
2. pausar a campanha de origem somente apos copia completa;
3. registrar mapeamento tecnico em `entity_switch_map`;
4. fechar incidente com status `switched` em caso de sucesso.

## O que mudou no backend

1. `POST /v1/contingency/execute` e `POST /v1/contingency/incidents/{incident_uuid}/retry`:
   - agora, quando encontra no de destino, executa o switch real;
   - antes apenas marcava incidente como `queued`.

2. `GET /v1/contingency/incidents/{incident_uuid}`:
   - agora retorna tambem:
   - `latest_switch_map`
   - `switch_maps`
   - `switch_map_count`

3. Persistencia:
   - novo uso da tabela `entity_switch_map` para trilha origem -> destino.

## Fluxo tecnico da etapa

1. Inicia tentativa (`running`) com lock transacional.
2. Seleciona no de destino (rota ativa, fallback por prioridade na mesma BM).
3. Busca campanha de origem na Meta.
4. Cria campanha de contingencia no destino (inicialmente `PAUSED`).
5. Lista adsets da origem, filtra pela campanha e replica no destino.
6. Lista ads da origem, filtra pela campanha e replica no destino.
7. Pausa campanha de origem.
8. Grava `entity_switch_map`.
9. Conclui execucao como `succeeded` e incidente como `switched`.

Se qualquer etapa de copia falhar:

1. execucao finaliza como `failed`;
2. incidente volta para `failed` ou `manual_required` (conforme `max_attempts`);
3. origem nao e pausada.

## Payloads

### Execute

```json
{
  "incident_uuid": "4cb17cf2-5af0-44c3-b08b-40bb12ba69ba",
  "max_attempts": 3
}
```

## Teste rapido (PowerShell)

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$TOKEN = "<ID_TOKEN_FIREBASE>"
$AD = "act_724102005983243"

# 1) Monitor para criar/recuperar incidente
$monitorBody = @{
  ad_account_id = $AD
  dry_run = $false
  refresh_status = $true
  max_candidates = 50
  trigger_type = "polling"
} | ConvertTo-Json

$monitor = Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/monitor" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $monitorBody

$INCIDENT = $monitor.items[0].incident_uuid

# 2) Execute (agora faz switch real)
$executeBody = @{
  incident_uuid = $INCIDENT
  max_attempts = 3
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/execute" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $executeBody

# 3) Consultar detalhe (inclui switch_map)
Invoke-RestMethod -Method Get -Uri "$BASE/v1/contingency/incidents/${INCIDENT}?execution_limit=20" `
  -Headers @{ Authorization = "Bearer $TOKEN" }
```

## Respostas esperadas

### Sucesso de switch

1. `incident_status = switched`
2. `execution_status = succeeded`
3. `target_campaign_id` preenchido
4. `switch_map` com ids de origem/destino

### Falha de switch

1. `error_code = contingency_switch_failed`
2. `execution_status = failed`
3. `incident_status = failed` ou `manual_required`
4. `error_detail` com causa tecnica da falha

## Limites atuais

1. Ate esta etapa, nao havia worker assincrono dedicado para o switch real.
   (Na Etapa 5 isso foi implementado com Cloud Scheduler + Cloud Tasks.)
2. Replica assume que os campos obrigatorios de adset/ad estao disponiveis na origem.
3. Cross-BM continua fora do escopo da V1.

## Status atual (checkpoint 2026-03-13)

1. A base da etapa (incidente, execucao e selecao de destino) segue valida.
2. O bloqueio tecnico mais recente no switch esta na leitura de creative de origem:
   - erro Meta `(#100) Tried accessing nonexisting field (template_data)`.
3. Detalhes completos do ponto de parada:
   - `docs/RUNBOOK_CONTINGENCIA_CHECKPOINT_2026-03-13.md`
