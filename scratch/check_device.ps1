$login = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -ContentType 'application/json' -Body '{"email":"tenant@example.com","password":"Secret123!"}' -UseBasicParsing
$token = $login.token
$headers = @{ Authorization = "Bearer $token" }

$devs = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/devices' -Headers $headers -UseBasicParsing
Write-Host "Devices count: $($devs.data.Count)"
foreach ($d in $devs.data) {
    Write-Host "Device: $($d.name) | ID: $($d.id) | Type: $($d.type) | Label: $($d.label)"
}

$opcDev = $devs.data | Where-Object { $_.name -eq 'OPC_UA_Server_172.17.0.3' }
if ($opcDev) {
    Write-Host "`nFetching latest telemetry for $($opcDev.name)..."
    $latest = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/telemetry/DEVICE/$($opcDev.id)/values/latest" -Headers $headers -UseBasicParsing
    Write-Host ($latest | ConvertTo-Json -Depth 3)
}
