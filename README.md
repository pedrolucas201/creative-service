# creative-service

Backend Go para integração com Meta Ads (campaigns, adsets, ads, creatives), com:

- autenticação por Firebase ID Token;
- autorização por escopo `uid -> bm_uuid -> ad_account_id`;
- resolução de config/token por BM via Secret Manager;
- storage de mídia em S3 ou GCS.

## Stack

- Go 1.24
- PostgreSQL
- Firebase Admin SDK
- Google Secret Manager
- Meta Graph API

## Executar localmente

1. Configure `.env` com:
- `DATABASE_URL`
- `GCP_PROJECT_ID`
- `FIREBASE_PROJECT_ID` (se `AUTH_REQUIRED=true`)
- `AUTH_REQUIRED` (`true` ou `false`)
- `STORAGE_PROVIDER` (`s3` ou `gcs`) e variáveis do provider

2. Rode:

```bash
go run ./cmd/api
```

3. Health check:

```bash
GET /v1/health
```

## Documentação

- Índice da documentação:
  - [docs/README.md](docs/README.md)
- Arquitetura consolidada:
  - [docs/ARQUITETURA_SM_FIREBASE_AUTORIZACAO.md](docs/ARQUITETURA_SM_FIREBASE_AUTORIZACAO.md)
- Mapa técnico end-to-end:
  - [docs/MAPEAMENTO_TECNICO_END_TO_END.md](docs/MAPEAMENTO_TECNICO_END_TO_END.md)
- Runbook de provisionamento:
  - [docs/RUNBOOK_FINAL_BM_PROVISIONING.md](docs/RUNBOOK_FINAL_BM_PROVISIONING.md)
- Resumo executivo:
  - [docs/RESUMO_EXECUTIVO_ARQUITETURA_SM_FIREBASE.md](docs/RESUMO_EXECUTIVO_ARQUITETURA_SM_FIREBASE.md)
- Relatorio consolidado de avancos:
  - [docs/RELATORIO_AVANCOS_SM_FIREBASE_2026-02-25.md](docs/RELATORIO_AVANCOS_SM_FIREBASE_2026-02-25.md)
- Relatorio consolidado atualizado:
  - [docs/RELATORIO_AVANCOS_SM_FIREBASE_2026-02-27.md](docs/RELATORIO_AVANCOS_SM_FIREBASE_2026-02-27.md)
- FAQ de reuniao:
  - [docs/FAQ_REUNIAO_FIREBASE_SM.md](docs/FAQ_REUNIAO_FIREBASE_SM.md)
- Contrato oficial de erros:
  - [docs/CONTRATO_ERROS_API.md](docs/CONTRATO_ERROS_API.md)
- Runbook de webhook de status da Meta:
  - [docs/RUNBOOK_META_WEBHOOK_STATUS.md](docs/RUNBOOK_META_WEBHOOK_STATUS.md)
- Relatorio consolidado da entrega webhook/status/UI (2026-03-04):
  - [docs/RELATORIO_META_WEBHOOK_STATUS_UI_2026-03-04.md](docs/RELATORIO_META_WEBHOOK_STATUS_UI_2026-03-04.md)

## Arquivos legados

Documentos antigos foram movidos para:

- `docs/archive/`

Eles podem conter contexto histórico útil, mas não são a fonte principal da arquitetura atual.
