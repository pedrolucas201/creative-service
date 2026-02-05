# Carrega variáveis do .env
Get-Content .env | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)$') {
        $name = $matches[1]
        $value = $matches[2]
        [System.Environment]::SetEnvironmentVariable($name, $value, 'Process')
        Write-Host "✓ $name carregado"
    }
}

Write-Host "`n🚀 Iniciando API..." -ForegroundColor Green
go run cmd/api/main.go
