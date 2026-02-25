# Resumo Executivo - Arquitetura SM + Firebase

## Objetivo

Evoluir o backend para um modelo seguro e escalável de acesso por usuário e por Business Manager (BM), removendo dependência de configuração legado por conta de anúncio.

## O que mudou

1. Configuração sensível por BM no Secret Manager.
2. Autenticação de usuários com Firebase ID Token.
3. Autorização por escopo:
   - `uid` -> `bm_uuid` -> `ad_account_id`.
4. Frontend autenticado:
   - login/cadastro no Firebase,
   - chamadas API com `Bearer token`.
5. Backend web-ready:
   - CORS habilitado para preflight e header `Authorization`.
6. Catálogo escopado por usuário:
   - listagens de clients/ad-accounts filtradas por `uid`.

## Diagrama (alto nível)

```text
Usuario (Flutter)
   |
   | 1) Login Firebase (ID token)
   v
Backend Go (API)
   |
   | 2) Valida token Firebase
   | 3) Verifica acesso uid -> BM -> ad_account
   v
PostgreSQL ----------------------+
  app_users                      |
  user_bm_access                 |
  ad_accounts (bm_uuid)          |
  business_managers              |
                                 |
                                 v
                        Secret Manager (config/token BM)
                                 |
                                 v
                            Meta Graph API
```

## Resultado de negócio

- Segurança: usuário só opera contas autorizadas.
- Governança: segredos centralizados por BM.
- Escalabilidade: onboarding de novas BMs sem alterar código.
- Operação: front e backend alinhados com autenticação real de usuário.

## Status atual

Pronto:

- Auth Firebase no backend.
- AuthZ por BM/ad_account via banco.
- Resolução de configuração/token por BM via Secret Manager.
- Front com login/cadastro e consumo autenticado da API.
- CORS corrigido para frontend web (preflight `OPTIONS`).
- Filtro de clientes e ad accounts por `uid` autenticado.

Pontos pendentes recomendados:

1. Aplicar/verificar autorização em 100% das rotas sensíveis.
2. Enforce de role (`viewer/admin/operator`) por operação.
3. Observabilidade de 401/403/200 com `uid`, `bm_uuid`, `ad_account_id`.
4. Fechar gap de autorização em `PATCH/DELETE /v1/ads`.

## Risco conhecido

- Magic link não é suportado de forma estável no plugin Firebase Auth para Windows desktop.
- Estratégia atual:
  - Windows: login por Email/Senha.
  - Web: magic link suportado.

## Documentos técnicos completos

- `ARQUITETURA_SM_FIREBASE_AUTORIZACAO.md`
- `MAPEAMENTO_TECNICO_END_TO_END.md`
- `RUNBOOK_FINAL_BM_PROVISIONING.md`
- `RELATORIO_AVANCOS_SM_FIREBASE_2026-02-25.md`
