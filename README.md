# creative-service (Meta Ads API)

Serviço interno para criação de **Ad Creatives** via Meta Marketing API.

## Decisões
- **Imagem**: sync
- **Vídeo**: async (job + worker)
- **Thumbnail**: obrigatória para vídeo
- **Mapeamento por `client_id`**: resolve `ad_account_id`, `page_id` e token (System User) via Postgres.

## Endpoints

### Creatives
- `POST /v1/creatives/image` (multipart): `client_id` + campos + `image`
- `POST /v1/jobs/creatives/video` (multipart): `client_id` + campos + `video` + `thumbnail`
- `GET /v1/jobs/{job_id}`

### Campaign Flow
- `POST /v1/campaigns` - Criar campanha
- `POST /v1/adsets` - Criar conjunto de anúncios
- `POST /v1/ads` - Criar anúncio final

### Health
- `GET /v1/health`

## Rodar local
```bash
docker compose up --build
psql 'postgres://postgres:postgres@localhost:5432/creatives?sslmode=disable' -f internal/storage/migrations/001_init.sql
```

Inserir clients:
```sql
INSERT INTO clients(client_id, ad_account_id, page_id, token_ref)
VALUES ('francisco', 'ACT_ID', 'PAGE_ID', 'ENV:TOKEN_FRANCISCO');
```

Exportar tokens:
```bash
export TOKEN_FRANCISCO='EAAB...'
```

## Observação (MVP)
A fila Redis usa `LPUSH` + `BRPOP` (sem ACK). Para produção, evoluir para `RPOPLPUSH` + processing list.
