# 🚀 Script Automatizado - Setup GitHub Profissional
# Execute este script após revisar o SETUP_GITHUB.md

Write-Host "🎯 Creative Service - Setup GitHub Profissional" -ForegroundColor Cyan
Write-Host "================================================`n" -ForegroundColor Cyan

# Verificar se está no diretório correto
if (-not (Test-Path "go.mod")) {
    Write-Host "❌ Erro: Execute este script no diretório raiz do projeto!" -ForegroundColor Red
    exit 1
}

# Etapa 1: Instalar Husky
Write-Host "📦 Etapa 1: Instalando Husky e Commitlint..." -ForegroundColor Yellow
npm install
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Erro ao instalar dependências npm" -ForegroundColor Red
    exit 1
}

# Etapa 2: Configurar Husky
Write-Host "`n🔧 Etapa 2: Configurando Husky..." -ForegroundColor Yellow
npx husky install

# Criar hook manualmente (Windows-friendly)
$huskyDir = ".husky"
if (-not (Test-Path $huskyDir)) {
    New-Item -ItemType Directory -Path $huskyDir | Out-Null
}

$commitMsgHook = @"
#!/usr/bin/env sh
. "`$(dirname -- "`$0")/_/husky.sh"

npx --no -- commitlint --edit `$1
"@

$commitMsgHook | Out-File -FilePath "$huskyDir/commit-msg" -Encoding utf8 -NoNewline

Write-Host "✅ Husky configurado!" -ForegroundColor Green

# Etapa 3: Testar Commitlint
Write-Host "`n🧪 Etapa 3: Testando Commitlint..." -ForegroundColor Yellow
echo "feat: teste" | npx commitlint
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Commitlint funcionando!" -ForegroundColor Green
} else {
    Write-Host "⚠️  Commitlint com problemas, mas pode funcionar no commit real" -ForegroundColor Yellow
}

# Etapa 4: Git Init
Write-Host "`n📝 Etapa 4: Inicializando Git..." -ForegroundColor Yellow

if (Test-Path ".git") {
    Write-Host "⚠️  Repositório Git já existe. Pulando git init..." -ForegroundColor Yellow
} else {
    git init
    Write-Host "✅ Git inicializado!" -ForegroundColor Green
}

# Etapa 5: Criar .env se não existir
if (-not (Test-Path ".env")) {
    Write-Host "`n📄 Criando .env a partir do .env.example..." -ForegroundColor Yellow
    Copy-Item ".env.example" ".env"
    Write-Host "⚠️  IMPORTANTE: Configure suas variáveis de ambiente no arquivo .env!" -ForegroundColor Yellow
}

# Etapa 6: Perguntar URL do repositório
Write-Host "`n🌐 Etapa 5: Configurar Remote do GitHub" -ForegroundColor Yellow
Write-Host "Digite a URL do seu repositório GitHub (ex: https://github.com/usuario/creative-service.git)" -ForegroundColor Cyan
Write-Host "Ou deixe em branco para pular esta etapa:" -ForegroundColor Cyan
$repoUrl = Read-Host "URL"

if ($repoUrl -ne "") {
    # Verificar se remote já existe
    $remoteExists = git remote | Select-String -Pattern "origin"
    
    if ($remoteExists) {
        Write-Host "⚠️  Remote 'origin' já existe. Atualizando URL..." -ForegroundColor Yellow
        git remote set-url origin $repoUrl
    } else {
        git remote add origin $repoUrl
    }
    Write-Host "✅ Remote configurado: $repoUrl" -ForegroundColor Green
} else {
    Write-Host "⏭️  Remote não configurado. Configure manualmente depois." -ForegroundColor Yellow
}

# Resumo Final
Write-Host "`n" -NoNewline
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "✅ SETUP CONCLUÍDO!" -ForegroundColor Green
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "📋 Próximos Passos:" -ForegroundColor Yellow
Write-Host ""
Write-Host "1️⃣  Configure suas variáveis em .env (se ainda não fez)" -ForegroundColor White
Write-Host "2️⃣  Revise o guia completo: SETUP_GITHUB.md" -ForegroundColor White
Write-Host "3️⃣  Execute o script de branches: .\run-git-setup-branches.ps1" -ForegroundColor White
Write-Host ""
Write-Host "Ou siga o fluxo manual descrito no SETUP_GITHUB.md" -ForegroundColor Cyan
Write-Host ""
