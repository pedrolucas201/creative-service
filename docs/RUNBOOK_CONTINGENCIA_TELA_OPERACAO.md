# Runbook - Tela Operacional de Contingencia

Status: Implementado  
Data: 2026-03-13  
Escopo: Frontend operacional da area de contingencia

## 1. Objetivo

Documentar a visao de `Operacao` da tela de `Contingencia`, criada no frontend para operacao manual assistida do fluxo de contingencia automatica.

Esta tela existe para:

1. escolher a conta de anuncios que sera monitorada;
2. rodar monitoramento manual ou simulacao;
3. visualizar o resultado da ultima leitura;
4. listar incidentes abertos da conta selecionada;
5. executar tentativas manuais de contingencia;
6. consultar o historico de execucoes de cada incidente.

Esta documentacao nao cobre:

1. cadastrar `contingency_nodes`;
2. cadastrar `contingency_routes`;
3. configurar Cloud Scheduler ou Cloud Tasks;
4. automatizar toda a operacao pela interface.

Observacao:

A configuracao de `nos` e `rotas` agora possui runbook proprio:

- `docs/RUNBOOK_CONTINGENCIA_CONFIGURACAO_NOS_ROTAS.md`

## 2. Onde esta a implementacao

Repositorio frontend: `C:\Users\PC\StudioProjects\flutter`

Arquivo principal:

- `lib/screens/contingency_screen.dart`

Ponto de entrada da navegacao:

- `lib/screens/main_list_screen.dart`

Cliente HTTP usado:

- `lib/api/creative_service_api.dart`

## 3. Endpoints usados pela tela

### 3.1 Monitoramento

- `POST /v1/contingency/monitor`

Usado para:

1. simular candidatos criticos (`dry_run = true`);
2. abrir ou reaproveitar incidentes (`dry_run = false`).

### 3.2 Lista de incidentes

- `GET /v1/contingency/incidents`

Usado para buscar incidentes abertos da conta selecionada.

### 3.3 Execucao manual

- `POST /v1/contingency/execute`

Usado para registrar e disparar uma tentativa manual de contingencia para um incidente especifico.

### 3.4 Historico de execucoes

- `GET /v1/contingency/executions`

Usado para buscar o historico persistido de tentativas por incidente.

## 4. Estrutura da tela

## 4.1 Bloco "Fluxo atual da contingencia"

Bloco informativo no topo da tela.

Funcao:

1. contextualizar o operador sobre o fluxo atual;
2. resumir rapidamente o estado da area na sessao atual.

Indicadores exibidos:

1. `Conta selecionada`
   - mostra se existe uma conta de anuncios ativa na tela;
2. `Incidentes abertos`
   - mostra a quantidade de incidentes carregados para a conta atual;
3. `Acionamento nesta tela`
   - sempre indica `Manual`, pois esta visao e operacional;
4. `Atualiza status antes`
   - reflete o estado do toggle de refresh de status.

Etapas visuais mostradas:

1. detectar o problema;
2. abrir o incidente;
3. escolher o destino;
4. executar a troca.

Observacao:

Este bloco nao executa nenhuma acao. Ele existe para leitura operacional.

## 4.2 Bloco "Conta monitorada"

Bloco de escopo operacional.

Funcao:

1. definir em qual conta de anuncios a operacao vai acontecer;
2. deixar explicito que a contingencia e sempre operada por conta.

Campo principal:

- dropdown `Conta de anuncios`

Comportamento ao trocar a conta:

1. limpa resultados antigos da tela;
2. recarrega incidentes da nova conta;
3. passa a usar a nova conta em todos os endpoints.

Pills informativos:

1. nome da conta;
2. `Conta ativa no sistema`;
3. `BM ativa no sistema`.

Importante:

Esses chips refletem o cadastro interno do sistema, nao uma consulta ao vivo da Meta.

## 4.3 Bloco "Analisar e abrir incidentes"

Bloco operacional de monitoramento manual.

Funcao:

1. rodar a analise de candidatos criticos;
2. abrir ou reaproveitar incidentes por campanha.

Campos disponiveis:

1. `Maximo de candidatos`
   - controla quantos candidatos o backend pode considerar na rodada;
2. `Atualizar status antes de monitorar`
   - se ligado, o backend tenta forcar uma leitura fresca antes de decidir.

Botoes disponiveis:

1. `Rodar simulacao`
   - chama o monitor com `dry_run = true`;
   - nao deve ser tratado como monitoramento real de producao;
2. `Monitorar e abrir incidentes`
   - chama o monitor com `dry_run = false`;
   - abre incidente novo ou reaproveita incidente aberto da mesma campanha.

## 4.4 Bloco "Resultado do monitoramento"

Bloco de leitura da ultima execucao de monitoramento feita na sessao atual.

Funcao:

1. resumir o retorno do monitor;
2. mostrar o que foi lido e processado;
3. dar visibilidade a falhas parciais.

Quando ainda nao houve monitoramento na sessao:

- exibe `Sem execucao recente nesta sessao`.

Quando houve leitura:

Metricas exibidas:

1. `Candidatos lidos`
2. `Campanhas unicas`
3. `Incidentes criados`
4. `Incidentes ja existentes`

Lista de campanhas identificadas:

Cada item mostra:

1. campanha afetada;
2. ad que gerou o sinal;
3. status critico do ad;
4. se o incidente foi criado ou reaproveitado;
5. horario de sincronizacao.

Bloco de falhas:

- aparece quando o backend retorna itens em `failed`.

## 4.5 Bloco "Fila operacional de contingencia"

Bloco principal de operacao.

Funcao:

1. listar incidentes abertos para a conta selecionada;
2. permitir tentativa manual de execucao;
3. permitir consulta ao historico de execucoes.

Campo global do bloco:

- `Maximo de tentativas por execucao`

Esse valor e enviado no `POST /v1/contingency/execute`.

Botao do cabecalho:

- `refresh`
- atualiza apenas a lista de incidentes.

Quando nao ha incidente:

- a tela informa que nao existe incidente aberto para a conta atual.

Quando existe incidente:

Cada card mostra:

1. `Campanha {id}`
2. `Incidente {id curto}`
3. status humano do incidente
4. motivo traduzido (`reason_code`)
5. origem do gatilho (`trigger_type`)
6. quantidade de tentativas
7. horario de abertura

Blocos auxiliares por card:

1. `Sinal que abriu o incidente`
   - aparece quando a ultima leitura da sessao tem o item correspondente;
2. `Ultima tentativa manual nesta sessao`
   - mostra o retorno mais recente disparado pela propria tela;
3. `Historico de execucoes`
   - exibido apos carregar execucoes do backend.

Acoes por card:

1. `Executar tentativa manual`
   - chama `POST /v1/contingency/execute`;
2. `Carregar historico` / `Atualizar historico`
   - chama `GET /v1/contingency/executions`.

## 5. Como usar a tela

Fluxo operacional recomendado:

1. escolher a conta em `Conta monitorada`;
2. confirmar se a conta esta ativa no sistema;
3. no bloco de monitoramento:
   - deixar `Atualizar status antes de monitorar` ligado;
   - manter `Maximo de candidatos` em `50`, salvo necessidade operacional;
4. clicar em `Rodar simulacao`;
5. revisar o bloco `Resultado do monitoramento`;
6. se a leitura fizer sentido, clicar em `Monitorar e abrir incidentes`;
7. abrir a `Fila operacional de contingencia`;
8. se existir incidente, clicar em `Executar tentativa manual`;
9. depois clicar em `Carregar historico`;
10. revisar status, destino escolhido e falhas tecnicas.

## 6. O que a tela resolve hoje

1. melhora a leitura do fluxo para operacao;
2. separa claramente conta, monitoramento, resultado e fila;
3. traduz status e motivos para linguagem mais humana;
4. evita depender apenas de retorno bruto de API;
5. deixa a tentativa manual de contingencia acessivel em UI.

## 7. O que ainda falta

1. historico persistente de incidentes recentes (incluindo `switched`, `failed` e `closed`) na propria UI;
2. filtro temporal e exportacao da auditoria de execucoes;
3. integracao visual com o estado de automacao (Scheduler/Tasks habilitado ou nao);
4. painel de saude da malha (origens sem rota, nos inativos, cobertura por BM).

## 8. Navegacao da funcionalidade

A navegacao foi separada no menu lateral em:

1. `Contingencia > Configuracao`
2. `Contingencia > Operacao`

Decisao aplicada:

1. configuracao vem primeiro no fluxo de uso;
2. operacao usa a malha ja cadastrada (nos e rotas);
3. usuario comum opera, enquanto configuracoes criticas ficam para `owner/admin`.
