param(
    [Parameter(Mandatory = $true)]
    [string]$BaseUrl,
    [Parameter(Mandatory = $true)]
    [string]$BMUUID
)

$ErrorActionPreference = "Stop"

function Invoke-JsonGet {
    param([string]$Url)
    Write-Host "GET $Url" -ForegroundColor Cyan
    $resp = curl.exe --silent --show-error --max-time 20 "$Url"
    if ($LASTEXITCODE -ne 0) {
        throw "Falha em GET $Url"
    }
    $resp
}

$health = Invoke-JsonGet "$BaseUrl/v1/health"
Write-Host "health: $health"

$clients = Invoke-JsonGet "$BaseUrl/v1/clients"
Write-Host "clients: $clients"

$bm = Invoke-JsonGet "$BaseUrl/v1/bm/$BMUUID/config"
Write-Host "bm config: $bm"

Write-Host "Smoke test finalizado." -ForegroundColor Green
