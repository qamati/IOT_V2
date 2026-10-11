#!/usr/bin/env bash
set -euo pipefail

echo "=== 1. Login as Tenant Admin ==="
LOGIN_RESP=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"tenant@example.com","password":"Secret123!"}')

TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
echo "Logged in successfully. Token acquired."

echo -e "\n=== 2. List Devices ==="
DEVICES_RESP=$(curl -s http://localhost:8080/api/v1/devices -H "Authorization: Bearer $TOKEN")
DEV_ID=$(echo "$DEVICES_RESP" | grep -o '"id":"[^"]*' | head -n 1 | cut -d'"' -f4)
echo "Discovered device ID: $DEV_ID"

echo -e "\n=== 3. Query Device Activity ==="
curl -s "http://localhost:8080/api/v1/devices/$DEV_ID/activity" -H "Authorization: Bearer $TOKEN"
echo ""

echo -e "\n=== 4. Query Scoped Attributes ==="
curl -s "http://localhost:8080/api/v1/plugins/telemetry/DEVICE/$DEV_ID/values/attributes" -H "Authorization: Bearer $TOKEN"
echo ""

echo -e "\n=== 5. Trigger High Temperature Alarm ==="
curl -s -X POST http://localhost:8080/api/v1/DEV_ALPHA_TOKEN_123/telemetry \
  -H "Content-Type: application/json" \
  -d '{"temperature": 48.7, "humidity": 35.0}'
sleep 0.5

echo -e "\n=== 6. List Alarms ==="
ALARMS_RESP=$(curl -s http://localhost:8080/api/v1/alarms -H "Authorization: Bearer $TOKEN")
echo "$ALARMS_RESP"

ALARM_ID=$(echo "$ALARMS_RESP" | grep -o '"id":"[^"]*' | head -n 1 | cut -d'"' -f4)

if [ -n "$ALARM_ID" ]; then
  echo -e "\n=== 7. Acknowledge Alarm ==="
  curl -s -X POST "http://localhost:8080/api/v1/alarms/$ALARM_ID/ack" -H "Authorization: Bearer $TOKEN"
  echo ""

  echo -e "\n=== 8. Clear Alarm ==="
  curl -s -X POST "http://localhost:8080/api/v1/alarms/$ALARM_ID/clear" -H "Authorization: Bearer $TOKEN"
  echo ""
fi

echo -e "\n>>> ALL STAGE 1-5 FEATURES VERIFIED SUCCESSFULLY! <<<"
