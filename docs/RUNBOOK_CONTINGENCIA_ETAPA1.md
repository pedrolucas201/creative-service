# Runbook - Contingencia Etapa 1

Data: 2026-03-06

## Objetivo da etapa

Implementar apenas a fundacao:

- schema de contingencia no banco (migration 010);
- endpoint para monitorar candidatos e abrir incidentes idempotentes;
- endpoint para listar incidentes por ad account.

Sem switch automatico nesta etapa.

## Endpoints novos

1. `POST /v1/contingency/monitor`
2. `GET /v1/contingency/incidents`

## Como funciona nesta etapa

1. O monitor consulta status de Ads da conta (`ListAds`) para atualizar `entity_status_cache`.
2. Seleciona Ads criticos (`DISAPPROVED` e `WITH_ISSUES`).
3. Deduplica por `campaign_id`.
4. Abre incidente em `contingency_incidents` (idempotente por campanha+conta quando incidente esta aberto).

## Teste rapido (PowerShell)

```powershell
$BASE = "https://creative-backend-663062637696.us-central1.run.app"
$TOKEN = "<ID_TOKEN_FIREBASE>"
$AD = "act_1427227328791737"

# 1) Dry run (nao cria incidente)
$body = @{
  ad_account_id = $AD
  dry_run = $true
  refresh_status = $true
  max_candidates = 50
  trigger_type = "polling"
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/monitor" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $body

# 2) Execucao real (cria incidente quando houver candidato)
$body = @{
  ad_account_id = $AD
  dry_run = $false
  refresh_status = $true
  max_candidates = 50
  trigger_type = "polling"
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BASE/v1/contingency/monitor" `
  -Headers @{ Authorization = "Bearer $TOKEN"; "Content-Type" = "application/json" } `
  -Body $body

# 3) Listar incidentes abertos
Invoke-RestMethod -Method Get -Uri "$BASE/v1/contingency/incidents?ad_account_id=$AD&status=open&limit=50" `
  -Headers @{ Authorization = "Bearer $TOKEN" }

# 4) Listar todos os incidentes
Invoke-RestMethod -Method Get -Uri "$BASE/v1/contingency/incidents?ad_account_id=$AD&status=all&limit=50" `
  -Headers @{ Authorization = "Bearer $TOKEN" }
```

## Regras desta etapa

1. RBAC: usa a mesma regra de escrita por ad account (owner/admin/operator).
2. Idempotencia: so 1 incidente aberto por campanha+conta (`detected`, `queued`, `executing`).
3. Trigger type permitido: `polling`, `webhook`, `manual`.

## Tabelas criadas na migration 010

1. `contingency_nodes`
2. `contingency_routes`
3. `contingency_incidents`
4. `contingency_executions`
5. `entity_switch_map`

## Proximo passo (Etapa 2)

Implementar executor (fila/task) para processar incidente e preparar switch real.

---

## Explicacao tecnica completa (Etapa 1)

## 1. O que foi implementado de fato

A Etapa 1 entrega a fundacao da contingencia, sem realizar switch automatico:

1. Estrutura de banco para contingencia (`migration 010`).
2. Endpoint de monitoramento para detectar candidatos criticos e abrir incidente idempotente.
3. Endpoint de consulta de incidentes.
4. Reuso do RBAC ja existente por `ad_account_id`.

Arquivos principais:

- `internal/storage/migrations/010_contingency_foundation.sql`
- `internal/httpapi/handlers_contingency.go`
- `internal/storage/contingency.go`
- `internal/httpapi/router.go`

## 2. Fluxo tecnico ponta a ponta

### 2.1 POST `/v1/contingency/monitor`

1. Valida JSON e `ad_account_id`.
2. Valida permissao de escrita na conta (`owner`, `admin`, `operator`).
3. Opcionalmente faz refresh de status chamando `Ads.ListAds(...)` para atualizar `entity_status_cache`.
4. Busca candidatos criticos em `entity_status_cache`:
   - `entity_type = 'ad'`
   - `status IN ('DISAPPROVED', 'WITH_ISSUES')`
5. Deduplica por `campaign_id` (1 incidente por campanha no processamento atual).
6. Para cada campanha:
   - `dry_run=true`: so retorna o candidato.
   - `dry_run=false`: chama `CreateOrGetOpenContingencyIncident(...)`.
7. Retorna contadores:
   - escaneados,
   - deduplicados,
   - incidentes criados,
   - incidentes ja existentes,
   - falhas parciais (se houver).

### 2.2 Idempotencia real

No banco existe indice unico parcial:

- `idx_contingency_incident_open_unique`
- chave: `(source_campaign_id, source_ad_account_id)`
- aplicado somente quando status esta em `('detected', 'queued', 'executing')`

Resultado:

- se ja existe incidente aberto para campanha+conta, novo insert nao duplica;
- backend captura erro `23505` e devolve o incidente aberto existente.

### 2.3 GET `/v1/contingency/incidents`

1. Exige `ad_account_id`.
2. Valida acesso de leitura da conta.
3. Aceita filtro:
   - `status=open` (padrao)
   - `status=all`
4. Aceita `limit` (cap em 200).
5. Retorna lista ordenada por `opened_at DESC`.

## 3. Regras implementadas no codigo

1. `trigger_type` permitido: `polling`, `webhook`, `manual`.
2. `max_candidates`:
   - default `50`
   - maximo `200`
3. `refresh_status`:
   - default `true` quando ausente
4. `reason_code` derivado do status do Ad:
   - `DISAPPROVED` -> `ad_disapproved`
   - `WITH_ISSUES` -> `ad_with_issues`
   - fallback -> `ad_status_critical`
5. `evidence` salvo em JSON com contexto tecnico (`ad_id`, `campaign_id`, status, erros, `synced_at`).

## 4. Estrutura SQL criada e papel de cada tabela

1. `contingency_nodes`
   - cadastro de nos de contingencia (destinos potenciais).
   - nesta etapa: estrutura pronta, ainda nao usada para switch.

2. `contingency_routes`
   - rota de origem -> no de destino com prioridade.
   - nesta etapa: estrutura pronta, ainda nao executa roteamento.

3. `contingency_incidents`
   - registro central de incidente por campanha/conta.
   - nesta etapa: tabela efetivamente usada pelo monitor.

4. `contingency_executions`
   - historico de tentativas de execucao por incidente.
   - nesta etapa: ainda nao populada por executor.

5. `entity_switch_map`
   - mapeamento origem/destino de campanha/adset/ad.
   - nesta etapa: estrutura pronta para fase de switch.

## 5. Limites atuais (importante)

1. Nao faz switch automatico ainda.
2. Nao pausa origem ainda.
3. Nao usa Cloud Tasks/Scheduler nesta etapa.
4. Deteccao de candidato esta focada em status critico de `ad`.

## 6. Como isso se conecta com o roadmap

Etapa 1 = "detectar e registrar com seguranca".  
Etapa 2 = "executar tentativa controlada".  
Etapa 3 = "operacao completa com retries, pausa de origem e governanca".

---

## Explicacao leiga (para negocio)

Hoje o sistema ganhou um "radar de risco", nao um "piloto automatico" ainda.

O que ele ja faz:

1. Olha os anuncios da conta e identifica quando algum entra em estado critico (ex.: reprovado).
2. Evita abrir chamados repetidos para a mesma campanha.
3. Registra um incidente com data, motivo e evidencias.
4. Permite listar esses incidentes pela API.

O que ele ainda nao faz nesta etapa:

1. Nao troca automaticamente para conta de contingencia.
2. Nao desliga campanha de origem.
3. Nao faz retries automaticos com fila.

Em termos simples:

- antes: voce descobria problema no olho, manualmente;
- agora: o sistema detecta e registra sozinho;
- proxima etapa: usar esse registro para acionar a troca automatica com seguranca.
