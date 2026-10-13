# ============================================================================
# Stage 10 — SCADA, ISA-18.2 Alarms, SBO Controls & ThingsBoard Symbols E2E Test
# ============================================================================

$ErrorActionPreference = "Stop"
$BaseUrl = "http://127.0.0.1:8080/api/v1"

Write-Host "`n>>> [1/7] Authenticating tenant admin..." -ForegroundColor Cyan
$loginBody = @{
    email    = "tenant@example.com"
    password = "Secret123!"
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri "$BaseUrl/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$Token = $loginRes.token
if (-not $Token) {
    throw "Failed to obtain JWT auth token!"
}
$Headers = @{
    "Authorization" = "Bearer $Token"
    "Content-Type"  = "application/json"
}
Write-Host ">>> Authenticated successfully! Token received." -ForegroundColor Green

Write-Host "`n>>> [2/7] Testing SCADA Tag Namespace & Deadbands..." -ForegroundColor Cyan
$tags = Invoke-RestMethod -Uri "$BaseUrl/scada/tags" -Method Get -Headers $Headers
Write-Host ">>> Retrieved $($tags.Count) process tags in namespace:"
foreach ($t in $tags) {
    Write-Host "    - $($t.path): $($t.value) $($t.eng_unit) [Quality: $($t.quality)]" -ForegroundColor Gray
}

if ($tags.Count -lt 5) {
    throw "Expected at least 5 seeded SCADA tags!"
}

# Test deadband suppression
Write-Host "`n>>> Testing Deadband Suppression (Tank 101 Level)..." -ForegroundColor Cyan
$updateSubDeadband = @{
    path    = "WaterTreatment.Tank101.Level"
    value   = 76.6 # previous is 76.4, deadband is 0.5 -> delta is 0.2 < deadband
    quality = "GOOD"
} | ConvertTo-Json

$resDeadband = Invoke-RestMethod -Uri "$BaseUrl/scada/tags/update" -Method Post -Body $updateSubDeadband -Headers $Headers
if ($resDeadband.emitted -eq $true) {
    throw "Deadband failure: update should have been suppressed (emitted = false)!"
}
Write-Host ">>> SUCCESS: Deadband correctly suppressed telemetry emission (emitted = false)" -ForegroundColor Green

# Test update exceeding deadband + triggering ISA-18.2 High-High alarm (> 90%)
Write-Host "`n>>> Updating Tank Level to 94.5% (triggers ISA-18.2 HIGH_HIGH alarm)..." -ForegroundColor Cyan
$updateHiHi = @{
    path    = "WaterTreatment.Tank101.Level"
    value   = 94.5
    quality = "GOOD"
} | ConvertTo-Json

$resHiHi = Invoke-RestMethod -Uri "$BaseUrl/scada/tags/update" -Method Post -Body $updateHiHi -Headers $Headers
if ($resHiHi.emitted -ne $true) {
    throw "Tag update failed: update should have emitted = true!"
}
Write-Host ">>> SUCCESS: Tank Level updated to 94.5% and emitted = true" -ForegroundColor Green

Write-Host "`n>>> [3/7] Testing ISA-18.2 Process Alarm Lifecycle & EEMUA 191 KPIs..." -ForegroundColor Cyan
$alarms = Invoke-RestMethod -Uri "$BaseUrl/scada/alarms" -Method Get -Headers $Headers
Write-Host ">>> Retrieved $($alarms.Count) active ISA-18.2 process alarms:"
foreach ($a in $alarms) {
    Write-Host "    - [$($a.state)] $($a.tag_path) ($($a.alarm_type)): $($a.message)" -ForegroundColor Yellow
}

$hihiAlarm = $alarms | Where-Object { $_.tag_path -eq "WaterTreatment.Tank101.Level" -and $_.alarm_type -eq "HIGH_HIGH" }
if (-not $hihiAlarm) {
    throw "Expected to find triggered HIGH_HIGH alarm on Tank 101!"
}

# Test Alarm KPIs
$kpis = Invoke-RestMethod -Uri "$BaseUrl/scada/alarms/kpis" -Method Get -Headers $Headers
Write-Host ">>> EEMUA 191 KPIs: Total=$($kpis.total_alarms), Unack=$($kpis.unack_alarms), Shelved=$($kpis.shelved_alarms), Rate10m=$($kpis.alarm_rate_10min)" -ForegroundColor Green

# Test Acknowledge Alarm
Write-Host "`n>>> Testing Alarm Acknowledge (UNACK_ALARM -> ACKED_ALARM)..." -ForegroundColor Cyan
$ackRes = Invoke-RestMethod -Uri "$BaseUrl/scada/alarms/$($hihiAlarm.id)/ack" -Method Post -Headers $Headers
if ($ackRes.state -ne "ACKED_ALARM") {
    throw "Expected alarm state ACKED_ALARM, got $($ackRes.state)"
}
Write-Host ">>> SUCCESS: Alarm acknowledged by $($ackRes.ack_user) at $($ackRes.ack_ts)" -ForegroundColor Green

# Test Shelve Alarm
Write-Host "`n>>> Testing Alarm Shelve (ACKED_ALARM -> SHELVED)..." -ForegroundColor Cyan
$shelveBody = @{ duration_ms = 300000 } | ConvertTo-Json
$shelveRes = Invoke-RestMethod -Uri "$BaseUrl/scada/alarms/$($hihiAlarm.id)/shelve" -Method Post -Body $shelveBody -Headers $Headers
if ($shelveRes.state -ne "SHELVED") {
    throw "Expected alarm state SHELVED, got $($shelveRes.state)"
}
Write-Host ">>> SUCCESS: Alarm shelved until $($shelveRes.shelve_until)" -ForegroundColor Green

# Test Unshelve Alarm
Write-Host "`n>>> Testing Alarm Unshelve (SHELVED -> UNACK_ALARM)..." -ForegroundColor Cyan
$unshelveRes = Invoke-RestMethod -Uri "$BaseUrl/scada/alarms/$($hihiAlarm.id)/unshelve" -Method Post -Headers $Headers
if ($unshelveRes.state -ne "UNACK_ALARM") {
    throw "Expected alarm state UNACK_ALARM after unshelve, got $($unshelveRes.state)"
}
Write-Host ">>> SUCCESS: Alarm restored to UNACK_ALARM" -ForegroundColor Green

Write-Host "`n>>> [4/7] Testing Select-Before-Operate (SBO) Safe Industrial Control..." -ForegroundColor Cyan
# Phase 1: Select
$selectBody = @{
    tag_path      = "WaterTreatment.Pump101.Status"
    desired_value = 1
} | ConvertTo-Json

$selectRes = Invoke-RestMethod -Uri "$BaseUrl/scada/control/select" -Method Post -Body $selectBody -Headers $Headers
$selectId = $selectRes.select_id
if ($selectRes.status -ne "SELECTED") {
    throw "Expected control status SELECTED, got $($selectRes.status)"
}
Write-Host ">>> SUCCESS: Phase 1 Arm & Select granted token: $selectId" -ForegroundColor Green

# Phase 2: Operate
$operateBody = @{
    select_id = $selectId
} | ConvertTo-Json

$operateRes = Invoke-RestMethod -Uri "$BaseUrl/scada/control/operate" -Method Post -Body $operateBody -Headers $Headers
if ($operateRes.status -ne "OPERATED") {
    throw "Expected control status OPERATED, got $($operateRes.status)"
}
Write-Host ">>> SUCCESS: Phase 2 Operate executed at $($operateRes.operated_at) for tag $($operateRes.tag_path)" -ForegroundColor Green

Write-Host "`n>>> [5/7] Testing SBO Control Audit Log..." -ForegroundColor Cyan
$controlLogs = Invoke-RestMethod -Uri "$BaseUrl/scada/control/log?limit=10" -Method Get -Headers $Headers
Write-Host ">>> Retrieved $($controlLogs.Count) SBO audit entries:"
foreach ($l in $controlLogs) {
    Write-Host "    - [$($l.status)] Tag: $($l.tag_path), Desired: $($l.desired_value), Operator: $($l.operator)" -ForegroundColor Gray
}
if ($controlLogs.Count -lt 1) {
    throw "Expected at least 1 logged control action in audit journal!"
}
Write-Host ">>> SUCCESS: SBO audit trail confirmed." -ForegroundColor Green

Write-Host "`n>>> [6/7] Testing ThingsBoard SVG Symbols Catalog..." -ForegroundColor Cyan
$symbols = Invoke-RestMethod -Uri "$BaseUrl/scada/symbols" -Method Get -Headers $Headers
Write-Host ">>> Retrieved $($symbols.Count) ThingsBoard industrial SVG symbols:"
foreach ($s in $symbols) {
    Write-Host "    - [$($s.category)] $($s.name) ($($s.style)) [Tags: $($s.tags.Count), Props: $($s.properties.Count), Behaviors: $($s.behaviors.Count)]" -ForegroundColor Magenta
}
if ($symbols.Count -lt 4) {
    throw "Expected at least 4 seeded ThingsBoard SVG symbols!"
}

# Test Category Filtering
$pumps = Invoke-RestMethod -Uri "$BaseUrl/scada/symbols?category=Pumps" -Method Get -Headers $Headers
Write-Host ">>> Filter category 'Pumps': found $($pumps.Count) symbols." -ForegroundColor Green
if ($pumps.Count -lt 2) {
    throw "Expected at least 2 Pump symbols!"
}

# Test Custom Symbol Creation, Retrieval, and Deletion
Write-Host "`n>>> [7/7] Testing Custom SVG Symbol Lifecycle..." -ForegroundColor Cyan
$customSym = @{
    name        = "Custom Mixer Agitator"
    category    = "Mixers"
    style       = "standard"
    svg_content = "<svg viewBox='0 0 100 100'><circle cx='50' cy='50' r='40' fill='#06b6d4'/></svg>"
    tags        = @(
        @{ name = "blade"; role = "shape"; elementId = "blade1" }
    )
    properties  = @(
        @{ id = "rpm"; type = "number"; default = 500; label = "Agitator Speed" }
    )
    behaviors   = @(
        @{ id = "running"; kind = "value"; valueType = "BOOL"; label = "Running State" }
    )
    animations  = @()
} | ConvertTo-Json

$createdSym = Invoke-RestMethod -Uri "$BaseUrl/scada/symbols" -Method Post -Body $customSym -Headers $Headers
$symId = $createdSym.id
Write-Host ">>> Created custom symbol ID: $symId" -ForegroundColor Green

# Get by ID
$fetchedSym = Invoke-RestMethod -Uri "$BaseUrl/scada/symbols/$symId" -Method Get -Headers $Headers
if ($fetchedSym.name -ne "Custom Mixer Agitator") {
    throw "Fetched symbol name mismatch!"
}

# Delete symbol
$delRes = Invoke-RestMethod -Uri "$BaseUrl/scada/symbols/$symId" -Method Delete -Headers $Headers
Write-Host ">>> Deleted custom symbol successfully: $($delRes.deleted)" -ForegroundColor Green

Write-Host "`n============================================================================" -ForegroundColor Cyan
Write-Host ">>> ALL STAGE 10 SCADA E2E TESTS PASSED WITH 100% SUCCESS!" -ForegroundColor Green
Write-Host "============================================================================`n" -ForegroundColor Cyan
