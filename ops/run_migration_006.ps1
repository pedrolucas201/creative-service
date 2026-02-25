param(
    [string]$DatabaseUrl = $env:DATABASE_URL,
    [string]$MigrationFile = "internal/storage/migrations/006_business_managers.sql"
)

$ErrorActionPreference = "Stop"

if (-not $DatabaseUrl) {
    Write-Error "DATABASE_URL nao definido. Passe -DatabaseUrl ou exporte DATABASE_URL."
}

if (-not (Test-Path $MigrationFile)) {
    Write-Error "Migration file nao encontrado: $MigrationFile"
}

if (-not (Get-Command psql -ErrorAction SilentlyContinue)) {
    Write-Error "psql nao encontrado no PATH. Rode essa migration via Cloud SQL Studio ou instale psql."
}

Write-Host "Aplicando migration: $MigrationFile" -ForegroundColor Cyan
psql "$DatabaseUrl" -v ON_ERROR_STOP=1 -f "$MigrationFile"

if ($LASTEXITCODE -ne 0) {
    Write-Error "Falha ao aplicar migration 006."
}

Write-Host "Migration 006 aplicada com sucesso." -ForegroundColor Green
