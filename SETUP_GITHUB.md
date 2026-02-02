# 🚀 Setup Profissional no GitHub - Creative Service

Este guia mostra como configurar e versionar o projeto de forma profissional com conventional commits e Husky.

---

## 📋 Pré-requisitos

```bash
# Instalar Node.js (para Husky/Commitlint)
# https://nodejs.org/ (versão LTS)

# Verificar instalação
node --version
npm --version
```

---

## 🔧 Etapa 1: Configurar Husky e Commitlint

```bash
# Instalar dependências do Husky
npm install

# Configurar Husky (cria pasta .husky)
npx husky install

# Adicionar hook de commit-msg
npx husky add .husky/commit-msg 'npx --no -- commitlint --edit "$1"'
```

**Windows PowerShell**: Se o comando acima falhar, crie manualmente:

```bash
# Criar arquivo .husky/commit-msg
mkdir .husky -ErrorAction SilentlyContinue
@"
#!/usr/bin/env sh
. "`$(dirname "`$0")/_/husky.sh"

npx --no -- commitlint --edit `$1
"@ | Out-File -FilePath .husky/commit-msg -Encoding utf8
```

---

## 📝 Etapa 2: Padrão de Commits Convencionais

### Formato
```
<tipo>(<escopo>): <descrição curta>

[corpo opcional - explicação detalhada]

[rodapé opcional - breaking changes, issues]
```

### Tipos Permitidos
- **feat**: Nova funcionalidade
- **fix**: Correção de bug
- **docs**: Documentação
- **style**: Formatação (não afeta código)
- **refactor**: Refatoração de código
- **perf**: Melhoria de performance
- **test**: Adição/modificação de testes
- **chore**: Tarefas de manutenção
- **ci**: Mudanças no CI/CD
- **build**: Mudanças no build system

### Exemplos
```bash
feat(api): adicionar endpoint de criação de campanhas
fix(worker): corrigir race condition no processamento de jobs
docs(readme): atualizar instruções de instalação
refactor(meta): extrair lógica de retry para função separada
chore(deps): atualizar dependências do Go
```

---

## 🌿 Etapa 3: Estrutura de Branches Profissional

### Branch Principal
- **main** - Código em produção (sempre estável)

### Branches de Desenvolvimento
- **develop** - Branch de integração (staging)

### Branches de Feature
- **feature/nome-da-feature** - Novas funcionalidades
- **fix/nome-do-bug** - Correções de bugs
- **refactor/nome-do-refactor** - Refatorações
- **docs/nome-da-doc** - Documentação

---

## 🚀 Etapa 4: Pipeline de Comandos Git

### 4.1 Inicializar Repositório Local

```bash
# Inicializar Git
git init

# Adicionar todos os arquivos
git add .

# Primeiro commit (estrutura inicial)
git commit -m "chore: initial project setup with complete architecture"
```

---

### 4.2 Criar e Conectar Repositório no GitHub

**No GitHub:**
1. Criar novo repositório (ex: `creative-service`)
2. **NÃO** inicializar com README, .gitignore ou license
3. Copiar a URL do repositório

**No terminal:**
```bash
# Adicionar remote
git remote add origin https://github.com/SEU_USUARIO/creative-service.git

# Renomear branch para main
git branch -M main

# Push inicial
git push -u origin main
```

---

### 4.3 Criar Branch Develop

```bash
# Criar e mudar para branch develop
git checkout -b develop

# Push da branch develop
git push -u origin develop
```

---

### 4.4 Organizar Código em Branches Temáticas

#### Branch 1: Core Infrastructure
```bash
# Criar branch
git checkout -b feature/core-infrastructure

# Adicionar arquivos core
git add cmd/ internal/config/ internal/storage/ internal/queue/ internal/secrets/ internal/blob/
git commit -m "feat(core): implement database, Redis queue, and blob storage layers"

git add internal/meta/
git commit -m "feat(meta): implement Meta API client with retry logic"

git add internal/service/semaphore.go
git commit -m "feat(service): implement semaphore for concurrency control"

# Push branch
git push -u origin feature/core-infrastructure
```

#### Branch 2: Creatives Sync
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b feature/creatives-sync

# Adicionar código de creatives síncronos
git add internal/service/creatives_sync.go
git commit -m "feat(creatives): implement synchronous image creative upload"

git add internal/httpapi/handlers.go internal/httpapi/router.go internal/httpapi/middleware.go internal/httpapi/responses.go
git commit -m "feat(api): implement HTTP handlers and middleware for creatives"

git add cmd/api/
git commit -m "feat(api): implement API server entrypoint"

# Push branch
git push -u origin feature/creatives-sync
```

#### Branch 3: Creatives Async (Worker)
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b feature/creatives-async

# Adicionar código de worker
git add internal/service/jobs_async.go internal/service/worker_processor.go
git commit -m "feat(worker): implement async video creative job processing"

git add cmd/worker/
git commit -m "feat(worker): implement worker entrypoint for background processing"

# Push branch
git push -u origin feature/creatives-async
```

#### Branch 4: Campaign Management
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b feature/campaign-management

# Adicionar endpoints de campaigns, adsets, ads
git add internal/service/campaigns.go
git commit -m "feat(campaigns): implement campaign creation endpoint"

git add internal/service/adsets.go
git commit -m "feat(adsets): implement adset creation endpoint"

git add internal/service/ads.go
git commit -m "feat(ads): implement ad creation endpoint"

# Push branch
git push -u origin feature/campaign-management
```

#### Branch 5: Docker & Infrastructure
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b chore/docker-infrastructure

# Adicionar arquivos Docker
git add Dockerfile docker-compose.yml Makefile
git commit -m "chore(docker): add Dockerfile and docker-compose for local development"

git add internal/storage/migrations/
git commit -m "chore(db): add PostgreSQL migrations for schema setup"

# Push branch
git push -u origin chore/docker-infrastructure
```

#### Branch 6: Documentation
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b docs/complete-documentation

# Adicionar documentação
git add README.md
git commit -m "docs(readme): add README with quick start guide"

git add explicacao_arquitetura.md
git commit -m "docs(architecture): add comprehensive architecture documentation"

git add .gitignore .env.example
git commit -m "chore: add .gitignore and .env.example"

# Push branch
git push -u origin docs/complete-documentation
```

#### Branch 7: Git Workflow Setup
```bash
# Voltar para develop
git checkout develop

# Criar branch
git checkout -b chore/git-workflow-setup

# Adicionar configurações de commit
git add package.json commitlint.config.js .husky/
git commit -m "chore(git): setup Husky and commitlint for conventional commits"

git add SETUP_GITHUB.md
git commit -m "docs(setup): add GitHub setup guide with professional workflow"

# Push branch
git push -u origin chore/git-workflow-setup
```

---

### 4.5 Merge via Pull Requests (Recomendado)

**No GitHub:**
1. Ir em "Pull Requests" → "New Pull Request"
2. Selecionar base: `develop` ← compare: `feature/core-infrastructure`
3. Adicionar título: "feat(core): Core infrastructure implementation"
4. Adicionar descrição explicando as mudanças
5. Criar PR e fazer merge
6. Repetir para todas as branches na ordem acima

---

### 4.6 OU Merge Local (Alternativa Rápida)

```bash
# Voltar para develop
git checkout develop

# Merge de cada branch na ordem
git merge feature/core-infrastructure
git merge feature/creatives-sync
git merge feature/creatives-async
git merge feature/campaign-management
git merge chore/docker-infrastructure
git merge docs/complete-documentation
git merge chore/git-workflow-setup

# Push develop atualizado
git push origin develop
```

---

### 4.7 Release para Main

```bash
# Quando develop estiver estável, merge para main
git checkout main
git merge develop

# Tag de versão
git tag -a v1.0.0 -m "feat: initial release with complete Meta Ads API integration"

# Push main com tags
git push origin main --tags
```

---

## 🔄 Fluxo de Trabalho Diário

### Criar Nova Feature
```bash
# Atualizar develop
git checkout develop
git pull origin develop

# Criar branch da feature
git checkout -b feature/nova-funcionalidade

# Fazer commits
git add .
git commit -m "feat(escopo): descrição da mudança"

# Push e criar PR
git push -u origin feature/nova-funcionalidade
```

### Correção de Bug
```bash
git checkout develop
git pull origin develop

git checkout -b fix/corrigir-bug-xpto

# Fazer correção
git add .
git commit -m "fix(worker): corrigir memory leak no processamento de vídeos"

git push -u origin fix/corrigir-bug-xpto
```

---

## ✅ Checklist Final

- [ ] Husky instalado e configurado
- [ ] Commitlint funcionando (testa com commit inválido)
- [ ] .gitignore configurado
- [ ] .env.example criado (sem secrets reais)
- [ ] Repositório criado no GitHub
- [ ] Branch main configurada como default
- [ ] Branch develop criada
- [ ] Todas as features em branches separadas
- [ ] Pull Requests criados e revisados
- [ ] Documentação completa commitada
- [ ] Tag v1.0.0 criada

---

## 🎯 Convenções do Projeto

### Nomes de Branches
- `feature/` - Novas funcionalidades
- `fix/` - Correção de bugs
- `refactor/` - Refatoração de código
- `docs/` - Documentação
- `chore/` - Manutenção/configuração
- `test/` - Adição de testes

### Commits
- Usar conventional commits sempre
- Subject em português OK, mas tipos em inglês
- Commits atômicos (uma mudança lógica por commit)
- Evitar "WIP", "fix", "update" genéricos

### Pull Requests
- Título descritivo seguindo conventional commits
- Descrição explicando O QUE mudou e POR QUÊ
- Referenciar issues relacionadas
- Solicitar review antes de merge

---

## 🆘 Troubleshooting

### Husky não roda no Windows
```bash
# Dar permissão aos hooks
icacls .husky\commit-msg /grant:r "%USERNAME%:(RX)"
```

### Commitlint não valida
```bash
# Testar manualmente
echo "feat: teste" | npx commitlint
```

### Conflitos no merge
```bash
# Resolver conflitos
git status
# Editar arquivos em conflito
git add .
git commit -m "merge: resolve conflicts from feature/x"
```

---

## 📚 Recursos

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Husky Documentation](https://typicode.github.io/husky/)
- [Git Flow](https://nvie.com/posts/a-successful-git-branching-model/)
- [Commitlint](https://commitlint.js.org/)

---

**Dica Final**: Use `git log --oneline --graph --all` para visualizar a árvore de commits! 🌳
