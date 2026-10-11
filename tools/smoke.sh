#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:8080"
echo "=== 1. Logging in as seeded demo user (demo@iotp.local) ==="
TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@iotp.local","password":"demo1234"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Login failed"
  exit 1
fi
echo "Success! JWT Token received."

echo -e "\n=== 2. Creating a new device ==="
DEV_RESP=$(curl -s -X POST "$BASE_URL/api/v1/devices" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Smart-Meter-100","label":"Main Building Electrical Meter"}')

DEVICE_ID=$(echo "$DEV_RESP" | jq -r '.device.id')
DEVICE_TOKEN=$(echo "$DEV_RESP" | jq -r '.credentials.credentials_value')
echo "Device created! ID: $DEVICE_ID"
echo "Device Access Token: $DEVICE_TOKEN"

echo -e "\n=== 3. Uploading device telemetry via wire-compatible endpoint ==="
curl -s -X POST "$BASE_URL/api/v1/$DEVICE_TOKEN/telemetry" \
  -H "Content-Type: application/json" \
  -d '{"power":342.5,"voltage":230.1,"current":1.48,"operational":true}'

sleep 0.1

echo -e "\n=== 4. Querying latest telemetry via REST API ==="
curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/telemetry/DEVICE/$DEVICE_ID/values/latest" | jq .

echo -e "\n=== 5. Querying time-series telemetry via REST API ==="
curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/telemetry/DEVICE/$DEVICE_ID/values/timeseries?keys=power,voltage" | jq .

echo -e "\nSMOKE TEST PASSED!"
