$ErrorActionPreference = 'Stop'
$login = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/auth/login' -Method POST -ContentType 'application/json' -Body '{"email":"tenant@example.com","password":"Secret123!"}' -UseBasicParsing
$devs = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/devices?page=0&page_size=20' -Method GET -Headers @{ Authorization = "Bearer $($login.token)" } -UseBasicParsing
$devs.data | Select-Object name, type, label, id | Format-Table -AutoSize
