Write-Host "=== Stage 9: Edge Fleet, Runtime & Store-and-Forward Verification ===" -ForegroundColor Yellow

# 1. Login
$loginBody = @{
    email    = "demo@iotp.local"
    password = "demo1234"
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginRes.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Authenticated as Demo Admin successfully." -ForegroundColor Green

# 2. List Edge Nodes
Write-Host "`n=== 1. List Edge Fleet Nodes in PostgreSQL ===" -ForegroundColor Cyan
$edges = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge" -Method Get -Headers $headers
Write-Host "Found $($edges.Count) edge nodes in PostgreSQL:" -ForegroundColor Green
$edges | ForEach-Object { Write-Host " - [$($_.status)] $($_.name) (Key: $($_.edge_key), Model: $($_.hardware_model))" }

# 3. Register New Edge Node in PostgreSQL
Write-Host "`n=== 2. Register New Edge Node in PostgreSQL ===" -ForegroundColor Cyan
$suffix = (Get-Random -Minimum 1000 -Maximum 9999)
$edgeName = "edge-station-hvac-$suffix"
$edgeRegBody = @{
    name             = $edgeName
    hardware_model   = "Advantech UNO-2271G (Industrial x86)"
    firmware_version = "v1.4.0-edge"
} | ConvertTo-Json

$newEdge = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge" -Method Post -Headers $headers -Body $edgeRegBody -ContentType "application/json"
$edgeId = $newEdge.id
$edgeKey = $newEdge.edge_key
Write-Host "Registered Edge ID: $edgeId (Name: $($newEdge.name), Key: $edgeKey)" -ForegroundColor Green

# 4. Create Test Device in Registry
Write-Host "`n=== 3. Create Test Sensor Device in Registry ===" -ForegroundColor Cyan
$devBody = @{
    name  = "stage9-pressure-transmitter-$suffix"
    label = "HVAC Pressure Sensor"
} | ConvertTo-Json

$newDev = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices" -Method Post -Headers $headers -Body $devBody -ContentType "application/json"
$devId = $newDev.device.id
Write-Host "Created Device ID: $devId (Name: $($newDev.device.name))" -ForegroundColor Green

# 5. Assign Device to Edge Node
Write-Host "`n=== 4. Assign Device to Edge Node ===" -ForegroundColor Cyan
$assignBody = @{
    device_ids = @($devId)
} | ConvertTo-Json

$assignRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/assignments" -Method Post -Headers $headers -Body $assignBody -ContentType "application/json"
Write-Host "Assigned Device: Success = $($assignRes.success), Count = $($assignRes.assigned_count)" -ForegroundColor Green

$assignedDevs = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/assignments" -Method Get -Headers $headers
Write-Host "Retrieved Edge Assignments: $($assignedDevs | ConvertTo-Json -Compress)" -ForegroundColor Green
if ($assignedDevs -notcontains $devId) {
    Write-Error "Expected device $devId to be in edge assignments!"
    exit 1
}

# 6. Test Heartbeat Ping from Edge
Write-Host "`n=== 5. Edge Heartbeat Ping (POST /api/v1/edge/heartbeat) ===" -ForegroundColor Cyan
$hbRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/heartbeat" -Method Post -Headers @{ "X-Edge-Key" = $edgeKey }
Write-Host "Heartbeat Response: Status = $($hbRes.status), Server Time = $($hbRes.server_time)" -ForegroundColor Green

# 7. Test Edge Configuration Pull
Write-Host "`n=== 6. Edge Configuration Pull (GET /api/v1/edge/configuration) ===" -ForegroundColor Cyan
$cfgRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/configuration" -Method Get -Headers @{ "X-Edge-Key" = $edgeKey }
Write-Host "Pulled Edge Configuration: Name = $($cfgRes.name), Assigned Devices = $($cfgRes.assigned_devices.Count)" -ForegroundColor Green

# 8. Store-and-Forward Sync Batch (L0 Alarms + L1 Telemetry + L2 Logs)
Write-Host "`n=== 7. Ingest Multi-Lane Store-and-Forward Batch ===" -ForegroundColor Cyan
$now = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$syncPayload = @{
    edge_id   = $edgeId
    sequence  = 1
    timestamp = $now
    lane_l0   = @(
        @{
            entity_id = $devId
            timestamp = $now
            type      = "ALARM"
            payload   = @{
                type     = "CRITICAL_PRESSURE_BURST"
                severity = "CRITICAL"
                details  = @{
                    message  = "Autonomous edge safety triggered: 78.5 bar burst detected"
                    pressure = 78.5
                }
            }
        }
    )
    lane_l1   = @(
        @{
            entity_id = $devId
            timestamp = $now
            type      = "TELEMETRY"
            payload   = @(
                @{ key = "line_pressure_bar"; ts = $now; dbl_v = 78.5 }
                @{ key = "feed_flow_lpm"; ts = $now; dbl_v = 142.3 }
            )
        }
    )
    lane_l2   = @(
        @{
            entity_id = $edgeId
            timestamp = $now
            type      = "LOG"
            payload   = @{ event = "battery_backup_engaged"; state = "OK" }
        }
    )
} | ConvertTo-Json -Depth 6

$syncRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/sync" -Method Post -Headers @{ "X-Edge-Key" = $edgeKey } -Body $syncPayload -ContentType "application/json"
Write-Host "Sync ACK: Sequence = $($syncRes.ack_sequence), Processed Items = $($syncRes.processed_items)" -ForegroundColor Green

if ($syncRes.ack_sequence -ne 1) {
    Write-Error "Expected ACK sequence 1, got $($syncRes.ack_sequence)"
    exit 1
}

# 9. Verify Deduplication Idempotency (Replay Same Sequence)
Write-Host "`n=== 8. Verify Idempotent Deduplication Cursor ===" -ForegroundColor Cyan
$dupSyncRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/sync" -Method Post -Headers @{ "X-Edge-Key" = $edgeKey } -Body $syncPayload -ContentType "application/json"
Write-Host "Duplicate Sequence ACK: Sequence = $($dupSyncRes.ack_sequence), Processed = $($dupSyncRes.processed_items)" -ForegroundColor Green

if ($dupSyncRes.processed_items -ne 0) {
    Write-Error "Duplicate batch was not deduplicated! Processed items: $($dupSyncRes.processed_items)"
    exit 1
}

# 10. Verify Backfilled Telemetry in TimescaleDB
Write-Host "`n=== 9. Verify Telemetry Persisted in TimescaleDB ===" -ForegroundColor Cyan
$tsRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/telemetry/DEVICE/$devId/values/latest" -Method Get -Headers $headers
$pressureVal = $tsRes.line_pressure_bar[0].value
$flowVal = $tsRes.feed_flow_lpm[0].value
Write-Host "TimescaleDB Telemetry: line_pressure_bar = $pressureVal bar, feed_flow_lpm = $flowVal lpm" -ForegroundColor Green

if ($pressureVal -ne 78.5) {
    Write-Error "Expected line_pressure_bar = 78.5, got $pressureVal"
    exit 1
}

# 11. Verify Sync Journal in PostgreSQL
Write-Host "`n=== 10. Verify Sync Journal in PostgreSQL ===" -ForegroundColor Cyan
$journalRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/journal" -Method Get -Headers $headers
Write-Host "Sync Journal Entries Count: $($journalRes.Count)" -ForegroundColor Green
$entry = $journalRes[0]
Write-Host "Latest Journal: Seq #$($entry.sequence), L0: $($entry.lane_l0_count), L1: $($entry.lane_l1_count), L2: $($entry.lane_l2_count), Status: $($entry.status)" -ForegroundColor Green

if ($entry.sequence -ne 1) {
    Write-Error "Expected latest journal entry sequence 1, got $($entry.sequence)"
    exit 1
}

# 12. Test Remote Command Execution
Write-Host "`n=== 11. Test Remote Commands Dispatch ===" -ForegroundColor Cyan
$cmdBody = @{ command = "REBOOT" } | ConvertTo-Json
$cmdRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/commands" -Method Post -Headers $headers -Body $cmdBody -ContentType "application/json"
Write-Host "Command Response: Command = $($cmdRes.command), Success = $($cmdRes.success), Message = $($cmdRes.message)" -ForegroundColor Green

$cmdSyncBody = @{ command = "FORCE_SYNC" } | ConvertTo-Json
$cmdSyncRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/commands" -Method Post -Headers $headers -Body $cmdSyncBody -ContentType "application/json"
Write-Host "Command Response: Command = $($cmdSyncRes.command), Success = $($cmdSyncRes.success), Message = $($cmdSyncRes.message)" -ForegroundColor Green

# 13. Test Unassign Device
Write-Host "`n=== 12. Test Unassign Device ===" -ForegroundColor Cyan
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/assignments/$devId" -Method Delete -Headers $headers
$remainingDevs = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/edge/$edgeId/assignments" -Method Get -Headers $headers
Write-Host "Remaining Device Assignments after unassign: $($remainingDevs.Count)" -ForegroundColor Green

if ($remainingDevs.Count -ne 0) {
    Write-Error "Expected 0 assigned devices after unassign, got $($remainingDevs.Count)"
    exit 1
}

Write-Host "`n=========================================================" -ForegroundColor Yellow
Write-Host "  STAGE 9: ALL VERIFICATION CHECKS PASSED 100% GREEN!    " -ForegroundColor Green
Write-Host "=========================================================" -ForegroundColor Yellow
