# Stage 3 — Data Pipeline (telemetry, attributes, device state)

> **Estimate:** 28 eng-weeks · **Depends on:** Stage 2 · **Unlocks:** stages 4, 5, 6
> **Specs:** `services/11-telemetry-attributes.md`, `07-device-state.md`, `08-message-bus.md` (Kafka hardening), `21-audit-events-usage.md` (basic usage)

## 1. Goal
Everything a device sends is **stored, queryable, and observable**: time-series + latest values + attributes with idempotent group-commit writes, real-time update publication, online/offline tracking, retention, and basic usage/rate limiting.

## 2. Scope
**In (P0):** `telemetry` service (write API, Timescale driver, latest cache, raw/aggregated queries, calendar buckets), attributes (CLIENT/SERVER/SHARED + shared push to devices), `devstate`, retention classes + TTL worker, usage counters (transport msgs, data points) and per-tenant rate limits, Kafka driver hardening (rebalance, lag metrics, DLQ), **temporary direct-save worker** (consumes `re.main` and persists telemetry/attributes so this stage completes before the rule engine exists; replaced by the default root chain in stage 4).
**P1 (spillover ok):** rollups (continuous aggregates), persistence strategies (dedup/skip), unit conversion, delete-range API.
**Out:** rule execution, alarms, dashboards.

## 3. Deliverables
1. `cmd/telemetry` with gRPC `Write(stream)`/`Query` and REST (`/telemetry/...`, `/attributes/...`).
2. `pkg/tsstore` interface + **conformance suite** + Timescale driver (+ SQLite driver for lite).
3. `cmd/devstate` + `devstate.activity`/`devstate.snapshot` topics wired from transports.
4. Shared-attribute downlink path (`telemetry` → `notify.transport.{node}`) tested over MQTT/HTTP.
5. Retention worker + tenant profile retention classes.
6. Usage counters + rate-limit enforcement in transports; usage query API.
7. Console pages: device **Latest telemetry**, **Attributes** (CRUD by scope), simple time-series chart (uPlot).
8. Load-test harness (`tools/loadgen`) + Grafana dashboards for ingest/lag/write latency.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| tsstore + Timescale | Schema, key dictionary, upsert (unnest), latest guard, compression/retention classes, conformance suite | 6 |
| Write API | Streaming gRPC, sharding, batching, dedupe, sanity rules, ack semantics | 4 |
| Query engine | Raw, aggregates, calendar buckets, fill, max-points, latest via Redis, keys discovery | 5 |
| Attributes | Table, cache, scopes, notifications, shared push downlink | 3 |
| devstate | Timer wheel, shards, snapshots, event emission | 4 |
| Retention/usage | TTL worker, counters → Redis → snapshot, limit enforcement, API | 3 |
| Bus hardening | Kafka rebalance hooks, lag metrics, DLQ/replay CLI | 1 |
| Frontend | Latest/attributes tabs, basic chart, timewindow picker v0 | 2 |

## 5. Acceptance criteria
- **Ingest:** 100 k rows/s sustained into Timescale on the reference rig (16 vCPU/64 GB NVMe) with p99 commit < 200 ms; no acknowledged-message loss under DB kill/restart (chaos test).
- **Idempotency:** replaying the same 1 M-message pack twice yields identical table contents.
- **Query:** raw 1 key/24 h p95 < 150 ms; 30-day 5-min buckets p95 < 400 ms (with rollups) / < 1.5 s (without); calendar `MONTH` buckets correct across DST (golden tests).
- **Device state:** 1 M simulated devices tracked in ≤ 150 MB; inactivity event within `timeout + 5 s` p99; rebalance rebuild < 60 s.
- **Shared attributes:** update reaches a connected MQTT device in < 500 ms p95 and is delivered on next connect for offline devices.
- **Limits:** tenant over its `transportTenantMsg` rate receives quota reason codes and others are unaffected (isolation test).

## 6. Demo (exit)
Simulate 10 k devices × 1 msg/s → live Grafana ingest dashboard → open a device, see latest values + 24 h chart → kill Timescale primary during load → recover with zero acked loss → change a shared attribute and watch the device receive it → stop a device and watch it flip to inactive.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Timescale compression vs late data | Keep `compress_after ≥ max lateness`; metrics on decompression events |
| Hot-entity write amplification | Per-entity ordering shard + batch API guidance; per-device limits |
| Per-tenant TTL on a shared hypertable | Retention classes (tables) + batched deletes for exceptions |
| Direct-save worker lingering | Feature flag + removal ticket gated to stage 4 DoD |

## 8. Definition of Done
- [ ] Conformance suite green for Timescale and SQLite drivers
- [ ] Chaos + idempotency + load reports archived
- [ ] Dashboards/alerts for ingest lag, write latency, wheel lag
- [ ] Direct-save worker behind flag with removal plan
