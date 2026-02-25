# Relatório Completo: Deploy do Creative Service no GCP

Data de consolidação: 18 de fevereiro de 2026  
Projeto GCP: `rogakronos`  
Serviço Cloud Run: `creative-backend`  
Cloud SQL: `rogakronos:us-central1:febe`  
Bucket GCS: `meta-service-storage`  

---

## 1. Objetivo do processo

O objetivo foi colocar o backend em produção no Google Cloud seguindo a estratégia:

1. Validar Storage no GCS.
2. Configurar Cloud SQL (PostgreSQL) com schema correto.
3. Subir imagem Docker no Cloud Run.
4. Validar funcionamento end-to-end.
5. Depois apontar o Flutter para o novo endpoint.

Em termos leigos: tirar o backend do ambiente local e colocar na nuvem, garantindo que ele continue:
- salvando arquivo de criativo,
- consultando banco,
- falando com a Meta API,
- e respondendo para o app Flutter.

Em termos técnicos: migração de runtime para arquitetura Cloud Run + Cloud SQL + GCS com variáveis de ambiente e IAM adequados.

---

## 2. Arquitetura final (estado desejado e alcançado)

### Leigo
- O app Flutter chama uma URL na internet.
- Essa URL bate no backend no Cloud Run.
- O backend grava/consulta dados no Cloud SQL.
- O backend salva imagens/vídeos no bucket GCS.
- O backend chama a Meta API para criação dos creatives.

### Técnico
- **Compute**: Cloud Run (serviço stateless com imagem Docker)
- **Database**: Cloud SQL PostgreSQL (`febe`)
- **Object Storage**: Cloud Storage (`meta-service-storage`)
- **Auth entre serviços**: Service Account anexada ao Cloud Run
- **Conexão DB recomendada**: Unix socket via `/cloudsql/PROJECT:REGION:INSTANCE`

---

## 3. Pré-requisitos e requisitos levantados

## 3.1 Requisitos de aplicação
- Backend Go com suporte a:
  - `STORAGE_PROVIDER=s3|gcs`
  - conexão PostgreSQL por `DATABASE_URL`
- Dockerfile funcional para Cloud Run
- Variáveis de ambiente corretas no serviço

## 3.2 Requisitos de infraestrutura
- APIs GCP habilitadas (Run, SQL Admin, Artifact Registry, Storage)
- Cloud SQL existente e acessível
- Bucket GCS existente e permissão de escrita para runtime
- Artifact Registry para hospedar a imagem

## 3.3 Requisitos de IAM (importante)
Dois grupos de permissões foram necessários:

1. **Permissões do usuário humano** (`devgomesss@gmail.com`)
   - para operar `gcloud`, deployar, listar logs etc.

2. **Permissões da Service Account de runtime** (`creative-service-runtime@...`)
   - para o container em produção acessar Cloud SQL e GCS.

Sem separar esses dois níveis, o processo quebra em pontos diferentes.

---

## 4. Mudanças de código e configuração feitas no projeto

## 4.1 Migração S3 -> GCS no backend
Foi implementada a abstração de storage para permitir troca por variável de ambiente:

- Criado `internal/storage/storage_interface.go`
- Criado `internal/storage/gcs_client.go`
- `internal/service/creatives_sync.go` passou a usar interface genérica
- `cmd/api/main.go` seleciona provider (`s3` ou `gcs`)
- `internal/config/config.go` ganhou variáveis de GCS

Resultado: o Flutter não precisou mudar para alternar S3/GCS; a troca ficou no backend/config.

## 4.2 Ajustes de Docker para Cloud Run
- `Dockerfile` alinhado com `go.mod` (Go 1.24)
- `CMD ["/bin/api"]`
- `ca-certificates` no runtime
- cópia de migrations no container (para compatibilidade)

## 4.3 Ajuste de inicialização/migrations
- `RUN_MIGRATIONS=false` por padrão no runtime para evitar migração automática inesperada.

## 4.4 `.dockerignore`
Criado para reduzir contexto de build e evitar incluir itens locais/sensíveis.

---

## 5. Passo a passo executado (resumo cronológico)

## 5.1 Storage (GCS)
1. Bucket já existia: `meta-service-storage`.
2. Teste real pelo próprio fluxo da API (`/v1/creatives/image`).
3. Retorno validou URL em `storage.googleapis.com/meta-service-storage/...`.
4. Estrutura de pastas foi mantida no padrão antigo.

### Resultado
Storage em GCS validado com sucesso.

---

## 5.2 Cloud SQL
1. Instância `febe` já existia.
2. Banco `creatives` disponível.
3. Migrations executadas via Cloud SQL Studio (copiar/colar SQL local).
4. Tabelas apareceram no schema.

### Problema encontrado
Apesar das migrations, o schema efetivo da tabela `creatives` estava legado em alguns pontos:
- `s3_url`, `s3_thumb_url`
- `ad_account_uuid` obrigatório
- ausência de `ad_account_id`

### Resultado
Banco acessível e schema ajustado posteriormente para casar com o código.

---

## 5.3 Artifact Registry + Cloud Run
1. Build local da imagem Docker.
2. Push para repo existente `titan-repo` com tag única (`devgomesss-20260218-01`).
3. Deploy no Cloud Run (`creative-backend`) em `us-central1`.
4. Healthcheck em produção: `GET /v1/health` retornando `{"ok":true}`.

### Resultado
Serviço publicado e respondendo.

---

## 6. Erros principais e como foram resolvidos

## 6.1 Erro de IAM para criar repo
Erro:
- `artifactregistry.repositories.create permission denied`

Causa:
- usuário sem permissão para criar Artifact Registry.

Solução:
- usar repo já existente (`titan-repo`) sem precisar criar outro.

---

## 6.2 Erro Cloud SQL socket: `connection refused`
Erro:
```json
{"error":"... dial unix /cloudsql/rogakronos:us-central1:febe/.s.PGSQL.5432: connect: connection refused"}
```

Leigo:
- o backend estava no ar, mas “não conseguia entrar no banco”.

Técnico:
- falha de conexão no socket `/cloudsql/...` no runtime.

O que ajudou a destravar:
1. Confirmar que `/v1/health` funcionava (serviço ok).
2. Reaplicar `DATABASE_URL` no formato correto de socket:
   - senha URL-encoded (`Postgres%402026%21`)
   - `connect_timeout=5`
3. Confirmar anotações Cloud Run com `run.googleapis.com/cloudsql-instances`.

Com isso, `GET /v1/clients` passou a responder 200.

---

## 6.3 Erro de schema após conexão funcionar
Depois da conexão DB funcionar, apareceram erros de persistência:

1. `column "ad_account_id" does not exist`
2. `null value in column "ad_account_uuid" violates not-null`
3. tentativa de FK falhando por ausência de unique em `ad_accounts.ad_account_id`

Causa:
- mismatch entre código atual e schema legado.

Solução:
- alinhar tabela `creatives` ao modelo atual:
  - `s3_url -> url`
  - `s3_thumb_url -> thumb_url`
  - criar `ad_account_id`
  - remover dependência obrigatória de `ad_account_uuid`
- ajustar constraints/FKs conforme chaves existentes.

---

## 6.4 Erro de dados (tabelas vazias)
Mesmo com conexão e schema, era necessário ter dados mínimos:
- `clients` com `francisco`
- `ad_accounts` com `act_1427227328791737`

Sem isso, o fluxo falharia em `get ad account`.

---

## 7. Socket vs TCP: decisão e aprendizado

## 7.1 Socket (`/cloudsql/...`)
### Leigo
Canal interno do GCP entre Cloud Run e Cloud SQL.

### Técnico
Cloud SQL connector gerenciado no runtime do Cloud Run, sem depender de IP público.

Vantagens:
- mais seguro
- menos exposição de rede
- padrão recomendado

## 7.2 TCP/IP público
### Leigo
Conexão direta por IP do banco.

### Técnico
`postgres://user:pass@IP:5432/db?sslmode=require`

Vantagens:
- útil como fallback de diagnóstico rápido.

Riscos:
- depende de rede autorizada
- aumenta superfície de ataque se aberto amplamente

## 7.3 Conclusão
O ambiente ficou funcional com **socket**, que é o caminho correto para produção no GCP.

---

## 8. Uso de gcloud vs GUI

## 8.1 Por que usar `gcloud` (CLI)
Leigo:
- menos clique repetido, menos erro manual.

Técnico:
- reprodutibilidade
- auditabilidade
- facilidade de automação futura (CI/CD)

## 8.2 Quando usar GUI
- verificar recursos
- abrir logs
- consultas SQL no Studio
- checagens visuais rápidas

Estratégia ideal aplicada:
- provisionamento/deploy pela CLI
- observabilidade e apoio pela GUI

---

## 9. Service Account e roles envolvidas

Service Account de runtime:
- `creative-service-runtime@rogakronos.iam.gserviceaccount.com`

Roles relevantes:
- `roles/cloudsql.client`
- `roles/storage.objectAdmin`
- (em alguns cenários) `roles/serviceusage.serviceUsageConsumer`

Importante:
- runtime (service account) e usuário humano têm necessidades diferentes.
- vários erros ocorreram por falta de permissão em um lado, enquanto o outro estava correto.

---

## 10. Estado final validado (produção)

Endpoint:
- `https://creative-backend-663062637696.us-central1.run.app`

Validações realizadas:
1. `GET /v1/health` -> `{"ok":true}`
2. `GET /v1/clients` -> retornando cliente `francisco`
3. `POST /v1/creatives/image` -> sucesso:
   - `creative_id` retornado
   - `image_hash` retornado
   - `validated: true`
   - URL no GCS com path correto em `meta-service-storage`

Resultado final:
- pipeline end-to-end funcionando em produção: Cloud Run + Cloud SQL + GCS + Meta API.

---

## 11. Próximo passo (pendente funcional de produto)

Atualizar Flutter para usar base URL de produção:
- de `localhost`
- para `https://creative-backend-663062637696.us-central1.run.app`

Depois:
- rebuild
- teste fim a fim pelo app

---

## 12. Melhorias recomendadas pós-go-live

1. Mover segredos para Secret Manager (`DATABASE_URL`, tokens Meta).
2. Reduzir privilégios (least privilege) de SA e usuários.
3. Revisar schema legado e eliminar colunas antigas definitivamente.
4. Adicionar monitoramento e alertas (Cloud Monitoring + Error Reporting).
5. Definir padrão de versionamento de imagem e rollback claro.
6. Documentar runbook operacional (deploy, rollback, incidentes).

---

## 13. Conclusão executiva

O deploy foi concluído com sucesso após resolver três blocos:

1. **Infra + IAM** (acesso aos recursos certos)
2. **Conexão Cloud Run -> Cloud SQL** (socket corretamente configurado)
3. **Compatibilidade de schema** (banco alinhado com o código em produção)

Com isso, o sistema passou a operar em nuvem mantendo o comportamento esperado:
- criação de creative na Meta,
- persistência no PostgreSQL,
- armazenamento no GCS.

---

## 14. Evolução pós-go-live: BM + Secret Manager como fonte real

Após o go-live inicial, foi implementada uma evolução de segurança e governança:

Objetivo:
- deixar de depender de `token_ref` legado em `ad_accounts`.
- usar configuração por BM no Secret Manager como fonte oficial.

### 14.1 Modelo aplicado

1. Tabela `business_managers` com:
- `bm_uuid`
- `project_id`
- `secret_name` (nome do secret no SM)

2. Vínculo em `ad_accounts`:
- coluna `bm_uuid` para mapear `ad_account_id -> bm_uuid`.

3. Resolução em runtime:
- request chega com `ad_account_id`.
- backend busca `bm_uuid` no banco.
- backend lê JSON da BM no Secret Manager.
- backend resolve `token_ref` do JSON e chama Meta API.

Em termos leigos:
- cada conta de anúncio aponta para uma BM.
- cada BM aponta para um “cofre” (secret).
- o backend pega o token direto do cofre, não de variável solta.

### 14.2 Mudanças técnicas entregues

- Migration `006_business_managers.sql` (tabela BM)
- Migration `007_link_ad_accounts_to_bm.sql` (vínculo ad account -> BM)
- Services (`creatives`, `campaigns`, `adsets`, `ads`) migrados para resolver token via BM/SM
- Endpoint de validação BM:
  - `GET /v1/bms/{bm_uuid}/config`

### 14.3 Provas de execução em produção

1. Deploy da imagem com integração BM/SM:
- revisão Cloud Run ativa com rota `/v1/bms/.../config`.

2. Endpoint BM respondendo JSON esperado:
- `token_ref`
- `ad_account_id`
- `page_id`
- `bm_id`

3. Criação de campanha em produção validada:
- retorno com `campaign_id` real após resolução via BM/SM.

### 14.4 Incidentes e correções nessa fase

1. Secret em estado destruído:
- erro: `Secret Version ... is in DESTROYED state`
- correção: criar nova versão ativa do secret.

2. Header Authorization inválido:
- erro: `invalid header field value for "Authorization"`
- causa: token com BOM/newline/whitespace no payload do secret.
- correção no código: sanitização com trim de BOM + espaços antes de montar header.

3. Teste manual com JSON inválido no PowerShell:
- correção: usar `Invoke-RestMethod` com `ConvertTo-Json` para evitar escaping incorreto do `curl -d`.

### 14.5 Resultado executivo dessa etapa

Status: **Concluído**

Benefícios:
- maior segurança operacional
- melhor rastreabilidade por BM
- desacoplamento entre dados de conta e segredo sensível
- base pronta para múltiplas BMs por cliente

---

## 15. Pendências estratégicas (próxima fase)

1. Propagar migrations 006/007 para todos os ambientes.
2. Preencher `bm_uuid` para todas as `ad_accounts` ativas.
3. Padronizar secrets restantes (DB URL, tokens) exclusivamente no Secret Manager.
4. Iniciar implementação de autenticação com **Identity Platform (magic link)**.

Conclusão atual:
- infraestrutura e fluxo de anúncios em produção estão estáveis.
- governança de segredos por BM foi implementada e validada.
- próximo ciclo recomendado: autenticação e controle de acesso por usuário (Identity Platform).

---

## 16. Atualização: Firebase Auth habilitável no backend (Magic Link)

Mudança de direção aprovada:
- autenticação via Firebase (projeto `glineui`) no lugar de fluxo direto de Identity Platform no backend.

Implementação técnica entregue:
1. Verificador Firebase Admin SDK em Go para validar ID token.
2. Middleware Bearer token no pipeline HTTP.
3. Rotas de negócio protegidas por autenticação.
4. Feature toggle por ambiente:
   - `AUTH_REQUIRED=true|false`
   - `FIREBASE_PROJECT_ID=<project-id>`

Impacto:
- `/v1/health` permanece público.
- endpoints de negócio exigem token Firebase quando auth está ativa.
- mantém compatibilidade operacional para dev (`AUTH_REQUIRED=false`).

Status:
- código compilando com sucesso (`go test ./...`).
- pronto para ativação em Cloud Run via env vars.

---

## 17. Atualização: autorização por BM/usuário implementada

Além da autenticação Firebase, foi implementado controle de acesso por escopo de negócio:

Objetivo:
- impedir que usuário autenticado acesse qualquer conta de anúncio fora do seu escopo.

Modelo aplicado:
- `app_users` registra usuário (`uid/email`).
- `user_bm_access` vincula `uid` às BMs autorizadas.
- validação em runtime:
  - request traz `ad_account_id`
  - backend valida se `uid` possui acesso à BM dessa conta
  - sem vínculo: `403 forbidden_for_ad_account`

Impacto:
- redução de risco de acesso cruzado entre contas/BMs.
- base pronta para RBAC simples (`role` em `user_bm_access`).

Status:
- patch aplicado em handlers de operações críticas (creatives/campaigns/adsets/ads).
- endpoint de diagnóstico de identidade adicionado (`/v1/me`).
- compilação validada com sucesso.
