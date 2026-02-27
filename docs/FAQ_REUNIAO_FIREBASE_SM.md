# FAQ Reuniao - Firebase + Secret Manager + RBAC

Guia curto para respostas objetivas em reunioes tecnicas e de status.

## 1) O que foi implementado de Firebase no backend?

- Validacao de `Bearer ID Token` no middleware.
- Extracao de `uid/email` para o contexto.
- Endpoint `/v1/me` para diagnostico.
- Sync (upsert) de usuario em `app_users`.

## 2) O que foi implementado de Secret Manager?

Resolucao de config/token por BM:

`ad_account_id -> bm_uuid -> business_managers(project_id, secret_name) -> Secret Manager`.

## 3) Qual e a regra de autorizacao hoje?

Usuario so opera uma `ad_account_id` se houver vinculo ativo em `user_bm_access` para a BM daquela conta. Sem vinculo: `403 forbidden_for_ad_account`.

## 4) O que mudou no banco?

- Migration `007`: `ad_accounts.bm_uuid` + FK para `business_managers`.
- Migration `008`: `app_users` e `user_bm_access` (PK `uid,bm_uuid` + role).

## 5) Por que criar `app_users` se ja existe Firebase?

Para manter espelho local da identidade autenticada e permitir join/controle de acesso no PostgreSQL.

## 6) Por que sincronizar usuario em cada rota autenticada?

Para nao depender de `/v1/me`, manter `uid/email` atualizados e evitar divergencia operacional.

## 7) Isso nao aumenta carga no banco?

E um upsert simples por PK (`uid`). Custo baixo no volume atual.

## 8) Como provar que o legado por conta nao e mais fonte principal?

Teste pratico: com `ad_accounts.token_ref` invalido, a operacao continuou funcionando porque a resolucao veio da BM no Secret Manager.

## 9) O que significa Auth x AuthZ aqui?

- Auth: validar quem e o usuario (`ID token` Firebase).
- AuthZ: validar o que ele pode operar (`user_bm_access` + BM da conta).

## 10) O que faltava e foi corrigido no Web?

Faltava CORS no router para preflight. Foi corrigido e validado com:

- `OPTIONS /v1/clients` = `204`
- `Access-Control-Allow-Origin/Headers/Methods` presentes.

## 11) Por que antes apareciam contas sem acesso no filtro?

Porque listagem era global. Agora:

- `/v1/clients` filtra por `uid`.
- `/v1/clients/{client_uuid}/ad-accounts` filtra por `uid`.

## 12) Magic link no Windows falhando e problema do backend?

Nao. E limitacao do plugin de Firebase Auth no Windows desktop.

## 13) Email/senha com `invalid-credential` e erro do Go?

Nao. Esse erro vem da camada Firebase Auth do frontend (senha incorreta, provider/sessao, usuario sem credencial de senha).

## 14) Role ja esta sendo aplicado como RBAC completo?

Ainda nao. `role` ja existe no banco, mas o enforcement por acao (`viewer/admin/operator`) ainda e pendente.

## 15) Quais gaps ainda estao abertos?

- Enforce de role por operacao.
- Fechar autorizacao em `PATCH/DELETE /v1/ads`.
- Observabilidade de seguranca (`uid`, `bm_uuid`, `ad_account_id`, `401/403/200`).

## 16) Fluxo final em uma frase

Front envia token -> backend valida Firebase -> sincroniza usuario -> valida acesso BM/ad_account -> resolve config no Secret Manager -> chama Meta API.

## 17) Referencias no codigo

- `internal/auth/firebase.go`
- `internal/auth/context.go`
- `internal/httpapi/middleware_auth.go`
- `internal/httpapi/middleware.go`
- `internal/httpapi/handlers.go`
- `internal/storage/postgres.go`
- `internal/storage/migrations/007_link_ad_accounts_to_bm.sql`
- `internal/storage/migrations/008_user_bm_access.sql`

## 18) Front ja esta publicado em producao web?

Sim. Publicado no Firebase Hosting em:

- `https://glineui.web.app`
- `https://glineui.firebaseapp.com`

## 19) O erro `Unsupported operation: MultipartFile... dart:io` era backend?

Nao. Era frontend Web usando fluxo de upload para desktop (`fromPath` + `dart:io`).

Correcao aplicada:

- Windows: upload por `path`.
- Web: upload por `bytes` (`MultipartFile.fromBytes`).

## 20) Qual e o contrato de erro oficial da API hoje?

A resposta de erro e:

- `error` (compatibilidade)
- `error_code` (codigo estavel)
- `user_message` (mensagem para usuario final)

Exemplo:

```json
{
  "error": "forbidden_for_ad_account",
  "error_code": "forbidden_for_ad_account",
  "user_message": "Voce nao tem acesso a esta conta de anuncios."
}
```

## 21) Isso quebra clientes antigos?

Nao. O campo antigo `error` foi mantido.

## 22) Onde fica o erro tecnico real?

Nao vai para o payload do cliente.

Fica no log do backend com:

- status;
- `error_code`;
- causa tecnica (`cause`) quando houver.

## 23) O frontend usa qual campo para mostrar erro?

- UI: `user_message`
- Regras no app: `error_code`
- Fallback legado: `error`
