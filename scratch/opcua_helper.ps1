$ErrorActionPreference = 'Stop'

# Create Namespace
$bodyNs = @{
    name = "Plant.Production"
    uri = "urn:iotp:production"
    description = "Industrial Process Floor"
} | ConvertTo-Json

try {
    $ns = Invoke-RestMethod -Uri "http://localhost:3100/api/namespaces" -Method POST -ContentType "application/json" -Body $bodyNs -UseBasicParsing
    Write-Host "Created Namespace ID: $($ns.id)"
    $nsId = $ns.id
} catch {
    $existing = Invoke-RestMethod -Uri "http://localhost:3100/api/namespaces" -Method GET -UseBasicParsing
    $nsId = $existing[0].id
    Write-Host "Using Existing Namespace ID: $nsId"
}

# Create Nodes
$nodesToCreate = @(
    @{ name = "Boiler.Temperature"; dataType = "Double"; initialValue = 82.5; description = "Main steam boiler temperature" },
    @{ name = "Boiler.Pressure"; dataType = "Double"; initialValue = 18.4; description = "Main steam boiler pressure bar" },
    @{ name = "FeedPump.Status"; dataType = "Boolean"; initialValue = $true; description = "Boiler feed pump running status" },
    @{ name = "FeedPump.SpeedRPM"; dataType = "Int32"; initialValue = 1750; description = "Boiler feed pump speed RPM" },
    @{ name = "Effluent.FlowRate"; dataType = "Double"; initialValue = 345.2; description = "Effluent flow rate GPM" }
)

foreach ($node in $nodesToCreate) {
    try {
        $bodyNode = @{
            namespaceId = $nsId
            name = $node.name
            dataType = $node.dataType
            initialValue = $node.initialValue
            description = $node.description
        } | ConvertTo-Json

        $nRes = Invoke-RestMethod -Uri "http://localhost:3100/api/nodes" -Method POST -ContentType "application/json" -Body $bodyNode -UseBasicParsing
        Write-Host "Created Node: $($nRes.name) ($($nRes.data_type)) with ID: $($nRes.id)"
    } catch {
        Write-Host "Node creation error or already exists: $($_.Exception.Message)"
    }
}

# Restart/Reload runtime
try {
    Invoke-RestMethod -Uri "http://localhost:3100/api/server/reload" -Method POST -UseBasicParsing
    Write-Host "OPC UA Server reloaded."
} catch {
    Write-Host "Reload status: $($_.Exception.Message)"
}

$allNodes = Invoke-RestMethod -Uri "http://localhost:3100/api/nodes" -Method GET -UseBasicParsing
Write-Host "Total Nodes in Server: $($allNodes.Count)"
foreach ($n in $allNodes) {
    Write-Host " - Node: $($n.name) | Type: $($n.data_type) | Val: $($n.initial_value)"
}
