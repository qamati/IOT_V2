# Service 10 — Rule Node Catalog

> **Stage:** 4 (P0 nodes), 5–8 (others) · **Lives in:** `pkg/rules/nodes/*` · Each node = Go type + `Descriptor` (JSON Schema + UI schema) · Priorities: P0 MVP · P1 parity · P2 later

## 1. Conventions
- **Type id:** `category.name` (e.g. `filter.script`). **Relations:** labels a node can emit; unconnected `Failure` marks the message failed.
- **Templates in configs:** `${metadataKey}` reads metadata; `$[dataKey]` reads a top-level key of the JSON body.
- **Every node descriptor** declares: `configSchema`, `uiSchema`, `relations` (static list or `dynamic`), `version` + migrator, `docs` (markdown), `sideEffects` (none | internal | external) used by *replay-in-sandbox* to stub external effects.
- **Message types** (originator-scoped): `POST_TELEMETRY_REQUEST` ("Post telemetry"), `POST_ATTRIBUTES_REQUEST` ("Post attributes"), `ATTRIBUTES_UPDATED`, `ATTRIBUTES_DELETED`, `TIMESERIES_UPDATED`, `TIMESERIES_DELETED`, `ATTRIBUTES_REQUEST`, `CONNECT_EVENT`, `DISCONNECT_EVENT`, `ACTIVITY_EVENT`, `INACTIVITY_EVENT`, `ENTITY_CREATED|UPDATED|DELETED|ASSIGNED|UNASSIGNED`, `ALARM`, `ALARM_ACK`, `ALARM_CLEAR`, `ALARM_ASSIGNED`, `ALARM_UNASSIGNED`, `ALARM_COMMENT`, `TO_SERVER_RPC_REQUEST`, `RPC_CALL_FROM_SERVER_TO_DEVICE`, `RPC_QUEUED|DELIVERED|SUCCESSFUL|TIMEOUT|EXPIRED|FAILED|DELETED`, `RELATION_ADD_OR_UPDATE|DELETED`, `REST_API_REQUEST`, `GENERATOR`, `OTA_STATE`, `CF_RESULT`, `SCADA_ALARM`, `SCADA_CONTROL`.
- **Standard metadata keys:** `deviceName`, `deviceType`, `customerId`, `ts`, `scope` (attributes), `integrationName`, `edgeId`.

## 2. Filter nodes
| Type | Pri | Relations | Config highlights |
|---|---|---|---|
| `filter.script` | P0 | True, False, Failure | `expr`/JS body returning bool; sees `msg, metadata, msgType` |
| `filter.msg_type` | P0 | True, False | list of message types |
| `filter.msg_type_switch` | P0 | one per type (`Post telemetry`, `Attributes Updated`, `Other`…) | — |
| `filter.originator_type` | P1 | True, False | entity types |
| `filter.originator_type_switch` | P1 | per entity type | — |
| `filter.switch` | P1 | dynamic (script returns relation names) | script |
| `filter.check_fields` | P1 | True, False | data keys, metadata keys, `checkAll` |
| `filter.check_relation` | P1 | True, False | direction, entity, relation type |
| `filter.alarm_status` | P1 | True, False | statuses list |
| `filter.gps_geofence` | P1 | True, False | lat/lon keys, perimeter (polygon/circle constant or from attributes), `minInsideDuration` |

## 3. Enrichment nodes (add data to metadata or body)
| Type | Pri | Config highlights |
|---|---|---|
| `enrich.originator_attributes` | P0 | scope(s), keys (client/server/shared/latest-ts), target `metadata|data`, `tellFailureIfAbsent` |
| `enrich.originator_telemetry` | P0 | mode `LATEST` or `INTERVAL(start,end)`+`FETCH_FIRST|LAST|ALL`, order, limit, aggregation |
| `enrich.originator_fields` | P1 | name, type, label, profile… → metadata |
| `enrich.customer_attributes` / `enrich.customer_details` | P1 | keys / detail fields |
| `enrich.tenant_attributes` / `enrich.tenant_details` | P1 | keys / detail fields |
| `enrich.related_attributes` | P1 | relation query (direction, type, level), keys |
| `enrich.device_attributes` | P1 | related devices by relation, scope/keys |
| `enrich.entity_details` | P1 | selected entity fields |
| `enrich.calculate_delta` | P1 | input key, output key, `useCache`, `tellFailureIfDeltaNegative`, `excludeZeroDeltas`, period |

## 4. Transformation nodes
| Type | Pri | Notes |
|---|---|---|
| `transform.script` | P0 | returns `{msg, metadata, msgType}`; array return = multiple messages |
| `transform.change_originator` | P1 | to customer / tenant / related entity (query) / alarm originator / entity by name template |
| `transform.copy_keys` | P1 | data↔metadata, key list or regex |
| `transform.rename_keys` | P1 | mapping |
| `transform.delete_keys` | P1 | list/regex, from data or metadata |
| `transform.json_path` | P1 | JSONPath expression → new body |
| `transform.split_array` | P1 | emit one message per element |
| `transform.deduplicate` | P1 | window, `FIRST|LAST|ALL`, key = originator; durable via timers |
| `transform.math` | P1 | arguments (data/metadata/attr/telemetry), expression, result target; rounding |
| `transform.aggregate_latest` | P2 | over related entities: min/max/avg/sum/count; period |
| `transform.aggregate_stream` | P2 | windowed aggregation (tumbling/sliding), out-of-order tolerance |

## 5. Action nodes
| Type | Pri | Relations | Notes |
|---|---|---|---|
| `action.save_timeseries` | P0 | Success, Failure | `ttl`, `useServerTs`, **strategies** `{timeseries, latest, realtime} × {PERSIST, DEDUP(window), SKIP}`; skip-latest-if-older |
| `action.save_attributes` | P0 | Success, Failure | scope, `notifyDevice`, `sendAttributesUpdatedNotification`, `updateAttributesOnlyOnValueChange` |
| `action.delete_attributes` | P1 | Success, Failure | scope, keys, `notifyDevice` |
| `action.create_alarm` | P0 | Created, Updated, False, Failure | declarative (type, severity, propagate…) or script returning alarm details; `useMessageAlarmData` |
| `action.clear_alarm` | P0 | Cleared, False, Failure | type, details script |
| `action.assign_alarm` / `action.unassign_alarm` | P1 | Success, Failure | assignee by user/email template |
| `action.alarm_comment` | P2 | Success, Failure | text template |
| `action.rpc_request` | P1 | Success, Failure | to originator device; `timeout`, `persistent`, `retries`, `expiration`; correlation id |
| `action.rpc_reply` | P1 | Success, Failure | replies to a client-side `TO_SERVER_RPC_REQUEST` |
| `action.log` | P0 | Success, Failure | script → logger (rate-limited, tenant-visible) |
| `action.delay` | P1 | Success, Failure, `Retry` | **durable timer**; max pending per tenant |
| `action.generator` | P1 | Success | N msgs/period, script body — for testing/simulation |
| `action.msg_count` | P2 | Success | counter message every interval |
| `action.create_relation` / `action.delete_relation` | P1 | Success, Failure | direction, type, target entity (by name/type), `createEntityIfNotExists` |
| `action.assign_to_customer` / `unassign` / `assign_to_tenant` | P2 | Success, Failure | |
| `action.copy_to_view` | P2 | Success | copy attributes to entity views |
| `action.send_notification` | P1 | Success, Failure | template id + target ids or inline |
| `action.send_email` / `action.send_sms` | P1 | Success, Failure | templates, attachments (P2) |
| `action.push_to_edge` / `action.push_to_cloud` | P1 | Success, Failure | scope attributes/telemetry/alarm/rpc; enqueue to edge lane |
| `action.send_to_calculated_fields` | P1 | Success, Failure | trigger dependent CF evaluation |
| `action.save_to_custom_table` | P2 | Success, Failure | schema-mapped insert (tenant sandbox table) |
| `action.scada_write` | P2 | Success, Failure | goes through SCADA control policy (never raw write) |

## 6. External nodes
| Type | Pri | Notes |
|---|---|---|
| `external.rest_call` | P0 | method, URL template, headers, body template, timeout, retries, **SSRF guard** (deny private ranges unless allow-listed), `useSimpleClientHttpFactory`, credentials from secrets, **Failure** on non-2xx (configurable); output: response in body, status in metadata |
| `external.mqtt` | P1 | topic template, QoS, TLS creds, connection pool |
| `external.kafka` | P1 | topic/key templates, headers, acks |
| `external.rabbitmq`, `external.aws_sns`, `external.aws_sqs`, `external.azure_iot_hub`, `external.gcp_pubsub` | P2 | creds via secrets |
| `external.slack` / `external.teams` | P1 / P2 | webhook or bot token |
| `external.twilio_sms` | P2 | |
| `external.ai_request` | P2 | model config id, prompt template, structured output schema, timeout/budget guards, PII-redaction option |
All external nodes: bounded concurrency per tenant, circuit breaker, per-node timeout, secrets by reference (never inline in exported chains).

## 7. Flow nodes
| Type | Pri | Notes |
|---|---|---|
| `flow.input` | P0 | implicit entry |
| `flow.rule_chain` | P0 | call nested chain; returns via its `flow.output` labels |
| `flow.output` | P0 | exit with relation label |
| `flow.ack` | P1 | acknowledge early (stop retry for this message) |
| `flow.checkpoint` | P1 | re-enqueue into another queue (decouple stages, e.g., to `highprio`) |

## 8. Example configurations
```json
// action.save_timeseries
{ "ttlSec": 0, "useServerTs": false,
  "strategies": { "timeseries": {"mode":"PERSIST"}, "latest": {"mode":"PERSIST"}, "realtime": {"mode":"DEDUP","windowMs":1000} } }

// filter.script (expr language)
{ "language": "expr", "script": "msg.temperature > 80 && metadata.deviceType == 'boiler'" }

// action.create_alarm (declarative)
{ "alarmType": "High temperature", "severity": "CRITICAL", "propagate": true,
  "details": "Temp is ${ts} -> $[temperature]", "dynamicSeverityScript": null }

// external.rest_call
{ "method": "POST", "url": "https://hooks.example.com/${deviceName}", "headers": {"Content-Type":"application/json"},
  "timeoutMs": 5000, "retries": 2, "treatNon2xxAsFailure": true, "bodyTemplate": null }
```
**Default root chain (shipped):** `msg_type_switch` → *Post telemetry* → `save_timeseries`; *Post attributes* → `save_attributes`; *Attributes Updated/Other* → `log` (disabled by default); *RPC from device* → `rpc_reply` stub; device-profile alarms are evaluated by `alarm` independently of the chain.

## 9. Node authoring checklist (for you and plugin authors)
1. Implement `Node` + register `Descriptor` (schema, relations, version).
2. Call exactly one `Tell*`/`Schedule` per message; never block the shard > 1 ms — use async helpers (`ctx.Async(func())`).
3. No global state; per-tenant caches via `Env`.
4. Declare `sideEffects`.
5. Add golden tests (input → relation/output) and a fuzz test for config parsing.
6. Provide docs + example chain.

## 10. Task checklist
- [ ] Descriptor system + JSON-schema forms contract with the editor
- [ ] P0: input/output/rule_chain, msg_type(_switch), filter.script, transform.script, enrich originator attrs/telemetry, save TS/attrs, create/clear alarm, log, rest_call
- [ ] P1 filters/enrichment/transforms/actions in the tables above
- [ ] External connectors with pooling, secrets, breakers, SSRF guard
- [ ] Node versioning + config migrators; export/import with secret refs
- [ ] Test harness (golden + fuzz) and docs generator
