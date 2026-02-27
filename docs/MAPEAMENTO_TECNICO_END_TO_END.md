# Mapeamento Técnico End-to-End

Este documento descreve o fluxo real do backend atual: rota HTTP -> handler -> service -> banco -> integrações externas.

## 1. Boot e composição

- Entrypoint: `cmd/api/main.go`
- Inicializa:
- Config (`internal/config`)
- PostgreSQL pool (`pgxpool`)
- Secret Manager resolver (`internal/secrets.SMResolver`)
- BM service (`internal/bm.Service`)
- Storage provider (`internal/s3.Client` ou `internal/storage.GCSClient` via `storage.StorageClient`)
- Semáforo global (`internal/service.Semaphore`)
- Services de domínio (`CreativeSyncService`, `CampaignService`, `AdSetService`, `AdService`)
- Handler HTTP (`internal/httpapi.Handler`)
- Auth Firebase opcional (`AUTH_REQUIRED=true`)

## 2. Pipeline padrão de request

1. Router recebe rota (`internal/httpapi/router.go`).
2. Middlewares globais: `Recoverer`, `AccessLog`, `CORS`.
3. Se auth habilitada: `AuthMiddleware` valida token Firebase e injeta identidade no context.
4. Handler valida payload e parâmetros.
5. Handler chama `requireAdAccountAccess` (na maioria das rotas de negócio):
- Sincroniza usuário em `app_users` com `EnsureAppUser`.
- Verifica ACL por BM/ad_account via `UserCanAccessAdAccount`.
6. Service:
- Busca `ad_accounts` no banco (`GetAdAccount`).
- Resolve BM config (`GetBMConfigByAdAccountID`) lendo `business_managers` + Secret Manager.
- Resolve token com `MultiResolver` (`ENV:` ou `SM:`).
- Chama Meta API via `internal/meta.Client`.
7. Resposta retorna ao handler.

## 3. Mapa por endpoint

## 3.1 Health e identidade

- `GET /v1/health`
- Handler: `Health`
- Banco: nenhum
- Meta: nenhum

- `GET /v1/me`
- Handler: `GetMe`
- Banco: `EnsureAppUser(uid, email)`
- Meta: nenhum

## 3.2 Catálogo interno

- `GET /v1/clients`
- Handler: `ListClients`
- Banco:
  - com identidade: `ListClientsByUID(uid)`
  - fallback sem identidade: `ListClients`
- Meta: nenhum

- `GET /v1/clients/{client_uuid}/ad-accounts`
- Handler: `ListAdAccountsByClient`
- Banco:
  - com identidade: `ListAdAccountsByClientForUID(client_uuid, uid)`
  - fallback sem identidade: `ListAdAccountsByClient`
- Meta: nenhum

- `GET /v1/bms/{bm_uuid}/config`
- Handler: `GetBMConfig`
- Banco: lê `business_managers`
- Secret Manager: lê secret JSON da BM
- Meta: nenhum

## 3.3 Creatives

- `POST /v1/creatives/image`
- Handler: `CreateImageCreative`
- Service: `CreativeSyncService.CreateImageCreative`
- Banco:
- `GetAdAccount`
- `GetClientByUUID`
- `CreateCreative`
- Storage:
- `Upload` mídia (S3 ou GCS)
- `GetURL` pública
- BM/Secrets:
- `GetBMConfigByAdAccountID`
- `Resolve(token_ref)`
- Meta:
- `UploadImage`
- `CreateCreative`
- `GetCreative` (validação)

- `POST /v1/creatives/video`
- Handler: `CreateVideoCreative`
- Service: `CreativeSyncService.CreateVideoCreative`
- Banco:
- `GetAdAccount`
- `GetClientByUUID`
- `CreateCreative`
- Storage:
- `Upload` vídeo
- `Upload` thumbnail
- `GetURL`
- BM/Secrets:
- `GetBMConfigByAdAccountID`
- `Resolve(token_ref)`
- Meta:
- `UploadVideo`
- `UploadImage` (thumb)
- `CreateCreative`
- `GetCreative` (validação)

- `GET /v1/creatives?ad_account_id=...&type=image|video`
- Handler: `ListCreatives`
- Banco: `ListCreatives(ad_account_id, typeFilter)`
- Meta: nenhum

- `GET /v1/creatives/{creative_id}`
- Handler: `GetCreative`
- Banco: `GetCreative`
- Meta: nenhum

- `DELETE /v1/creatives/{creative_id}`
- Handler: `SoftDeleteCreative`
- Banco:
- `GetCreative`
- `SoftDeleteCreative`
- Meta: nenhum

## 3.4 Campaigns

- `POST /v1/campaigns`
- Handler: `CreateCampaign`
- Service: `CampaignService.CreateCampaign`
- Banco: `GetAdAccount`
- BM/Secrets: `GetBMConfigByAdAccountID`, `Resolve`
- Meta: `CreateCampaign`

- `GET /v1/campaigns?ad_account_id=...`
- Handler: `ListCampaigns`
- Service: `CampaignService.ListCampaigns`
- Banco: `GetAdAccount`
- Meta: `ListCampaigns`

- `PATCH /v1/campaigns/{campaign_id}`
- Handler: `UpdateCampaign`
- Service: `CampaignService.UpdateCampaign`
- Banco: `GetAdAccount`
- Meta: `UpdateCampaign`

- `DELETE /v1/campaigns/{campaign_id}?ad_account_id=...`
- Handler: `DeleteCampaign`
- Service: `CampaignService.DeleteCampaign`
- Banco: `GetAdAccount`
- Meta: `SoftDeleteCampaign` (`status=DELETED`)

## 3.5 AdSets

- `POST /v1/adsets`
- Handler: `CreateAdSet`
- Service: `AdSetService.CreateAdSet`
- Banco: `GetAdAccount`
- Meta: `CreateAdSet`

- `GET /v1/adsets?ad_account_id=...`
- Handler: `ListAdSets`
- Service: `AdSetService.ListAdSets`
- Banco: `GetAdAccount`
- Meta: `ListAdSets`

- `PATCH /v1/adsets/{adset_id}`
- Handler: `UpdateAdSet`
- Service: `AdSetService.UpdateAdSet`
- Banco: `GetAdAccount`
- Meta: `UpdateAdSet`

- `DELETE /v1/adsets/{adset_id}?ad_account_id=...`
- Handler: `DeleteAdSet`
- Service: `AdSetService.DeleteAdSet`
- Banco: `GetAdAccount`
- Meta: `SoftDeleteAdSet` (`status=DELETED`)

## 3.6 Ads

- `POST /v1/ads`
- Handler: `CreateAd`
- Service: `AdService.CreateAd`
- Banco: `GetAdAccount`
- Meta: `CreateAd`

- `GET /v1/ads?ad_account_id=...`
- Handler: `ListAds`
- Service: `AdService.ListAds`
- Banco: `GetAdAccount`
- Meta: `ListAds`

- `PATCH /v1/ads/{ad_id}`
- Handler: `UpdateAd`
- Service: `AdService.UpdateAd`
- Banco: `GetAdAccount`
- Meta: `UpdateAd`

- `DELETE /v1/ads/{ad_id}?ad_account_id=...`
- Handler: `DeleteAd`
- Service: `AdService.DeleteAd`
- Banco: `GetAdAccount`
- Meta: `SoftDeleteAd` (`status=DELETED`)

## 4. Banco de dados (visão final)

- `clients`: cliente de negócio (UUID, name, email, soft delete).
- `ad_accounts`: conta Meta (`ad_account_id` PK), ligada a `client_uuid` e `bm_uuid`.
- `business_managers`: vínculo BM -> (`project_id`, `secret_name`) no Secret Manager.
- `app_users`: usuário autenticado (Firebase UID).
- `user_bm_access`: ACL de usuário por BM (role + ativo).
- `creatives`: criativos persistidos localmente com URL de mídia e metadata.

## 5. Integrações externas

- Meta Graph API:
- Base URL configurável (`META_BASE_URL`, default `https://graph.facebook.com`)
- Versão configurável (`META_API_VERSION`, default `v24.0`)
- Retry com exponential backoff em 429 e 5xx (`internal/meta/client.go`)

- Secret Manager (GCP):
- Resolve `token_ref` e configuração de BM
- Cache em memória com TTL (default 2 minutos)

- Object storage:
- `STORAGE_PROVIDER=s3|gcs`
- Interface única: `storage.StorageClient`

## 6. Pontos de atenção atuais

- CORS web está ativo e validado em produção (`OPTIONS /v1/clients` retornando `204` com `Access-Control-Allow-*`).
- Contrato de erro padronizado ativo na API: `error`, `error_code`, `user_message`.
- Frontend deve priorizar `error_code` para regra e `user_message` para UX.
- Há documentação antiga no repositório citando `worker`, `redis`, `jobs` e `cmd/worker`, mas esse runtime não existe no código atual.
- RBAC por role já existe (`owner`, `admin`, `operator`, `viewer`), mas enforcement fino por operação ainda pode evoluir.
- `AccessLog` não registra request/response hoje (middleware vazio).
