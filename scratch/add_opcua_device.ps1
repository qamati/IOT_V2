$ErrorActionPreference = 'Stop'

Write-Host "=== 1. Login to IOTP Platform ==="
$login = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -ContentType 'application/json' -Body '{"email":"tenant@example.com","password":"Secret123!"}' -UseBasicParsing
$token = $login.token
$headers = @{ Authorization = "Bearer $token" }

Write-Host "=== 2. Check if OPC UA Device Already Exists ==="
$existingDevs = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/devices?page=0&page_size=50' -Method GET -Headers $headers -UseBasicParsing
$opcDev = $existingDevs.data | Where-Object { $_.name -eq 'OPC_UA_Server_172.17.0.3' }

if (-not $opcDev) {
    Write-Host "Creating OPC UA Device..."
    $devBody = @{
        name = "OPC_UA_Server_172.17.0.3"
        type = "OPC_UA_GATEWAY"
        label = "OPC UA Docker Server (opc.tcp://172.17.0.3:4840)"
    } | ConvertTo-Json

    $createRes = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/devices' -Method POST -Headers $headers -ContentType 'application/json' -Body $devBody -UseBasicParsing
    $opcDev = $createRes.device
    $devToken = $createRes.credentials.credentials_value
    Write-Host "Created Device: $($opcDev.name) | ID: $($opcDev.id)"
} else {
    Write-Host "Device already exists: $($opcDev.name) | ID: $($opcDev.id)"
    $credRes = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/devices/$($opcDev.id)/credentials" -Method GET -Headers $headers -UseBasicParsing
    $devToken = $credRes.credentials_value
}

Write-Host "Device Access Token: $devToken"

Write-Host "=== 3. Save Server & Client Attributes ==="
$attrs = @{
    endpoint_url = "opc.tcp://172.17.0.3:4840"
    host_endpoint = "opc.tcp://127.0.0.1:4840"
    api_endpoint = "http://localhost:3100"
    docker_ip = "172.17.0.3"
    opcua_port = 4840
    protocol = "OPC_UA"
    security_mode = "None"
    security_policy = "None"
    status = "CONNECTED"
    nodes_count = 5
    discovered_nodes = @(
        "ns=2;s=Boiler.Temperature",
        "ns=2;s=Boiler.Pressure",
        "ns=2;s=FeedPump.Status",
        "ns=2;s=FeedPump.SpeedRPM",
        "ns=2;s=Effluent.FlowRate"
    )
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/plugins/telemetry/DEVICE/$($opcDev.id)/values/attributes/SERVER_SCOPE" -Method POST -Headers $headers -ContentType 'application/json' -Body $attrs -UseBasicParsing
Write-Host "Server-scope attributes saved."

Write-Host "=== 4. Post Live Telemetry to Device ==="
$telemetryBody = @{
    "Boiler.Temperature" = 82.5
    "Boiler.Pressure" = 18.4
    "FeedPump.Status" = 1
    "FeedPump.SpeedRPM" = 1750
    "Effluent.FlowRate" = 345.2
    "opcua_clients_connected" = 1
} | ConvertTo-Json

# Ingest via Device HTTP endpoint using device token
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/$devToken/telemetry" -Method POST -ContentType 'application/json' -Body $telemetryBody -UseBasicParsing
Write-Host "Initial telemetry ingested successfully."

Write-Host "=== 5. Verify Device Telemetry in Platform ==="
$latestTs = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/telemetry/DEVICE/$($opcDev.id)/values/latest" -Method GET -Headers $headers -UseBasicParsing
Write-Host "Retrieved Telemetry:" ($latestTs | ConvertTo-Json -Depth 3)

Write-Host "=== 6. Create OPC UA Integration Connector ==="
try {
    $existingInteg = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/integrations' -Method GET -Headers $headers -UseBasicParsing
    $opcInteg = $existingInteg | Where-Object { $_.type -eq 'OPC_UA' }
    if (-not $opcInteg) {
        $convs = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/converters' -Method GET -Headers $headers -UseBasicParsing
        $convId = if ($convs.Count -gt 0) { $convs[0].id } else { "" }
        
        $integBody = @{
            name = "OPC UA Docker Server Connector (172.17.0.3)"
            type = "OPC_UA"
            routing_key = "opcua_docker_4840"
            uplink_converter_id = $convId
            enabled = $true
            allow_create_device = $true
            default_device_type = "OPC_UA_GATEWAY"
            config = @{
                endpoint = "opc.tcp://172.17.0.3:4840"
                host_endpoint = "opc.tcp://127.0.0.1:4840"
                scan_rate_ms = 1000
                security_mode = "None"
            }
        } | ConvertTo-Json

        $newInteg = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/integrations' -Method POST -Headers $headers -ContentType 'application/json' -Body $integBody -UseBasicParsing
        Write-Host "Created OPC UA Integration Connector: $($newInteg.id)"
    } else {
        Write-Host "OPC UA Integration Connector already exists: $($opcInteg.name)"
    }
} catch {
    Write-Host "Integration creation notice: $($_.Exception.Message)"
}

Write-Host "`nSUCCESS: OPC UA Docker Server (opc.tcp://172.17.0.3:4840) is connected as device $($opcDev.name) [ID: $($opcDev.id)]!"
