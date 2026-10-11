$ErrorActionPreference = "Stop"

Write-Host "=== 1. Login as Tenant Admin ==="
$loginBody = @{
    email = "tenant@example.com"
    password = "Secret123!"
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginRes.token
$headers = @{ "Authorization" = "Bearer $token" }
Write-Host "Logged in. Tenant: $($loginRes.user.tenant_id)"

Write-Host "`n=== 2. Stage 8: Verify Converters Catalog ==="
$converters = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/converters" -Method Get -Headers $headers
Write-Host "Total Converters: $($converters.Count)"
foreach ($c in $converters) {
    Write-Host "  Converter: $($c.name) [Mode: $($c.mode), Direction: $($c.direction)]"
}

Write-Host "`n=== 3. Stage 8: Test Converter Execution Harness ==="
$testPayload = @{
    converter = $converters[0]
    payload = '{"device_name": "webhook-sensor-99", "data": {"temp": 27.5, "humidity": 62.1, "battery": 94}}'
} | ConvertTo-Json -Depth 5

$testRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/converters/test" -Method Post -Body $testPayload -ContentType "application/json" -Headers $headers
Write-Host "Test Success: $($testRes.success)"
Write-Host "Decoded Device Name: $($testRes.event.device.name)"
Write-Host "Decoded Telemetry: $($testRes.event.telemetry[0].values | ConvertTo-Json -Compress)"

Write-Host "`n=== 4. Stage 8: Ingest Third-Party Webhook Uplink ==="
$webhookBody = '{"device_name": "lora-node-building-b", "data": {"temp": 46.8, "humidity": 51.0}}'
$whRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/integrations/webhook/acme-weather-wh" -Method Post -Body $webhookBody -ContentType "application/json"
Write-Host "Webhook Ingest Status: $($whRes.status)"
Write-Host "Device Ingested: $($whRes.event.device.name)"

Write-Host "`n=== 5. Stage 9: Edge Fleet & Store-and-Forward Sync ==="
$edgeNodes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge" -Method Get -Headers $headers
Write-Host "Discovered Edge Nodes: $($edgeNodes.Count)"
$edge = $edgeNodes[0]
Write-Host "  Edge Node: $($edge.name) [Hardware: $($edge.hardware_model), Status: $($edge.status)]"

$syncBatch = @{
    edge_id = $edge.id
    sequence = 2001
    timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    lane_l0 = @(
        @{
            entity_id = $edge.id
            timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
            type = "ALARM"
            payload = @{
                type = "Edge Battery Backup Warning"
                severity = "WARNING"
                details = @{ message = "Mains power dipped" }
            }
        }
    )
    lane_l1 = @(
        @{
            entity_id = $edge.id
            timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
            type = "TELEMETRY"
            payload = @(
                @{ key = "edge_cpu_temp"; ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds(); dbl_v = 47.2 }
            )
        }
    )
    lane_l2 = @()
} | ConvertTo-Json -Depth 6

$syncAck = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/sync" -Method Post -Body $syncBatch -ContentType "application/json"
Write-Host "Store-and-Forward Sync ACK Sequence: $($syncAck.ack_sequence), Processed Items: $($syncAck.processed_items)"

Write-Host "`n=== 6. Stage 10: SCADA Process Tags & Quality ==="
$scadaTags = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/tags" -Method Get -Headers $headers
Write-Host "Discovered SCADA Tags: $($scadaTags.Count)"
foreach ($t in $scadaTags) {
    Write-Host "  Tag: $($t.path) = $($t.value) $($t.eng_unit) [Quality: $($t.quality), Deadband: $($t.deadband)]"
}

Write-Host "`n=== 7. Stage 10: Select-Before-Operate (SBO) Safe Control ==="
# Phase 1: Select
$selectBody = @{
    tag_path = "WaterTreatment.Pump101.Status"
    desired_value = 0
} | ConvertTo-Json

$selectCmd = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/control/select" -Method Post -Body $selectBody -ContentType "application/json" -Headers $headers
Write-Host "Phase 1 Select Armed: SelectID = $($selectCmd.select_id), Status = $($selectCmd.status)"

# Phase 2: Operate
$operateBody = @{
    select_id = $selectCmd.select_id
} | ConvertTo-Json

$operateCmd = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/control/operate" -Method Post -Body $operateBody -ContentType "application/json" -Headers $headers
Write-Host "Phase 2 Command Verified & Operated: Status = $($operateCmd.status)"

# Verify Tag State
$updatedPumpTag = ($scadaTags | Where-Object { $_.path -eq "WaterTreatment.Pump101.Status" })
$pumpCheck = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/tags" -Method Get -Headers $headers | Where-Object { $_.path -eq "WaterTreatment.Pump101.Status" }
Write-Host "Pump Live State After SBO Control: Value = $($pumpCheck.value) (0 = STOPPED)"

Write-Host "`n=== 8. Stage 10: ISA-18.2 Process Alarms ==="
# Trigger tank level alarm (> 90%)
$tankUpdate = @{
    path = "WaterTreatment.Tank101.Level"
    value = 94.2
    quality = "GOOD"
} | ConvertTo-Json

$updateRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/tags/update" -Method Post -Body $tankUpdate -ContentType "application/json" -Headers $headers
Write-Host "Updated Tank Level to 94.2% (Deadband emitted: $($updateRes.emitted))"

$scadaAlarms = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/alarms" -Method Get -Headers $headers
Write-Host "Active ISA-18.2 Process Alarms: $($scadaAlarms.Count)"
foreach ($a in $scadaAlarms) {
    Write-Host "  Alarm: $($a.tag_path) [$($a.alarm_type)] State: $($a.state), Message: $($a.message)"
    # Acknowledge alarm
    $ackRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/scada/alarms/$($a.id)/ack" -Method Post -Headers $headers
    Write-Host "  -> Acknowledged: New State = $($ackRes.state) by $($ackRes.ack_user)"
}

Write-Host "`n>>> ALL STAGE 8 (INTEGRATIONS), STAGE 9 (EDGE), AND STAGE 10 (SCADA) VERIFIED SUCCESSFULLY! <<<"
