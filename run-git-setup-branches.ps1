# 🌿 Script Automatizado - Criar Branches Temáticas
# Este script organiza o código em branches separadas profissionalmente

Write-Host "🌿 Criando estrutura de branches profissional..." -ForegroundColor Cyan
Write-Host "================================================`n" -ForegroundColor Cyan

# Verificar se git está inicializado
if (-not (Test-Path ".git")) {
    Write-Host "❌ Erro: Git não inicializado! Execute setup-github.ps1 primeiro." -ForegroundColor Red
    exit 1
}

# Verificar se há mudanças não commitadas
$status = git status --porcelain
if ($status) {
    Write-Host "⚠️  Há mudanças não commitadas. Fazendo commit inicial..." -ForegroundColor Yellow
    git add .
    git commit -m "chore: initial project setup with complete architecture"
}

# Renomear branch para main
Write-Host "📌 Configurando branch main..." -ForegroundColor Yellow
git branch -M main

# Criar branch develop
Write-Host "📌 Criando branch develop..." -ForegroundColor Yellow
git checkout -b develop 2>$null
if ($LASTEXITCODE -ne 0) {
    git checkout develop
}

Write-Host "`n✅ Branch develop criada/atualizada!" -ForegroundColor Green

# Perguntar se quer criar branches temáticas
Write-Host "`n❓ Deseja criar branches temáticas separadas para cada módulo? (S/N)" -ForegroundColor Cyan
$createBranches = Read-Host

if ($createBranches -match '^[Ss]') {
    
    # Branch 1: Core Infrastructure
    Write-Host "`n🔧 [1/7] Criando branch: feature/core-infrastructure" -ForegroundColor Yellow
    git checkout -b feature/core-infrastructure
    
    git add cmd/ internal/config/ internal/storage/ internal/queue/ internal/secrets/ internal/blob/ 2>$null
    git commit -m "feat(core): implement database, Redis queue, and blob storage layers" 2>$null
    
    git add internal/meta/ 2>$null
    git commit -m "feat(meta): implement Meta API client with retry logic" 2>$null
    
    git add internal/service/semaphore.go 2>$null
    git commit -m "feat(service): implement semaphore for concurrency control" 2>$null
    
    Write-Host "✅ feature/core-infrastructure criada!" -ForegroundColor Green
    
    # Branch 2: Creatives Sync
    Write-Host "`n🖼️  [2/7] Criando branch: feature/creatives-sync" -ForegroundColor Yellow
    git checkout develop
    git checkout -b feature/creatives-sync
    
    git add internal/service/creatives_sync.go 2>$null
    git commit -m "feat(creatives): implement synchronous image creative upload" 2>$null
    
    git add internal/httpapi/ 2>$null
    git commit -m "feat(api): implement HTTP handlers and middleware" 2>$null
    
    git add cmd/api/ 2>$null
    git commit -m "feat(api): implement API server entrypoint" 2>$null
    
    Write-Host "✅ feature/creatives-sync criada!" -ForegroundColor Green
    
    # Branch 3: Creatives Async
    Write-Host "`n🎬 [3/7] Criando branch: feature/creatives-async" -ForegroundColor Yellow
    git checkout develop
    git checkout -b feature/creatives-async
    
    git add internal/service/jobs_async.go internal/service/worker_processor.go 2>$null
    git commit -m "feat(worker): implement async video creative job processing" 2>$null
    
    git add cmd/worker/ 2>$null
    git commit -m "feat(worker): implement worker entrypoint" 2>$null
    
    Write-Host "✅ feature/creatives-async criada!" -ForegroundColor Green
    
    # Branch 4: Campaign Management
    Write-Host "`n📣 [4/7] Criando branch: feature/campaign-management" -ForegroundColor Yellow
    git checkout develop
    git checkout -b feature/campaign-management
    
    git add internal/service/campaigns.go 2>$null
    git commit -m "feat(campaigns): implement campaign creation endpoint" 2>$null
    
    git add internal/service/adsets.go 2>$null
    git commit -m "feat(adsets): implement adset creation endpoint" 2>$null
    
    git add internal/service/ads.go 2>$null
    git commit -m "feat(ads): implement ad creation endpoint" 2>$null
    
    Write-Host "✅ feature/campaign-management criada!" -ForegroundColor Green
    
    # Branch 5: Docker Infrastructure
    Write-Host "`n🐳 [5/7] Criando branch: chore/docker-infrastructure" -ForegroundColor Yellow
    git checkout develop
    git checkout -b chore/docker-infrastructure
    
    git add Dockerfile docker-compose.yml Makefile 2>$null
    git commit -m "chore(docker): add Docker setup for local development" 2>$null
    
    Write-Host "✅ chore/docker-infrastructure criada!" -ForegroundColor Green
    
    # Branch 6: Documentation
    Write-Host "`n📚 [6/7] Criando branch: docs/complete-documentation" -ForegroundColor Yellow
    git checkout develop
    git checkout -b docs/complete-documentation
    
    git add README.md 2>$null
    git commit -m "docs(readme): add README with quick start guide" 2>$null
    
    git add explicacao_arquitetura.md 2>$null
    git commit -m "docs(architecture): add comprehensive architecture documentation" 2>$null
    
    git add .gitignore .env.example 2>$null
    git commit -m "chore: add .gitignore and .env.example" 2>$null
    
    Write-Host "✅ docs/complete-documentation criada!" -ForegroundColor Green
    
    # Branch 7: Git Workflow
    Write-Host "`n🔀 [7/7] Criando branch: chore/git-workflow-setup" -ForegroundColor Yellow
    git checkout develop
    git checkout -b chore/git-workflow-setup
    
    git add package.json commitlint.config.js .husky/ 2>$null
    git commit -m "chore(git): setup Husky and commitlint for conventional commits" 2>$null
    
    git add SETUP_GITHUB.md setup-github.ps1 run-git-setup-branches.ps1 2>$null
    git commit -m "docs(setup): add GitHub setup automation scripts" 2>$null
    
    Write-Host "✅ chore/git-workflow-setup criada!" -ForegroundColor Green
    
    # Voltar para develop
    git checkout develop
    
    Write-Host "`n✅ Todas as branches criadas com sucesso!" -ForegroundColor Green
    Write-Host "`n📋 Branches criadas:" -ForegroundColor Cyan
    git branch
    
} else {
    Write-Host "`n⏭️  Criação de branches pulada." -ForegroundColor Yellow
}

# Perguntar sobre push
Write-Host "`n❓ Deseja fazer push de todas as branches para o GitHub? (S/N)" -ForegroundColor Cyan
$doPush = Read-Host

if ($doPush -match '^[Ss]') {
    # Verificar se remote está configurado
    $remote = git remote -v | Select-String -Pattern "origin"
    
    if (-not $remote) {
        Write-Host "❌ Remote 'origin' não configurado! Configure primeiro com:" -ForegroundColor Red
        Write-Host "   git remote add origin git@github.com:pedrolucas201/creative-service.git" -ForegroundColor Yellow
        exit 1
    }
    
    Write-Host "`n🚀 Fazendo push de todas as branches..." -ForegroundColor Yellow
    
    git push -u origin main
    git push -u origin develop
    
    if ($createBranches -match '^[Ss]') {
        git push -u origin feature/core-infrastructure
        git push -u origin feature/creatives-sync
        git push -u origin feature/creatives-async
        git push -u origin feature/campaign-management
        git push -u origin chore/docker-infrastructure
        git push -u origin docs/complete-documentation
        git push -u origin chore/git-workflow-setup
    }
    
    Write-Host "`n✅ Push concluído!" -ForegroundColor Green
} else {
    Write-Host "`n⏭️  Push cancelado. Execute manualmente quando estiver pronto." -ForegroundColor Yellow
}

# Resumo final
Write-Host "`n================================================" -ForegroundColor Cyan
Write-Host "🎉 SETUP DE BRANCHES CONCLUÍDO!" -ForegroundColor Green
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "📋 Próximos Passos:" -ForegroundColor Yellow
Write-Host ""
Write-Host "1️⃣  Acesse o GitHub e crie Pull Requests para cada branch" -ForegroundColor White
Write-Host "2️⃣  Revise e faça merge das branches para develop" -ForegroundColor White
Write-Host "3️⃣  Quando develop estiver estável, merge para main" -ForegroundColor White
Write-Host "4️⃣  Crie uma tag de release: git tag -a v1.0.0 -m 'Initial release'" -ForegroundColor White
Write-Host ""
Write-Host "📖 Consulte SETUP_GITHUB.md para mais detalhes!" -ForegroundColor Cyan
Write-Host ""
