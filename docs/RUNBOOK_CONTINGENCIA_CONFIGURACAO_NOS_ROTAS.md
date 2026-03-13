# Runbook - Configuracao de Nos e Rotas de Contingencia

Status: Implementado  
Data: 2026-03-13  
Escopo: Backend + frontend da configuracao operacional da malha de contingencia

## 1. Objetivo

Documentar a nova area de `Configuracao` dentro da tela de `Contingencia`.

Esta area existe para:

1. listar as contas elegiveis da mesma BM da conta monitorada;
2. cadastrar ou atualizar `contingency_nodes`;
3. cadastrar ou atualizar `contingency_routes`;
4. remover rotas antigas;
5. tornar a malha de contingencia operavel pelo app, sem depender de SQL manual.

## 2. Conceitos

### 2.1 No

Um `no` e uma conta de anuncios destino elegivel para receber uma contingencia.

Exemplos de atributos operacionais:

1. nome operacional;
2. prioridade;
3. peso;
4. ativo/inativo.

Tabela usada:

- `contingency_nodes`

### 2.2 Rota

Uma `rota` liga a conta monitorada a um no de destino.

Ela define:

1. qual no pode ser usado por aquela conta;
2. em qual ordem de preferencia;
3. se essa preferencia esta ativa ou inativa.

Tabela usada:

- `contingency_routes`

## 3. Onde esta a implementacao

### Backend

- `internal/storage/contingency.go`
- `internal/httpapi/handlers_contingency.go`
- `internal/httpapi/router.go`
- `internal/httpapi/responses.go`

### Frontend

Repositorio: `C:\Users\PC\StudioProjects\flutter`

- `lib/api/creative_service_api.dart`
- `lib/screens/contingency_screen.dart`

## 4. Endpoints novos

### 4.1 GET `/v1/contingency/config`

Query param:

- `ad_account_id`

Funcao:

1. carregar a conta origem selecionada;
2. listar contas elegiveis da mesma BM;
3. listar nos ja cadastrados na mesma BM;
4. listar rotas ja configuradas para a conta origem.

Resposta:

- `source_account`
- `available_accounts`
- `nodes`
- `routes`

### 4.2 POST `/v1/contingency/nodes`

Body:

1. `ad_account_id`
2. `node_name`
3. `priority`
4. `weight`
5. `is_active`
6. `cooldown_until` (opcional)

Funcao:

1. criar um no novo para a conta informada;
2. ou atualizar o no ja existente da mesma conta.

Observacao:

O backend resolve automaticamente a BM do no a partir da `ad_account`.

### 4.3 POST `/v1/contingency/routes`

Body:

1. `source_ad_account_id`
2. `target_node_uuid`
3. `order_index`
4. `is_active`

Funcao:

1. criar uma rota da conta origem para o no destino;
2. ou atualizar a rota existente.

Regra:

O backend valida que origem e destino pertencem a mesma BM e impede apontar a conta para ela mesma.

### 4.4 DELETE `/v1/contingency/routes/{route_uuid}`

Funcao:

1. remover uma rota antiga da conta monitorada.

## 5. Regras de negocio

1. A configuracao de nos e rotas exige perfil alto:
   - `owner`
   - `admin`

2. O backend nao escolhe conta aleatoria:
   - primeiro respeita `contingency_routes`;
   - depois usa fallback por `contingency_nodes` da mesma BM.

3. O no e global para a BM:
   - uma vez cadastrado, ele pode servir a mais de uma conta origem.

4. A rota e especifica da conta origem:
   - ela transforma o no em destino preferencial daquela conta.

5. A rota nao aceita destino:
   - de outra BM;
   - da propria conta origem;
   - de conta deletada/invalida.

## 6. Fluxo da nova UI

### 6.1 Visao "Configuracao"

A tela `Contingencia` agora tem dois modos:

1. `Operacao`
2. `Configuracao`

### 6.2 Bloco "Malha da conta monitorada"

Mostra:

1. conta origem atual;
2. quantidade de contas elegiveis;
3. quantidade de nos cadastrados;
4. quantidade de rotas configuradas.

### 6.3 Bloco "Nos"

Permite:

1. escolher uma conta destino da mesma BM;
2. definir nome operacional;
3. definir prioridade;
4. definir peso;
5. ativar/inativar o no;
6. salvar o cadastro.

Tambem lista os nos ja existentes.

### 6.4 Bloco "Rotas"

Permite:

1. escolher um no destino;
2. definir ordem da rota;
3. ativar/inativar a rota;
4. salvar a rota;
5. remover rotas existentes.

Tambem lista as rotas atuais da conta selecionada.

## 7. Como usar

Fluxo recomendado:

1. abrir `Contingencia`;
2. trocar para `Configuracao`;
3. escolher a conta monitorada;
4. cadastrar os nos destino que a BM podera usar;
5. cadastrar as rotas da conta origem para esses nos;
6. voltar para `Operacao`;
7. rodar simulacao ou monitoramento real.

## 8. Resultado prático

Antes:

1. a tela de contingencia apenas operava incidentes;
2. a configuracao de nos/rotas dependia de SQL manual.

Agora:

1. a mesma area tambem permite preparar a malha de contingencia;
2. owner/admin conseguem manter nos e rotas pelo app;
3. a operacao deixa de depender de ajuste direto no banco na maior parte do fluxo.
