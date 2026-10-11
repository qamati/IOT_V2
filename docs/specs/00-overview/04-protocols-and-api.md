# 04 — Protocols & API Reference

> Strategy: **device-facing v1 APIs are wire-compatible with ThingsBoard** (MQTT topics, HTTP paths, gateway topics, payload shapes) so existing devices, SDKs and gateways work unchanged. **v2** APIs add what we do better (batching, MQTT v5 features, binary framing). Management APIs (REST/WS) are **our own OpenAPI-first design** that follows ThingsBoard resource naming where sensible, which keeps a future import/migration tool simple.

## 1. Device API v1 — MQTT (3.1.1 and 5.0)

Auth at CONNECT: `username = ACCESS_TOKEN` (password empty) **or** `clientId/username/password` (MQTT_BASIC) **or** mutual-TLS client certificate (X509).

| Purpose | Direction | Topic |
|---|---|---|
| Publish telemetry | device → server | `v1/devices/me/telemetry` |
| Publish client attributes | device → server | `v1/devices/me/attributes` |
| Receive shared-attribute updates | server → device | subscribe `v1/devices/me/attributes` |
| Request attributes | device → server | publish `v1/devices/me/attributes/request/{id}` |
| Attribute response | server → device | subscribe `v1/devices/me/attributes/response/+` |
| Server-side RPC request | server → device | subscribe `v1/devices/me/rpc/request/+` |
| Server-side RPC reply | device → server | publish `v1/devices/me/rpc/response/{id}` |
| Client-side RPC request | device → server | publish `v1/devices/me/rpc/request/{id}` |
| Client-side RPC response | server → device | subscribe `v1/devices/me/rpc/response/+` |
| Claim device | device → server | publish `v1/devices/me/claim` |
| Provisioning request/response | both | publish `/provision/request`, subscribe `/provision/response` |
| OTA chunk download | both | `v2/fw/request/{reqId}/chunk/{n}` → `v2/fw/response/{reqId}/chunk/{n}` (and `v2/sw/…`) |

**Gateway topics** (one connection, many devices):

| Purpose | Topic |
|---|---|
| Connect / disconnect child device | `v1/gateway/connect`, `v1/gateway/disconnect` |
| Telemetry / attributes | `v1/gateway/telemetry`, `v1/gateway/attributes` |
| Attribute request/response | `v1/gateway/attributes/request`, `v1/gateway/attributes/response` |
| RPC | `v1/gateway/rpc` |
| Claim | `v1/gateway/claim` |

**Payload shapes (JSON):**
```json
{"temperature": 22.5, "humidity": 40, "alarm": false}                       // server timestamp
{"ts": 1730000000000, "values": {"temperature": 22.5}}                       // explicit ts
[{"ts":1730000000000,"values":{"t":1}}, {"ts":1730000001000,"values":{"t":2}}]  // batch
```
Gateway telemetry: `{"Device A":[{"ts":...,"values":{...}}],"Device B":[...]}`.

**Profile-level variants:** custom topic filters; **Protobuf** payloads (descriptor stored in device profile); **Sparkplug B** (`spBv1.0/{group}/{NBIRTH|NDATA|DBIRTH|DDATA|NCMD|DCMD|NDEATH|DDEATH}/{edge}/{device}`).

**QoS:** 0/1 supported; QoS 2 → downgrade or reject (document choice). Retained messages ignored for device topics. Max payload default 64 KB (profile-configurable).

### v2 additions (ours)
- MQTT v5 **user properties** (`ts`, `trace-id`), **reason codes** for rate limiting, **topic aliases**, **session expiry** for sleepy devices.
- Binary batch frame: `v2/batch` with zstd + protobuf (`TelemetryBatch`) for cellular devices.

## 2. Device API v1 — HTTP(S)

| Method & path | Purpose |
|---|---|
| `POST /api/v1/{token}/telemetry` | Upload telemetry (same JSON shapes) |
| `POST /api/v1/{token}/attributes` | Upload client attributes |
| `GET  /api/v1/{token}/attributes?clientKeys=a,b&sharedKeys=c` | Read attributes (supports `timeout` long-poll for subscription) |
| `GET  /api/v1/{token}/rpc?timeout=20000` | Long-poll for server RPC |
| `POST /api/v1/{token}/rpc/{id}` | Reply to server RPC |
| `POST /api/v1/{token}/rpc` | Client-side RPC (blocking until response/timeout) |
| `POST /api/v1/{token}/claim` | Claim device |
| `GET  /api/v1/{token}/firmware?title=&version=[&size=&chunk=]` | OTA download (Range supported) |
| `POST /api/v1/provision` | Device provisioning |

## 3. Device API v1 — CoAP / LwM2M / SNMP

- **CoAP:** `coap(s)://host:5683/api/v1/{token}/{telemetry|attributes|rpc|claim|firmware}`; **Observe** on `attributes` and `rpc`; DTLS PSK/X.509; blockwise for OTA.
- **LwM2M:** servers on 5685 (UDP), 5686 (DTLS), bootstrap 5687/5688; registration lifecycle, observe/write-attributes, objects mapped to telemetry/attributes via profile lists (`/3/0/9` → `batteryLevel`); security: NoSec, PSK, RPK, X.509; FOTA via Object 5.
- **SNMP:** platform acts as manager; profile defines OIDs, polling period, mapping to telemetry/attributes; v1/v2c/v3.

## 4. Management REST API (OpenAPI 3.1, prefix `/api/v1`)

**Conventions**
| Topic | Rule |
|---|---|
| Auth | `Authorization: Bearer <JWT>` or `X-Api-Key: <key>` |
| IDs | UUID strings; entity references `{ "entityType": "DEVICE", "id": "…" }` |
| Pagination | `page`, `pageSize` (default 20, max 1000), `textSearch`, `sortProperty`, `sortOrder=ASC|DESC`. Response `{ data[], totalPages, totalElements, hasNext }`. **Cursor mode** (`cursor` token) for large collections (alarms, events, audit) |
| Errors | RFC 9457 `application/problem+json`: `type, title, status, detail, errorCode, traceId` |
| Idempotency | `Idempotency-Key` header on POST/PUT/DELETE with side effects |
| Concurrency | `ETag`/`If-Match` using `version` |
| Rate limits | `X-RateLimit-Limit/Remaining/Reset`, `429` + `Retry-After` |
| Time | epoch ms (int64) |
| Versioning | URL major (`/api/v1`), additive minor changes; deprecation via `Sunset` header |

**Resource groups (≈ ThingsBoard-aligned names):**

| Group | Representative endpoints |
|---|---|
| Auth | `POST /auth/login`, `/auth/token` (refresh), `/auth/logout`, `GET /auth/user`, `/auth/2fa/*`, `/auth/oauth2/*`, `/api-keys` |
| Tenants & profiles | `/tenants`, `/tenantProfiles` |
| Customers & users | `/customers`, `/users`, `/users/{id}/activationLink`, `/roles`, `/entityGroups` |
| Devices | `/devices`, `/devices/{id}/credentials`, `/devices/bulk`, `/devices/{id}/claim`, `/deviceProfiles`, `/devices/connectivity/{id}` |
| Assets / views | `/assets`, `/assetProfiles`, `/entityViews` |
| Relations | `POST /relations`, `GET /relations?fromId&relationType`, `POST /relations/query` |
| Entity query | `POST /entitiesQuery/find`, `/entitiesQuery/count`, `/alarmsQuery/find`, `/entityQueries/keys` |
| Time-series | `GET /telemetry/{type}/{id}/values/timeseries`, `/keys/timeseries`, `DELETE …/timeseries`, `POST …/timeseries/{scope}` |
| Attributes | `GET/POST/DELETE /telemetry/{type}/{id}/attributes/{scope}` |
| Alarms | `/alarms`, `/alarm/{id}/ack|clear|assign`, `/alarm/{id}/comment` |
| Alarm rules | `/alarmRules` |
| Rule chains | `/ruleChains`, `/ruleChains/{id}/revisions`, `/ruleChains/{id}/publish`, `/ruleChains/testScript`, `/components` |
| RPC | `POST /rpc/oneway/{deviceId}`, `/rpc/twoway/{deviceId}`, `GET /rpc/persistent/{rpcId}`, `/rpc/persistent/device/{deviceId}` |
| OTA | `/otaPackages`, `/otaPackages/{id}/download`, `/otaCampaigns` |
| Dashboards | `/dashboards`, `/widgetsBundles`, `/widgetTypes`, `/images`, `/resources` |
| Notifications | `/notification/targets|templates|rules|requests`, `/notifications` |
| Calculated fields | `/calculatedFields`, `/calculatedFields/testScript` |
| Integrations | `/integrations`, `/converters`, `/converters/test` |
| Edge | `/edges`, `/edges/{id}/assign`, `/edges/{id}/sync`, `/edges/{id}/events` |
| SCADA | `/scada/tags`, `/scada/udts`, `/scada/drivers`, `/scada/control`, `/scada/alarms` |
| Ops | `/audit/logs`, `/events/{entityType}/{id}`, `/usage`, `/admin/settings/*` |

## 5. WebSocket API (`wss://host/api/v1/ws`)

Auth: first frame `{"auth":{"token":"<JWT>"}}` (preferred) or `?token=` on connect (discouraged — logged by proxies). Server sends `{"type":"AUTH_OK"}`; JWT refresh via `{"type":"REAUTH","token":…}`.

**Client commands** (batched in `cmds[]`, each with client-chosen `cmdId`):
```json
{ "cmds": [
  { "type":"ENTITY_DATA", "cmdId":1,
    "query": { "entityFilter": {"type":"singleEntity","singleEntity":{"entityType":"DEVICE","id":"…"}},
               "pageLink": {"pageSize":10,"page":0}, "entityFields":[{"type":"ENTITY_FIELD","key":"name"}],
               "latestValues":[{"type":"TIME_SERIES","key":"temperature"}] },
    "latestCmd": { "keys": [{"type":"TIME_SERIES","key":"temperature"}] },
    "tsCmd": { "keys":["temperature"], "startTs":1730000000000, "timeWindow":3600000,
               "interval":60000, "agg":"AVG", "limit":1000 } },
  { "type":"ALARM_DATA",  "cmdId":2, "query": { … } },
  { "type":"ENTITY_COUNT","cmdId":3, "query": { … } },
  { "type":"NOTIFICATIONS","cmdId":4, "limit":10 },
  { "type":"UNSUBSCRIBE","cmdId":1 }
] }
```
**Server updates:** `{ "cmdId":1, "data": {…initial page…}, "update": [ … ] }`; errors `{ "cmdId":1, "errorCode":…, "errorMsg":… }`.

**Behaviour:** per-session/tenant subscription caps; update **coalescing** (latest wins) with min interval (default 1 s for dynamic queries, 0 for single-entity); permessage-deflate; optional MessagePack framing.

## 6. Internal gRPC and event contracts (protobuf, `buf` managed)

| Package | Key services/messages |
|---|---|
| `iotp.core.v1` | `EntityService`, `RelationService`, `QueryService` |
| `iotp.devreg.v1` | `DeviceAuthService.Authenticate`, `ProvisionService`, `ClaimService` |
| `iotp.telemetry.v1` | `WriteService.Write(stream)`, `QueryService.Timeseries/Latest/Aggregate` |
| `iotp.bus.v1` | `Envelope`, `TelemetryUp`, `AttributesUp`, `DeviceEvent`, `RpcDown`, `EntityEvent`, `AlarmEvent` |
| `iotp.edge.v1` | `EdgeSync.Stream(stream Up) returns (stream Down)`, `Handshake`, `FullSync`, `Ack` |

```proto
message Envelope {
  bytes  id = 1;            // uuidv7
  bytes  tenant_id = 2;
  EntityRef originator = 3; // type + id
  string type = 4;          // POST_TELEMETRY_REQUEST, ATTRIBUTES_UPDATED, …
  int64  ts = 5;            // ms
  map<string,string> metadata = 6;
  bytes  trace_context = 7; // W3C traceparent
  oneof payload { bytes json = 10; bytes binary = 11; TelemetryUp telemetry = 12; }
}
```

## 7. Webhooks and exports

- **Outbound webhooks** via rule node *REST API call* or notification channel *Webhook* (HMAC-SHA256 `X-Signature`, retries with backoff, DLQ).
- **Exports:** CSV/NDJSON/Parquet from `/telemetry/export` (async job → signed URL).
- **OpenAPI** served at `/api/v1/openapi.json`; **AsyncAPI** for bus topics in `api/asyncapi.yaml`.

## 8. Compatibility test suite (do this early)

- Run TB-compatible device simulators (MQTT token, MQTT basic, X.509, gateway) against `tr-mqtt`.
- Contract tests with recorded transcripts for each topic in §1.
- Fuzz payload decoders (JSON/protobuf) with `go test -fuzz`.
