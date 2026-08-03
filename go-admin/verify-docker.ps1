<#
.SYNOPSIS
  go-admin docker-compose 一键验证：build -> up -> healthcheck -> register/login

.USAGE
  cd go-admin; .\verify-docker.ps1
#>
$ErrorActionPreference = 'Stop'

Write-Host '==> 1/5 docker compose build'
docker compose build
if ($LASTEXITCODE -ne 0) { throw 'docker compose build failed' }

Write-Host '==> 2/5 docker compose up -d'
docker compose up -d
if ($LASTEXITCODE -ne 0) { throw 'docker compose up failed' }

Write-Host '==> 3/5 wait for go-admin healthy (max 150s)'
$status = 'starting'
$deadline = (Get-Date).AddSeconds(150)
do {
    $status = docker inspect --format '{{.State.Health.Status}}' go-admin 2>$null
    if ($status -eq 'healthy') { break }
    Start-Sleep -Seconds 3
} while ((Get-Date) -lt $deadline)
if ($status -ne 'healthy') {
    Write-Host 'go-admin did not become healthy within 150s. Last logs:'
    docker compose logs go-admin --tail 100
    throw 'go-admin healthcheck timeout'
}
Write-Host "go-admin is healthy."

$ts = Get-Date -Format 'yyyyMMddHHmmss'
$user = "docker_$ts"
$body = @{ username = $user; password = '123456' } | ConvertTo-Json

Write-Host "==> 4/5 curl POST /api/user/register (user=$user)"
$reg = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8080/api/user/register' -ContentType 'application/json' -Body $body
$reg | ConvertTo-Json -Depth 5

Write-Host '==> 5/5 curl POST /api/user/login'
$login = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8080/api/user/login' -ContentType 'application/json' -Body $body
$login | ConvertTo-Json -Depth 5

if ($reg.code -eq 0 -and $login.code -eq 0) {
    Write-Host ''
    Write-Host "SUCCESS: register + login OK for user '$user'"
} else {
    throw "API verification failed: register.code=$($reg.code) login.code=$($login.code)"
}
