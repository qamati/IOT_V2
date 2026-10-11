# iotp Walking Skeleton Smoke Test
$ErrorActionPreference = "Stop"

$baseUrl = "http://localhost:8080"
Write-Host "=== 1. Logging in as seeded demo user (demo@iotp.local) ==="
$loginBody = @{
    email    = "demo@iotp.local"
    password = "demo1234"
} | ConvertTo-Json

$loginResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginResp.token
if (-not $token) {
    Write-Error "Login failed: no token received"
}
Write-Host "Success! JWT Token received." -ForegroundColor Green

Write-Host "`n=== 2. Creating a new device ==="
$devNum = Get-Random -Minimum 1000 -Maximum 9999
$devBody = @{
    name  = "Smart-Meter-$devNum"
    label = "Main Building Electrical Meter"
} | ConvertTo-Json

$headers = @{
    Authorization = "Bearer $token"
}

$devResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/devices" -Method Post -Body $devBody -Headers $headers -ContentType "application/json"
$deviceId = $devResp.device.id
$deviceToken = $devResp.credentials.credentials_value
Write-Host "Device created! ID: $deviceId" -ForegroundColor Green
Write-Host "Device Access Token: $deviceToken" -ForegroundColor Green

Write-Host "`n=== 3. Uploading device telemetry via wire-compatible endpoint ==="
$telemetryBody = @{
    power       = 342.5
    voltage     = 230.1
    current     = 1.48
    operational = $true
} | ConvertTo-Json

Invoke-RestMethod -Uri "$baseUrl/api/v1/$deviceToken/telemetry" -Method Post -Body $telemetryBody -ContentType "application/json"
Write-Host "Telemetry published successfully!" -ForegroundColor Green

Start-Sleep -Milliseconds 100

Write-Host "`n=== 4. Querying latest telemetry via REST API ==="
$latestResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/telemetry/DEVICE/$deviceId/values/latest" -Headers $headers
Write-Host "Latest values returned:" -ForegroundColor Cyan
$latestResp | ConvertTo-Json | Write-Host

Write-Host "`n=== 5. Querying time-series telemetry via REST API ==="
$tsResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/telemetry/DEVICE/$deviceId/values/timeseries?keys=power,voltage" -Headers $headers
Write-Host "Timeseries points returned:" -ForegroundColor Cyan
$tsResp | ConvertTo-Json | Write-Host

Write-Host "`n==============================================="
Write-Host " SMOKE TEST PASSED: ALL SKELETON GATES GREEN!" -ForegroundColor Green
Write-Host "==============================================="
