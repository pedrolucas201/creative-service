# Arquitetura Atual (SM + Firebase + Autorização por BM)

Este documento consolida as mudanças feitas após a introdução de:

- Secret Manager como fonte de configuração/tokens por BM.
- Firebase Authentication no backend.
- Autorização por relacionamento `uid -> bm_uuid -> ad_account_id`.
- Ajustes no frontend Flutter para login e consumo autenticado da API.

---

## 1. Objetivo da arquitetura

Antes, a aplicação dependia de configuração/token mais "legado" por `ad_accounts`.

Agora, o objetivo é:

- Centralizar segredo/config por Business Manager (BM) no Secret Manager.
- Resolver automaticamente config/token a partir do `ad_account_id`.
- Exigir autenticação de usuário (Firebase ID Token).
- Exigir autorização por escopo (usuário só opera contas das BMs permitidas).

---

## 2. Mudanças no banco de dados

As duas mudanças estruturais principais foram as migrations `007` e `008`.

## 2.1 Migration 007 (vínculo ad account -> BM)

Arquivo:

- `internal/storage/migrations/007_link_ad_accounts_to_bm.sql`

O que adiciona:

- coluna `ad_accounts.bm_uuid`
- FK `ad_accounts.bm_uuid -> business_managers.bm_uuid`
- índice parcial `idx_ad_accounts_bm_uuid` para performance

Impacto:

- Toda operação por `ad_account_id` passa a poder descobrir sua BM.

## 2.2 Migration 008 (authz por usuário)

Arquivo:

- `internal/storage/migrations/008_user_bm_access.sql`

O que adiciona:

- tabela `app_users` (usuário autenticado do Firebase)
- tabela `user_bm_access` (quais BMs cada uid pode acessar)
- roles (`owner`, `admin`, `operator`, `viewer`)

Impacto:

- O backend passa a validar escopo de acesso por usuário antes de operar em uma ad account.

---

## 3. Backend: fluxo de autenticação e autorização

Arquivos principais:

- `cmd/api/main.go`
- `internal/httpapi/router.go`
- `internal/httpapi/middleware_auth.go`
- `internal/httpapi/handlers.go`
- `internal/storage/postgres.go`

## 3.1 Autenticação (quem é o usuário)

1. Cliente envia header `Authorization: Bearer <ID_TOKEN>`.
2. `AuthMiddleware` valida token no Firebase.
3. Identidade (`uid`, `email`) entra no contexto da requisição.

Se token faltar/inválido:

- `401 missing_authorization_header` ou `401 invalid_or_expired_token`.

## 3.2 Autorização (o que esse usuário pode acessar)

No handler, `requireAdAccountAccess(...)` executa:

1. `EnsureAppUser(uid, email)` em `app_users`.
2. `UserCanAccessAdAccount(uid, ad_account_id)`:
   - faz join `user_bm_access` + `ad_accounts` por `bm_uuid`
   - exige `is_active = true`

Sem permissão:

- `403 forbidden_for_ad_account`.

## 3.3 CORS para frontend Web

Com o frontend em `localhost` e backend em Cloud Run, a API passou a responder preflight (`OPTIONS`) corretamente:

- `Access-Control-Allow-Origin` para origens permitidas
- `Access-Control-Allow-Headers` com `Authorization, Content-Type`
- `Access-Control-Allow-Methods` com `GET, POST, PATCH, DELETE, OPTIONS`
- `204 No Content` para preflight válido

Arquivos:

- `internal/httpapi/middleware.go`
- `internal/httpapi/router.go`

---

## 4. Backend: resolução de configuração via BM + Secret Manager

Arquivos principais:

- `internal/bm/service.go`
- `internal/secrets/sm_resolver.go`
- `internal/service/*.go`

## 4.1 Cadeia de resolução

Para qualquer operação por `ad_account_id`:

1. Busca `ad_accounts.bm_uuid`.
2. Busca metadados da BM em `business_managers` (`project_id`, `secret_name`, `is_active`).
3. Lê secret no Secret Manager.
4. Faz parse do JSON da BM (`token_ref`, `page_id`, etc.).
5. Resolve `token_ref` (ENV/SM) e chama Meta API.

## 4.2 Resultado prático

- O token/config de execução passa a vir da BM via Secret Manager.
- O campo legado em `ad_accounts` deixa de ser a fonte principal.

---

## 5. Backend: fluxo de negócio (resumo)

Para endpoints de campaigns/adsets/ads/creatives:

1. Middleware autentica usuário.
2. Handler valida payload.
3. Handler valida permissão por `ad_account_id`.
4. Service resolve BM/config/token.
5. Service chama Meta Graph API.
6. Responde ao cliente.

Persistência adicional:

- Creatives: grava metadados em `creatives`.
- Storage: usa provider configurado (`S3` ou `GCS`).

---

## 6. Frontend Flutter: mudanças feitas

Repositório frontend:

- `C:\Users\PC\StudioProjects\flutter`

Arquivos principais:

- `lib/main.dart`
- `lib/api/creative_service_api.dart`
- `lib/screens/login_screen.dart`
- `lib/screens/register_screen.dart`
- `lib/screens/main_list_screen.dart`
- `lib/firebase_options.dart`

## 6.1 Navegação de autenticação

- `AuthGate` em `main.dart`:
  - autenticado -> `MainListScreen`
  - não autenticado -> `LoginScreen`

## 6.2 Chamada da API autenticada

- `creative_service_api.dart` passou a anexar automaticamente:
  - `Authorization: Bearer <ID_TOKEN>`
- Isso habilita consumo das rotas protegidas no backend.

## 6.3 Cadastro

- Tela dedicada de cadastro (`register_screen.dart`) com:
  - `createUserWithEmailAndPassword`

## 6.4 Magic Link

- Windows desktop:
  - não suportado no plugin `firebase_auth` atual para desktop.
  - app força `Email/Senha` no Windows.
- Web:
  - fluxo habilitado com auto-complete na volta do link.
  - requer abrir o link no mesmo contexto de navegador/sessão.

---

## 7. Fluxo end-to-end (completo)

## 7.1 Login e acesso ao app

1. Usuário faz login (Email/Senha no Windows; Web pode usar magic link).
2. Firebase mantém sessão (`currentUser`).
3. App entra em `MainListScreen` via `AuthGate`.

## 7.2 Requisição de negócio

1. Front envia request com `Bearer ID_TOKEN`.
2. Backend valida token Firebase.
3. Backend valida acesso do `uid` ao `ad_account_id`.
4. Backend resolve BM e secret no Secret Manager.
5. Backend chama Meta API.
6. Backend responde sucesso/erro.

---

## 8. Estado operacional atual

Pronto:

- Auth Firebase no backend.
- AuthZ por BM/ad_account via banco.
- Resolução automática de config/token por BM via Secret Manager.
- Front consumindo API com token.
- Cadastro de usuário no frontend.
- CORS web (preflight `OPTIONS`) corrigido e validado em produção.
- Listagens por escopo de usuário:
  - `GET /v1/clients` usa `uid` autenticado.
  - `GET /v1/clients/{client_uuid}/ad-accounts` usa `uid` autenticado.

Pendente recomendado:

- Revisar cobertura de `requireAdAccountAccess` em todos os handlers.
- Enforce de role (`viewer/admin/...`) por tipo de operação.
- Observabilidade/auditoria (uid, bm_uuid, ad_account_id, status).

---

## 9. Documentos relacionados

- Mapa técnico de endpoints:
  - `MAPEAMENTO_TECNICO_END_TO_END.md`
- Runbook de provisionamento BM + authz:
  - `RUNBOOK_FINAL_BM_PROVISIONING.md`
- Relatório consolidado de avanço:
  - `RELATORIO_AVANCOS_SM_FIREBASE_2026-02-25.md`
- Arquitetura legada/histórica:
  - `explicacao_arquitetura.md` (contém partes antigas, não 100% alinhadas ao runtime atual)
