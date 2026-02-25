# Relatorio Consolidado de Avancos (SM + Firebase + AuthZ + Web)

Data de referencia: 25/02/2026

Este documento consolida o que foi implementado de ponta a ponta no backend e frontend, os problemas reais encontrados, o que ja foi validado em producao e o que ainda falta.

---

## 1) O que foi entregue

### 1.1 Backend com autenticacao Firebase

- Middleware valida `Authorization: Bearer <ID_TOKEN>`.
- Extrai `uid` e `email` para o contexto da request.
- Endpoint de diagnostico: `GET /v1/me`.
- Sync de usuario no banco via upsert em `app_users`.

Arquivos principais:

- `internal/auth/firebase.go`
- `internal/auth/context.go`
- `internal/httpapi/middleware_auth.go`
- `internal/httpapi/router.go`
- `cmd/api/main.go`

### 1.2 Autorizacao por escopo BM/Ad Account (RBAC base)

- Regra de negocio ativa: `uid -> bm_uuid -> ad_account_id`.
- Sem vinculo ativo em `user_bm_access`: `403 forbidden_for_ad_account`.
- Cobertura aplicada nos endpoints principais de operacao por `ad_account_id`.

Arquivos principais:

- `internal/httpapi/handlers.go`
- `internal/storage/postgres.go`

### 1.3 Config por BM com Secret Manager

- Resolucao por cadeia:
  - `ad_account_id` -> `ad_accounts.bm_uuid`
  - `bm_uuid` -> `business_managers(project_id, secret_name)`
  - leitura de JSON no Secret Manager
  - uso da config para operacoes Meta (campaign/adset/ad/creative)
- Token legado em `ad_accounts.token_ref` deixou de ser fonte principal.

Arquivos principais:

- `internal/bm/service.go`
- `internal/secrets/sm_resolver.go`
- `internal/service/*.go`

### 1.4 Web destravado com CORS

- CORS implementado e registrado no router.
- Preflight `OPTIONS` passou a responder `204` com `Access-Control-Allow-*`.
- Resolveu bloqueio de navegador no frontend web.

Arquivos principais:

- `internal/httpapi/middleware.go`
- `internal/httpapi/router.go`

### 1.5 Filtro de clientes e ad accounts por UID

- `GET /v1/clients` agora respeita o `uid` autenticado.
- `GET /v1/clients/{client_uuid}/ad-accounts` agora respeita o `uid` autenticado.
- Resultado: usuario nao enxerga mais BM/ad account fora do proprio escopo.

Arquivos principais:

- `internal/httpapi/handlers.go`
- `internal/storage/postgres.go`

### 1.6 Frontend Flutter

- AuthGate para navegação autenticado/nao autenticado.
- Login/cadastro com Firebase.
- Bearer token automatico nas chamadas para API.
- Web com magic link e retorno por URL.
- Windows desktop com fallback para email/senha.

---

## 2) Mudancas de banco de dados

### Migration 007

- Adiciona `ad_accounts.bm_uuid`.
- FK: `ad_accounts.bm_uuid -> business_managers.bm_uuid`.

Arquivo:

- `internal/storage/migrations/007_link_ad_accounts_to_bm.sql`

### Migration 008

- Cria `app_users`.
- Cria `user_bm_access` com PK composta `(uid, bm_uuid)`.
- Role permitido: `owner | admin | operator | viewer`.

Arquivo:

- `internal/storage/migrations/008_user_bm_access.sql`

---

## 3) Fluxo end-to-end atual

1. Usuario autentica no Firebase e obtem `ID_TOKEN`.
2. Front chama backend com `Authorization: Bearer <ID_TOKEN>`.
3. Backend valida token e injeta identidade no contexto.
4. Backend sincroniza `app_users` por `uid`.
5. Backend valida acesso ao `ad_account_id` via `user_bm_access`.
6. Backend resolve BM + Secret Manager a partir do `ad_account_id`.
7. Backend chama Meta Graph API e retorna resposta.

---

## 4) Evidencias de validacao realizadas

- Sem token: `401 missing_authorization_header`.
- Token valido: `200` em `/v1/me`.
- Conta permitida: operacoes retornando `200`.
- Conta sem vinculo: `403 forbidden_for_ad_account`.
- Prova de arquitetura BM/SM:
  - `ad_accounts.token_ref` legado invalido,
  - operacao ainda funcionando com token vindo do Secret Manager.
- CORS em producao:
  - `OPTIONS /v1/clients` retornando `204 No Content` com headers CORS.

---

## 5) Problemas reais encontrados e resolvidos

1. Secret version destruida no Secret Manager (`FAILED_PRECONDITION`).
2. Erros de payload no PowerShell com `curl.exe` e escape de JSON.
3. Magic link no Windows desktop com erro de plugin (`type 'bool' is not a subtype of Map<dynamic, dynamic>?`).
4. Web em branco por `firebase_options.dart` sem configuracao de Web.
5. Front web sem dados por preflight `OPTIONS` retornando `405` (corrigido com middleware CORS no router).
6. Filtro mostrando contas sem permissao (corrigido com listagem escopada por UID).

---

## 6) Estado atual

Concluido:

- Firebase Auth no backend.
- AuthZ por BM/ad account.
- Secret Manager por BM.
- CORS para web.
- Listagem de clientes/ad accounts por UID.
- Front com login/cadastro e consumo autenticado da API.

Pendente recomendado:

1. Enforce de role por operacao (`viewer/admin/operator`).
2. Fechar gap de autorizacao em `PATCH /v1/ads/{ad_id}` e `DELETE /v1/ads/{ad_id}` com `requireAdAccountAccess`.
3. Observabilidade de seguranca (metricas e logs por `uid`, `bm_uuid`, `ad_account_id`, `401/403/200`).
4. Definir runbook oficial de onboarding (cliente -> BM -> secret -> ad account -> user_bm_access).
5. Fluxo de senha esquecida no frontend para reduzir erro `invalid-credential`.

---

## 7) Referencias rapidas

- Arquitetura: `docs/ARQUITETURA_SM_FIREBASE_AUTORIZACAO.md`
- Mapeamento tecnico: `docs/MAPEAMENTO_TECNICO_END_TO_END.md`
- Runbook de provisioning: `docs/RUNBOOK_FINAL_BM_PROVISIONING.md`
- FAQ para reunioes: `docs/FAQ_REUNIAO_FIREBASE_SM.md`
