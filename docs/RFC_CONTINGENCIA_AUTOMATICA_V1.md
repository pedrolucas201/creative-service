# RFC - Contingencia Automatica V1

Status: Draft (aguardando validacao de produto/operacao)  
Data: 2026-03-06  
Autores: Time Backend + Operacao Ads

## 1. Objetivo
Criar um fluxo de contingencia automatica para campanhas de ads:

- detectar queda de entrega/status critico por `ad` e por `ad_account`;
- redirecionar a campanha inteira para outro no elegivel;
- pausar a origem apos switch com sucesso;
- manter trilha completa de auditoria e controle de risco.

## 2. Escopo da V1
Inclui:

- deteccao hibrida (Webhook Meta + monitoramento periodico);
- contingencia para outro no dentro da mesma BM;
- switch no nivel de campanha inteira;
- pausa automatica da origem apos switch concluido;
- limites de tentativa/cooldown;
- rastreabilidade completa no banco.

Nao inclui (fase posterior):

- troca automatica entre BMs;
- roteamento com heuristica avancada/ML;
- aprovacao humana obrigatoria em cada switch.

## 3. Premissas Aprovadas Ate Aqui

1. Deteccao por duas fontes: evento (webhook) e varredura periodica.
2. A unidade de contingencia e a campanha inteira.
3. A origem deve ser pausada apos switch bem-sucedido.
4. Troca de BM fica para fase 2 (inicialmente fora da automacao).

## 4. Arquitetura Proposta (V1)

## 4.1 Componentes

1. `Cloud Scheduler` (a cada 5 min): dispara endpoint de monitoramento.
2. Endpoint `monitor` no backend: identifica candidatos e cria execucoes.
3. `Cloud Tasks`: processa execucoes unitarias com retry controlado.
4. Orquestrador de contingencia: decide no destino e executa switch.
5. Persistencia de estado: incidentes, tentativas, mapeamentos e locks.

## 4.2 Fluxo Alto Nivel

1. Webhook e/ou monitor identifica campanha em estado critico.
2. Sistema abre incidente idempotente para a campanha.
3. Sistema seleciona proximo no elegivel da mesma BM.
4. Sistema replica/ativa estrutura no no destino.
5. Em sucesso: pausa origem + grava mapeamento origem/destino.
6. Em falha: tenta proximo no respeitando cooldown e limite diario.
7. Ao estourar limite: marca `manual_required`.

## 5. Regras de Negocio da V1

## 5.1 O que e considerado "queda"

Queda por conta (imediata):

- conta desabilitada/restrita para entrega.

Queda por campanha (confirmada):

- sinais criticos no nivel de `ad` que inviabilizam entrega;
- confirmacao em 2 leituras consecutivas (janela de 10 min) para evitar falso positivo.

Observacao: evento critico via webhook pode antecipar abertura do incidente.

## 5.2 Politica de Tentativas (recomendacao)

1. Maximo de 3 switches por campanha em 24h.
2. Cooldown de 30 min entre tentativas da mesma campanha.
3. Apenas 1 incidente ativo por campanha (lock de concorrencia).
4. Sem loop infinito: quando atingir limite, exige acao manual.

## 5.3 Politica de Pausa da Origem

1. Somente pausar origem apos confirmacao de criacao/ativacao no destino.
2. Se falhar apos pausa parcial, registrar rollback/manual_required.

## 6. Modelo de Dados (proposto)

Tabela `contingency_nodes`:

- `node_uuid` (PK)
- `bm_uuid`
- `ad_account_id`
- `priority`
- `weight`
- `is_active`
- `cooldown_until`
- `last_used_at`

Tabela `contingency_routes`:

- `route_uuid` (PK)
- `source_ad_account_id`
- `target_node_uuid`
- `order_index`
- `is_active`

Tabela `contingency_incidents`:

- `incident_uuid` (PK)
- `campaign_id`
- `source_ad_account_id`
- `trigger_type` (`webhook`/`polling`)
- `reason_code`
- `status` (`detected`, `queued`, `executing`, `switched`, `failed`, `manual_required`)
- `opened_at`
- `closed_at`

Tabela `contingency_executions`:

- `execution_uuid` (PK)
- `incident_uuid`
- `attempt`
- `target_node_uuid`
- `status`
- `error_code`
- `error_message`
- `started_at`
- `finished_at`

Tabela `entity_switch_map`:

- `switch_uuid` (PK)
- `incident_uuid`
- `source_campaign_id`
- `target_campaign_id`
- `source_adset_ids` (jsonb)
- `target_adset_ids` (jsonb)
- `source_ad_ids` (jsonb)
- `target_ad_ids` (jsonb)
- `created_at`

## 7. API Interna (proposta)

1. `POST /v1/contingency/monitor`  
   Executa varredura, abre incidentes e enfileira execucoes.

2. `POST /v1/contingency/execute`  
   Processa uma execucao (acionado por Cloud Tasks).

3. `POST /v1/contingency/incidents/{incident_uuid}/retry`  
   Retry manual (RBAC admin/owner).

4. `GET /v1/contingency/incidents`  
   Lista incidentes e status de execucao.

## 8. Idempotencia e Concorrencia

1. Chave idempotente por campanha+janela para abertura de incidente.
2. Lock por campanha durante execucao para impedir corrida.
3. Task com deduplicacao por `incident_uuid + attempt`.
4. Operacoes de escrita em transacao sempre que possivel.

## 9. RBAC e Seguranca

1. Execucao automatica via service account interna.
2. Endpoints manuais de override apenas para `owner` e `admin`.
3. Todas as acoes auditadas com `uid`, `bm_uuid`, `ad_account_id`, `incident_uuid`.

## 10. Observabilidade

Logs obrigatorios:

- abertura/fechamento de incidente;
- no escolhido e motivo;
- tentativa/sucesso/falha por execucao;
- pausa da origem;
- erros de API Meta.

Metricas sugeridas:

- incidentes abertos por hora;
- taxa de switch com sucesso;
- tempo medio de recuperacao;
- campanhas em `manual_required`;
- tentativas por campanha/24h.

Alertas sugeridos:

- pico de incidentes;
- taxa de falha acima de limiar;
- fila de tasks acumulando.

## 11. Plano de Entrega

Fase A:

- schema de tabelas;
- endpoint monitor;
- abertura de incidente idempotente;
- observabilidade basica.

Fase B:

- executor com Cloud Tasks;
- selecao de no na mesma BM;
- pausa de origem em sucesso.

Fase C:

- endpoints de operacao manual;
- dashboard de incidentes;
- hardening de retries e rollback.

Fase D (posterior):

- contingencia entre BMs com aprovacao/manual gate.

## 12. Riscos e Mitigacoes

Risco: falso positivo e troca desnecessaria.  
Mitigacao: confirmacao em 2 leituras + cooldown.

Risco: loop de switch e custo alto.  
Mitigacao: limite 3/24h + `manual_required`.

Risco: inconsistencias origem/destino.  
Mitigacao: mapeamento de entidades + trilha de execucao + rollback parcial.

Risco: perda de governanca em troca cross-BM.  
Mitigacao: adiar para fase 2 com gate manual.

## 13. Pendencias para Aprovacao

1. Definicao final dos `reason_code` que disparam contingencia.
2. Ordem de escolha de nos (prioridade fixa, peso, round-robin).
3. Politica de rollback quando destino cria parcialmente.
4. SLA esperado entre deteccao e switch concluido.
5. Se pausa origem deve ocorrer sempre ou somente com validacao extra de entrega.

## 14. Referencia de implementacao (Etapa 1)

A implementacao incremental e a explicacao em dois niveis (tecnico + leigo) estao detalhadas em:

- `docs/RUNBOOK_CONTINGENCIA_ETAPA1.md`
- `docs/RUNBOOK_CONTINGENCIA_ETAPA2.md`
- `docs/RUNBOOK_CONTINGENCIA_ETAPA3.md`
- `docs/RUNBOOK_CONTINGENCIA_ETAPA4.md`
- `docs/RUNBOOK_CONTINGENCIA_ETAPA5_SCHEDULER_TASKS.md`
- `docs/RUNBOOK_CONTINGENCIA_CONFIGURACAO_NOS_ROTAS.md`
- `docs/RUNBOOK_CONTINGENCIA_TELA_OPERACAO.md`
- `docs/RUNBOOK_CONTINGENCIA_CHECKPOINT_2026-03-13.md`

Resumo rapido do estado atual (ate Etapa 5):

1. Detecta candidatos criticos por status de `ad`.
2. Abre incidente idempotente por campanha+conta.
3. Permite listar incidentes por conta.
4. Possui executor de tentativa com selecao de no e historico de execucoes.
5. Possui operacao manual (owner/admin) para:
   - detalhar incidente + execucoes,
   - forcar retry de incidente,
   - encerrar incidente manualmente.
6. UI separada em `Contingencia > Configuracao` e `Contingencia > Operacao`.
7. A automacao com Scheduler/Tasks esta pronta em codigo, pendente de habilitacao completa de IAM/API no ambiente.
8. Checkpoint atual de bloqueio tecnico no switch esta documentado em:
   - `docs/RUNBOOK_CONTINGENCIA_CHECKPOINT_2026-03-13.md`
6. Executa switch real (campanha/adsets/ads) para no de contingencia elegivel.
7. Pausa campanha de origem apenas apos switch concluido com sucesso.
8. Registra mapeamento tecnico em `entity_switch_map`.
9. Possui pipeline automatica com:
   - `Cloud Scheduler` -> `/v1/internal/contingency/tick`
   - `Cloud Tasks` -> `/v1/internal/contingency/execute`
10. Possui tela operacional de contingencia no frontend para:
   - selecionar conta monitorada,
   - rodar simulacao e monitoramento real,
   - visualizar resultado da ultima leitura,
   - operar incidentes e execucoes manualmente.
11. Possui visao de configuracao no frontend para:
   - listar contas elegiveis da mesma BM,
   - cadastrar/atualizar nos,
   - cadastrar/atualizar/remover rotas.
