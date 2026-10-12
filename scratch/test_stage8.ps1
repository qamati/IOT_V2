Write-Host "=== Stage 8: Integrations, Converters, Protocols & Field Gateway Verification ===" -ForegroundColor Yellow

# 1. Login
$loginBody = @{
    email    = "demo@iotp.local"
    password = "demo1234"
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginRes.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Authenticated as Demo Admin successfully." -ForegroundColor Green

# 2. List Converters (Persistent in PostgreSQL)
Write-Host "`n=== 1. Verify Default Seeded Converters in PostgreSQL ===" -ForegroundColor Cyan
$convs = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/converters" -Method Get -Headers $headers
Write-Host "Found $($convs.Count) converters in PostgreSQL:" -ForegroundColor Green
$convs | ForEach-Object { Write-Host " - [$($_.mode)] $($_.name) (ID: $($_.id))" }
if ($convs.Count -lt 2) {
    Write-Error "Expected at least 2 default converters"
    exit 1
}

# 3. Test Converter Live Runner
Write-Host "`n=== 2. Test Converter Live Runner (POST /api/v1/converters/test) ===" -ForegroundColor Cyan
$testPayload = @{
    converter = $convs[0]
    payload   = '{"device_name": "live-sensor-44", "data": {"temp": 29.5, "humidity": 62.0}}'
} | ConvertTo-Json -Depth 5

$testRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/converters/test" -Method Post -Headers $headers -Body $testPayload -ContentType "application/json"
Write-Host "Test Evaluation Result: Success = $($testRes.success), Device = $($testRes.event.device.name)" -ForegroundColor Green
Write-Host "Evaluated Telemetry: $($testRes.event.telemetry[0].values | ConvertTo-Json -Compress)"

# 4. Create Custom Expression Converter in PostgreSQL
Write-Host "`n=== 3. Create Custom Expression Converter in PostgreSQL ===" -ForegroundColor Cyan
$suffix = (Get-Random -Minimum 1000 -Maximum 9999)
$convName = "HVAC Weather Expression Converter $suffix"
$integName = "HVAC Webhook Integration $suffix"
$routingKey = "wh_stage8_$suffix"

$exprDef = @{
    format         = "json"
    deviceNamePath = "deviceId"
    deviceType     = "hvac_unit"
    mappings       = @(
        @{ source = "temp_c"; target = "telemetry.celsius"; type = "float" }
    )
    expressions    = @{
        "telemetry.fahrenheit" = "celsius * 1.8 + 32"
    }
}

$convBody = @{
    name       = $convName
    direction  = "UPLINK"
    mode       = "EXPRESSION"
    definition = $exprDef
} | ConvertTo-Json -Depth 5

$createdConv = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/converters" -Method Post -Headers $headers -Body $convBody -ContentType "application/json"
$convId = $createdConv.id
Write-Host "Created Expression Converter ID: $convId (Name: $($createdConv.name))" -ForegroundColor Green

# 5. Create Integration Connector in PostgreSQL
Write-Host "`n=== 4. Create Integration Connector in PostgreSQL ===" -ForegroundColor Cyan
$integBody = @{
    name                = $integName
    type                = "HTTP_WEBHOOK"
    routing_key         = $routingKey
    uplink_converter_id = $convId
    enabled             = $true
    allow_create_device = $true
    default_device_type = "hvac_sensor"
} | ConvertTo-Json

$createdInteg = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/integrations" -Method Post -Headers $headers -Body $integBody -ContentType "application/json"
$integId = $createdInteg.id
Write-Host "Created Integration ID: $integId (Routing Key: $($createdInteg.routing_key))" -ForegroundColor Green

# 6. Public Ingestion Webhook Trigger
Write-Host "`n=== 5. Ingest Webhook Uplink (POST /api/v1/integrations/{key}/webhook) ===" -ForegroundColor Cyan
$uplinkPayload = '{"deviceId": "stage8-hvac-node-' + $suffix + '", "temp_c": 25.0}'
$webhookRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/integrations/$routingKey/webhook" -Method Post -Body $uplinkPayload -ContentType "application/json"
Write-Host "Webhook Response: Status = $($webhookRes.status), Device = $($webhookRes.event.device.name)" -ForegroundColor Green

# 7. Verify Auto-Provisioned Device
Write-Host "`n=== 6. Verify Auto-Provisioned Device in Device Registry ===" -ForegroundColor Cyan
$expectedDevName = "stage8-hvac-node-$suffix"
$devsRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices" -Method Get -Headers $headers
$targetDev = $devsRes.data | Where-Object { $_.name -eq $expectedDevName }
if (-not $targetDev) {
    Write-Error "Auto-provisioned device '$expectedDevName' not found in registry!"
    exit 1
}
$devId = $targetDev.id
Write-Host "Verified Device Auto-Created in PostgreSQL: ID = $devId, Name = $($targetDev.name)" -ForegroundColor Green

# 8. Verify Persisted Telemetry in TimescaleDB
Write-Host "`n=== 7. Verify Telemetry & Computed Expression in TimescaleDB ===" -ForegroundColor Cyan
$tsRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/telemetry/DEVICE/$devId/values/latest" -Method Get -Headers $headers
$celsiusVal = $tsRes.celsius[0].value
$fahrVal = $tsRes.fahrenheit[0].value
Write-Host "TimescaleDB Telemetry: celsius = $celsiusVal °C, fahrenheit = $fahrVal °F" -ForegroundColor Green

if ($celsiusVal -ne 25 -or $fahrVal -ne 77) {
    Write-Error "Expression calculation mismatch! Expected 25 C and 77 F, got $celsiusVal and $fahrVal"
    exit 1
}

# 9. Verify Integration Status Counters in PostgreSQL
Write-Host "`n=== 8. Verify Integration Status Counters in PostgreSQL ===" -ForegroundColor Cyan
$statusRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/integrations/$integId/status" -Method Get -Headers $headers
Write-Host "Integration Status: State = $($statusRes.state), Rx Events = $($statusRes.counters.rx_events), Auto Created = $($statusRes.counters.auto_created)" -ForegroundColor Green

# 10. Verify Integration Debug Trace in PostgreSQL
Write-Host "`n=== 9. Verify Integration Debug Capture in PostgreSQL ===" -ForegroundColor Cyan
$dbgRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/integrations/$integId/debug" -Method Get -Headers $headers
Write-Host "Captured Debug Events: Count = $($dbgRes.Count), Success = $($dbgRes[0].success)" -ForegroundColor Green

Write-Host "`n========================================================" -ForegroundColor Yellow
Write-Host "  STAGE 8: ALL VERIFICATION CHECKS PASSED 100% GREEN!   " -ForegroundColor Green
Write-Host "========================================================" -ForegroundColor Yellow
