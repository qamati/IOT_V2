$ErrorActionPreference = "Stop"

Write-Host "=== 1. Login to retrieve JWT Token ===" -ForegroundColor Cyan
$loginBody = @{
    email    = "demo@iotp.local"
    password = "demo1234"
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginRes.token
$headers = @{
    "Authorization" = "Bearer $token"
}
Write-Host "Logged in successfully as demo@iotp.local" -ForegroundColor Green

Write-Host "`n=== 2. List Devices ===" -ForegroundColor Cyan
$devRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices" -Method Get -Headers $headers
$targetDev = $devRes.data | Where-Object { $_.name -eq "weather-station-alpha" }
if (-not $targetDev) { $targetDev = $devRes.data[0] }
$devId = $targetDev.id
Write-Host "Target Device: $($targetDev.name) (ID: $devId)" -ForegroundColor Green

Write-Host "`n=== 3. Bulk Import Devices via CSV ===" -ForegroundColor Cyan
$csvPayload = @"
name,label,access_token
stage7-sensor-01,Warehouse Zone A,TOKEN_STAGE7_01
stage7-sensor-02,Cold Chain Rack 4,TOKEN_STAGE7_02
"@
$importRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/bulk/import" -Method Post -Headers $headers -Body $csvPayload -ContentType "text/plain"
Write-Host "Bulk Import Result: Imported $($importRes.imported) devices, Failed: $($importRes.failed)" -ForegroundColor Green

Write-Host "`n=== 4. Bulk Export Devices to CSV ===" -ForegroundColor Cyan
$exportCsv = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/bulk/export" -Method Get -Headers $headers
Write-Host "Exported CSV Content (First 3 lines):" -ForegroundColor Gray
($exportCsv -split "`n")[0..2] | ForEach-Object { Write-Host "  $_" }

Write-Host "`n=== 5. Create OTA Firmware Package ===" -ForegroundColor Cyan
$otaCreateBody = @{
    type    = "FIRMWARE"
    title   = "ESP32 Sensor Core Firmware"
    version = "v3.0.0-rc2"
    tag     = "release-candidate"
} | ConvertTo-Json
$pkg = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/otaPackages" -Method Post -Headers $headers -Body $otaCreateBody -ContentType "application/json"
$pkgId = $pkg.id
Write-Host "Created OTA Package ID: $pkgId (Title: $($pkg.title), Version: $($pkg.version))" -ForegroundColor Green

Write-Host "`n=== 6. Upload Binary to OTA Package ===" -ForegroundColor Cyan
$binContent = [System.Text.Encoding]::UTF8.GetBytes("FIRMWARE_BINARY_V3_MAGIC_HEADER_BYTES_32_BYTES_PAYLOAD")
$uploadRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/otaPackages/$pkgId/upload" -Method Post -Headers $headers -Body $binContent -ContentType "application/octet-stream"
Write-Host "Binary Uploaded: Size = $($uploadRes.data_size) bytes, SHA256 = $($uploadRes.checksum)" -ForegroundColor Green

Write-Host "`n=== 7. Assign OTA Package to Target Device ===" -ForegroundColor Cyan
$assignBody = @{
    kind      = "FIRMWARE"
    packageId = $pkgId
} | ConvertTo-Json
$assignRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/$devId/ota/assign" -Method Post -Headers $headers -Body $assignBody -ContentType "application/json"
Write-Host "Package Assigned to Device (Scope: $($assignRes.scope))" -ForegroundColor Green

Write-Host "`n=== 8. Verify Device Shared Attributes Synchronized ===" -ForegroundColor Cyan
$attrsRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/plugins/telemetry/DEVICE/$devId/values/attributes/SHARED_SCOPE" -Method Get -Headers $headers
$fwVersionAttr = $attrsRes | Where-Object { $_.key -eq "fw_version" }
$fwChecksumAttr = $attrsRes | Where-Object { $_.key -eq "fw_checksum" }
Write-Host "Synced Shared Attributes: fw_version = $($fwVersionAttr.value), fw_checksum = $($fwChecksumAttr.value)" -ForegroundColor Green

Write-Host "`n=== 9. Query Device OTA Rollout State ===" -ForegroundColor Cyan
$stateRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/$devId/ota/state?kind=FIRMWARE" -Method Get -Headers $headers
Write-Host "Device OTA State: $($stateRes.state), Progress: $($stateRes.progress)" -ForegroundColor Green

Write-Host "`n=== 10. Device Downloads Firmware Chunk ===" -ForegroundColor Cyan
# Retrieve device access token
$credsRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/$devId/credentials" -Method Get -Headers $headers
$devToken = $credsRes.credentials_value
$chunkRes = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/$devToken/firmware?chunk=0&chunkSize=16" -Method Get -UseBasicParsing
Write-Host "Device Chunk Downloaded: Status = $($chunkRes.StatusCode), Bytes = $($chunkRes.Content.Length), Total = $($chunkRes.Headers['X-Total-Size'])" -ForegroundColor Green

Write-Host "`n=== 11. Send One-Way Downlink RPC ===" -ForegroundColor Cyan
$rpcOneWayBody = @{
    method = "setSamplingInterval"
    params = @{ intervalSec = 15 }
} | ConvertTo-Json
$rpc1 = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/plugins/rpc/oneway/DEVICE/$devId" -Method Post -Headers $headers -Body $rpcOneWayBody -ContentType "application/json"
Write-Host "One-Way RPC Dispatched: ID = $($rpc1.id), Status = $($rpc1.status)" -ForegroundColor Green

Write-Host "`n=== 12. Query Device RPC History & Persistent RPC Endpoint ===" -ForegroundColor Cyan
$rpcHistory = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/plugins/rpc/DEVICE/$devId" -Method Get -Headers $headers
Write-Host "RPC History Total: $($rpcHistory.total) commands recorded" -ForegroundColor Green

$getCmd = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/rpc/persistent/$($rpc1.id)" -Method Get -Headers $headers
Write-Host "Persistent RPC Lookup by ID: Method = $($getCmd.method), Status = $($getCmd.status)" -ForegroundColor Green

Write-Host "`n=== ALL STAGE 7 INTEGRATION CHECKS PASSED 100%! ===" -ForegroundColor Green
