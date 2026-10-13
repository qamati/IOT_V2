$ErrorActionPreference = 'Stop'

Write-Host "=== 1. Login ==="
$login = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -ContentType 'application/json' -Body '{"email":"tenant@example.com","password":"Secret123!"}'
$token = $login.token
$headers = @{ Authorization = "Bearer $token" }

Write-Host "=== 2. Create Device ==="
$devName = "Flow_Transmitter_" + (Get-Random -Maximum 9999)
$devBody = @{ name = $devName; type = "DEFAULT"; label = "Industrial Transmitter" } | ConvertTo-Json
$devRes = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/devices' -Method POST -Headers $headers -ContentType 'application/json' -Body $devBody
Write-Host "Created Device: $($devRes.device.name) ID: $($devRes.device.id)"
Write-Host "Credentials Type: $($devRes.credentials.credentials_type) Value: $($devRes.credentials.credentials_value)"

Write-Host "=== 3. Fetch Device Credentials ==="
$devId = $devRes.device.id
$credRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/$devId/credentials" -Method GET -Headers $headers
Write-Host "Fetched Token: $($credRes.credentials_value)"

Write-Host "=== 4. Fetch SCADA Tags ==="
$tags = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/scada/tags' -Method GET -Headers $headers
Write-Host "Total SCADA Tags: $($tags.Count)"
foreach ($t in ($tags | Select-Object -First 3)) {
    Write-Host " - $($t.path) = $($t.value) $($t.eng_unit)"
}

Write-Host "=== 5. Fetch Web Console SPA ==="
$spa = Invoke-WebRequest -Uri 'http://localhost:8080/' -Method GET -UseBasicParsing
Write-Host "Web Console Status: $($spa.StatusCode)"
Write-Host "ALL VERIFICATION CHECKS PASSED!"
