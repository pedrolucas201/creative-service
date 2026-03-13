# Runbook - Contingencia Etapa 2 (Executor)

Data: 2026-03-09

## Objetivo da etapa

Adicionar o executor de contingencia:

1. iniciar tentativa por incidente;
2. selecionar no de destino elegivel na mesma BM;
3. registrar execucao em `contingency_executions`;
4. atualizar status do incidente com controle de tentativas.

Importante: nesta etapa ainda **nao** ocorre switch de campanha e **nao** pausa a origem automaticamente.

## Endpoints novos

1. `POST /v1/contingency/execute`
2. `GET /v1/contingency/executions?incident_uuid=...`

## Fluxo tecnico da execucao

1. Recebe `incident_uuid`.
2. Valida acesso RBAC de escrita na `source_ad_account_id` do incidente.
3. Bloqueia incidente em transacao (`FOR UPDATE`) e abre tentativa:
   - cria linha em `contingency_executions` com status `running`;
   - move incidente para `executing`;
   - incrementa `attempt_count`.
4. Seleciona no de destino:
   - prioridade 1: `contingency_routes` ativo;
   - fallback: `contingency_nodes` ativo da mesma BM;
   - sempre exclui a conta de origem;
   - respeita `cooldown_until`.
5. Finaliza tentativa:
   - com no encontrado:
     - execucao -> `succeeded`;
     - incidente -> `queued`;
     - atualiza `last_used_at` do no.
   - sem no encontrado:
     - execucao -> `failed`;
     - incidente -> `failed` (ou `manual_required` quando bate limite de tentativas).

## Regras de negocio da Etapa 2

1. Limite default de tentativas por chamada: `max_attempts = 3`.
2. `max_attempts` permitido: maximo `10`.
3. Nao permite nova execucao se ja houver tentativa `running` para o mesmo incidente.
4. Estados finais desta etapa:
   - `queued` (planejado para worker de switch);
   - `failed` (sem no elegivel / erro);
   - `manual_required` (estouro do limite de tentativas).

## SQL de apoio (seed de nos/rotas)

Exemplo: criar no para conta de contingencia:

```sql
INSERT INTO contingency_nodes (bm_uuid, ad_account_id, node_name, priority, weight, is_active)
VALUES
  ('e312d632-249a-43d1-8957-b5c1bedb9223', 'act_724102005983243', 'Contingencia 01', 10, 100, TRUE)
ON CONFLICT (ad_account_id)
DO UPDATE SET
  node_name = EXCLUDED.node_name,
  priority = EXCLUDED.priority,
  weight = EXCLUDED.weight,
  is_active = EXCLUDED.is_active,
  updated_at = now();
```

Exemplo: criar rota de origem para destino:

```sql
INSERT INTO contingency_routes (source_ad_account_id, target_node_uuid, order_index, is_active)
SELECT
  'act_1427227328791737',
  node_uuid,
  1,
  TRUE
FROM contingency_nodes
WHERE ad_account_id = 'act_724102005983243'
ON CONFLICT (source_ad_account_id, target_node_uuid)
DO UPDATE SET
  order_index = EXCLUDED.order_index,
  is_active = EXCLUDED.is_active,
  updated_at = now();
```

## Teste rapido (PowerShell)

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$TOKEN = "<ID_TOKEN_FIREBASE>"
$AD = "act_1427227328791737"

# 1) Criar/obter incidente (Etapa 1)
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

# 2) Executar tentativa (Etapa 2)
$executeBody = @{
  incident_uuid = $INCIDENT
  max_attempts = 3
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/execute" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $executeBody

# 3) Ver historico de execucoes
Invoke-RestMethod -Method Get -Uri "$BASE/v1/contingency/executions?incident_uuid=$INCIDENT&limit=50" `
  -Headers @{ Authorization = "Bearer $TOKEN" }
```

## Resposta esperada

Com no elegivel:

- `incident_status = queued`
- `execution_status = succeeded`
- `target_node_found = true`

Sem no elegivel:

- `execution_status = failed`
- `incident_status = failed` ou `manual_required`
- `target_node_found = false`

## Limite atual da etapa

Esta etapa prepara o incidente para switch, mas nao executa copia de campanha/adset/ad nem pausa origem automaticamente.

## Proximo passo (Etapa 3)

Operacao manual para contingencia:

1. consultar incidente por UUID com historico;
2. retry manual (owner/admin);
3. fechamento manual do incidente.

Detalhes em: `docs/RUNBOOK_CONTINGENCIA_ETAPA3.md`.
