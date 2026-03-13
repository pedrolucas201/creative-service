# Runbook - Contingencia Checkpoint (Stand-by)

Status: Stand-by (bloqueio tecnico identificado)  
Data: 2026-03-13

## 1. Objetivo deste checkpoint

Registrar o estado real da implementacao de contingencia no momento da pausa:

1. o que ja esta pronto e validado;
2. o que esta parcialmente pronto;
3. onde o fluxo esta parando hoje;
4. qual erro tecnico foi observado por ultimo.

## 2. O que esta pronto

## 2.1 Backend (feature operacional)

1. Monitor de candidatos criticos por `ad` (status em cache).
2. Abertura de incidente idempotente por campanha+conta.
3. Fila de execucoes com tentativas e historico em banco.
4. Selecao de no de contingencia por rota ativa e fallback na mesma BM.
5. Endpoints manuais de operacao:
   - `POST /v1/contingency/monitor`
   - `GET /v1/contingency/incidents`
   - `GET /v1/contingency/incidents/{incident_uuid}`
   - `POST /v1/contingency/execute`
   - `POST /v1/contingency/incidents/{incident_uuid}/retry`
   - `POST /v1/contingency/incidents/{incident_uuid}/close`
6. Endpoints de configuracao de malha:
   - `GET /v1/contingency/config`
   - `POST /v1/contingency/nodes`
   - `POST /v1/contingency/routes`
   - `DELETE /v1/contingency/routes/{route_uuid}`
7. Endpoints internos para automacao (Scheduler/Tasks):
   - `POST /v1/internal/contingency/tick`
   - `POST /v1/internal/contingency/execute`

## 2.2 Frontend (area de contingencia)

1. Menu lateral com dropdown:
   - `Contingencia > Configuracao`
   - `Contingencia > Operacao`
2. Tela de configuracao para:
   - cadastrar/atualizar no;
   - cadastrar/atualizar rota;
   - remover rota.
3. Tela de operacao para:
   - monitorar/simular;
   - abrir/reaproveitar incidentes;
   - executar tentativa manual;
   - carregar historico de execucoes.
4. Painel de `Ultima execucao manual` para rastrear resultado mesmo quando o incidente sai da fila aberta.

## 3. Onde paramos

O fluxo avanca ate a etapa de switch, seleciona o destino corretamente e inicia a replica, mas o ultimo erro observado na sessao atual foi:

1. falha ao consultar dados de creative de origem antes de criar o creative no destino.

Erro exibido no painel da UI:

```text
Codigo: contingency_switch_failed
Detalhe: get source creative 2027445481445702: meta api error: code=100 subcode=0 type=OAuthException msg=(#100) Tried accessing nonexisting field (template_data)
```

Resumo tecnico:

1. `campaign` de destino e selecionada no fluxo;
2. a etapa de leitura do creative de origem tenta campo inexistente para aquele objeto/conta;
3. a Meta retorna erro `(#100) Tried accessing nonexisting field (template_data)`;
4. a execucao encerra como `failed` e nao cria campanha/adset/ad/creative na tentativa em questao.

## 4. Hipotese tecnica consolidada

O endpoint de leitura de creative esta pedindo um conjunto de fields mais amplo do que o objeto realmente suporta no contexto atual.

Implicacao:

1. basta ajustar a lista de fields permitidos no `GetCreative` da contingencia;
2. apos isso, revalidar a criacao de creative no destino e o restante da cadeia (adset/ad).

## 5. O que falta para concluir esta frente

## 5.1 Curto prazo (destravar switch)

1. remover `template_data` (e quaisquer fields nao suportados) da leitura do creative de origem;
2. reexecutar tentativa manual;
3. validar criacao completa:
   - campanha
   - adsets
   - creatives
   - ads
4. validar `entity_switch_map` com ids de origem/destino.

## 5.2 Proximo passo (observabilidade de operacao)

1. implementar historico persistente de incidentes na UI (`open`, `failed`, `switched`, `closed`);
2. manter trilha de auditoria acessivel sem depender de estado de sessao.

## 6. Estado de automacao (Scheduler/Tasks)

1. codigo da automacao esta pronto;
2. ambiente ainda depende de liberacoes de IAM/API para rodar 100% automatico em producao;
3. operacao manual segue funcional para testes e troubleshooting.

