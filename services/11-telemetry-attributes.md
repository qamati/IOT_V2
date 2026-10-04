# Service 11 — Telemetry & Attributes (`telemetry`)

> **Stage:** 3 · **Binary:** `cmd/telemetry` · **Scales by:** data points/s (writers partitioned by entity hash) · **State:** stores + caches · **Priority:** P0

## 1. Purpose
Owns **time-series** (append-mostly key/values with timestamps), **latest values**, and **attributes** (scoped key/values). Provides idempotent group-commit writes, raw/aggregated queries, retention, rollups, export, and the real-time update stream.

## 2. ThingsBoard reference
- Data types: bool, long, double, string, json. Tables `ts_kv`, `ts_kv_latest`, `ts_kv_dictionary` (key→int). Backends: SQL (optionally Timescale / partitioned by day-month-year), Cassandra, or hybrid.
- Attribute scopes: `CLIENT_SCOPE` (device-owned), `SERVER_SCOPE` (platform-only), `SHARED_SCOPE` (pushed to device).
- Query API: raw, latest, aggregated (`MIN, MAX, AVG, SUM, COUNT, NONE`) by `interval`, `limit`, `orderBy`, calendar interval types, `useStrictDataTypes`; delete ranges with optional latest rewrite; TTL by tenant profile.
- Newer versions: per-message **persistence strategies** (persist/deduplicate/skip) for time-series, latest and WebSocket updates; **unit conversion**.

## 3. Architecture
```mermaid
flowchart LR
  RE[ruleengine save nodes] -->|gRPC stream Write| W[Write API]
  W --> SH[Shard by entity hash]
  SH --> DD[Dedup + strategy filter]
  DD --> MB[Micro-batcher: 50 ms / 5000 rows]
  MB --> ST[(tsstore: Timescale | ClickHouse | SQLite)]
  MB --> LV[(Latest cache: Redis + store)]
  MB --> RT[[NATS: rt.TENANT.ENTITY]]
  Q[Query API] --> RC[Rollup router]
  RC --> ST
  Q --> LV
  AT[Attributes API] --> PG[(Postgres attribute_kv)]
  AT --> AC[(Attr cache)]
  RET[Retention worker] --> ST
```

## 4. Store abstraction
```go
type Value struct {
    Kind   Kind // Bool, Long, Double, String, JSON
    B bool; L int64; D float64; S string; J json.RawMessage
}
type Point struct { Entity uuid.UUID; Key int32; Ts int64; V Value }   // Key = key_dictionary id

type Store interface {
    Write(ctx context.Context, pts []Point, opt WriteOpt) error        // idempotent upsert on (entity,key,ts)
    WriteLatest(ctx context.Context, pts []Point) error                // upsert only if ts >= existing
    Query(ctx context.Context, q Query) (Series, error)                // raw or aggregated
    Latest(ctx context.Context, e uuid.UUID, keys []int32) ([]Point, error)
    Delete(ctx context.Context, d DeleteReq) error
    Caps() Caps                                                         // supportsRollups, supportsCalendarBuckets, ...
}
type Query struct {
    Entity uuid.UUID; Keys []int32; StartTs, EndTs int64
    Interval int64; IntervalType IntervalType // MILLIS|WEEK|WEEK_ISO|MONTH|QUARTER (+ TZ)
    Agg Agg /*NONE|MIN|MAX|AVG|SUM|COUNT*/; Limit int; Order Order; Strict bool; TZ string
}
```

## 5. Write path
1. `Write` is a gRPC **client-stream** (per rule-engine node batch) returning per-batch commit acks; in lite mode it is an in-process call.
2. Points are sharded by `hash(entity)` to preserve order and enable in-batch dedupe.
3. **Strategy filter** per `(tenant/profile, kind)` ∈ `{timeseries, latest, realtime}` × `{PERSIST, DEDUP(window), SKIP}`; DEDUP keeps last `(value, ts)` per `(entity,key)` in an LRU and drops equal values inside the window.
4. **Micro-batcher** flushes at 5 000 rows or 50 ms. Duplicates on `(entity,key,ts)` *inside a batch* must be collapsed (last wins) — Postgres errors on `ON CONFLICT DO UPDATE` hitting a row twice.
5. Commit with the **unnest pattern**:
```sql
INSERT INTO ts_kv (entity_id,key_id,ts,bool_v,str_v,long_v,dbl_v,json_v)
SELECT * FROM unnest($1::uuid[],$2::int[],$3::bigint[],$4::boolean[],$5::text[],$6::bigint[],$7::float8[],$8::jsonb[])
ON CONFLICT (entity_id,key_id,ts) DO UPDATE
SET bool_v=EXCLUDED.bool_v, str_v=EXCLUDED.str_v, long_v=EXCLUDED.long_v, dbl_v=EXCLUDED.dbl_v, json_v=EXCLUDED.json_v;

INSERT INTO ts_kv_latest (entity_id,key_id,ts,bool_v,str_v,long_v,dbl_v,json_v)
SELECT * FROM unnest(...)
ON CONFLICT (entity_id,key_id) DO UPDATE SET ts=EXCLUDED.ts, bool_v=EXCLUDED.bool_v, str_v=EXCLUDED.str_v,
  long_v=EXCLUDED.long_v, dbl_v=EXCLUDED.dbl_v, json_v=EXCLUDED.json_v
WHERE ts_kv_latest.ts <= EXCLUDED.ts;
```
6. After commit: update Redis latest hash, publish `RtUpdate` to NATS subject `rt.{tenant}.{entity}`, ack the stream batch.

**Sanity rules:** reject `ts > now + 10 min` (configurable) and `ts < 2000-01-01`; missing `ts` → server time; numeric `NaN/Inf` → string or rejected per profile; max key length 255; max value size (string 64 KB, JSON 1 MB); per-message key limit 1 000.

## 6. Backends
### 6.1 TimescaleDB (default)
Hypertable on `ts` (ms bigint) with 1-day chunks; compress after 7 days (segment by `entity_id,key_id`, order `ts DESC`); **late data older than the compression boundary is slow** — keep `compress_after ≥ max lateness`. **Retention classes:** hypertable retention is table-wide, so route tenants to `ts_kv_30d / ts_kv_365d / ts_kv_forever` by profile retention and apply `add_retention_policy` per table; finer per-tenant TTL via batched deletes. **Continuous aggregates:** `ts_1m`, `ts_1h`, `ts_1d` (avg, min, max, sum, count per entity/key) for numeric keys; the **rollup router** picks the coarsest rollup ≤ requested interval when the range is large.

### 6.2 ClickHouse (scale)
```sql
CREATE TABLE ts_kv (
  entity_id UUID, key_id Int32, ts Int64,
  kind Enum8('bool'=1,'long'=2,'double'=3,'string'=4,'json'=5),
  l Int64, d Float64, s String, j String
) ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(toDateTime(intDiv(ts,1000)))
ORDER BY (entity_id, key_id, ts)
TTL toDateTime(intDiv(ts,1000)) + INTERVAL 365 DAY;
-- latest via AggregatingMergeTree + argMaxState(ts), or a small ReplacingMergeTree keyed (entity_id,key_id)
```
Use async inserts or large batches (≥ 10 k rows); idempotency via `ReplacingMergeTree` (eventual) — queries use `FINAL` only where needed.

### 6.3 SQLite/Pebble (lite and edge)
One DB file per day (`ts_YYYYMMDD.db`) → retention = delete file; WAL mode; `WITHOUT ROWID` tables on `(entity,key,ts)`; latest in a separate small table.

## 7. Read path
- **Raw:** `ts BETWEEN start AND end ORDER BY ts DESC|ASC LIMIT n`, multi-key in one scan (`key_id = ANY($keys)`).
- **Aggregated:** bucket `floor(ts/interval)*interval`; calendar buckets (`WEEK`, `WEEK_ISO`, `MONTH`, `QUARTER`) computed in the request timezone. Result timestamp default = **interval midpoint** (ThingsBoard-compatible), selectable `start|mid|end`. `COUNT` works on all types; `SUM/AVG` numeric only; strings/bools with MIN/MAX follow documented ordering.
- **Fill:** optional `fill=none|null|previous|linear` + `maxGap`.
- **Limits:** max points per response (default 50 000); server downsamples (LTTB) when `maxPoints` supplied.
- **Latest:** Redis hash `latest:{entity}` first, else store; `keys=*` supported with cap.
- **Units:** values stored in the key's *base unit*; `unitsPreference` (user/tenant) converts at read via the units registry (temperature offsets handled).

## 8. Attributes
| Aspect | Design |
|---|---|
| Storage | `attribute_kv(entity_id, scope, key_id, typed value, last_update_ts, version)` in Postgres; write-through cache (Redis + in-proc) |
| Scopes | CLIENT (device-written), SERVER (platform-only), SHARED (platform-written, pushed to device) |
| Write | Batched upsert; `version` increments (used by edge sync and optimistic updates) |
| Notifications | On change emit `ATTRIBUTES_UPDATED` (rule engine) and, for SHARED, a downlink to the device session |
| Read | By scope + keys; list keys; bulk by entity list |
| Delete | By scope + keys; emits `ATTRIBUTES_DELETED` |
| Limits | Max attributes per entity (profile), value size caps |

## 9. Retention, export, deletion
- Retention worker enforces `tenant_profile.retention.*`; deletes by retention-class table or batched `DELETE` with `LIMIT`; emits usage metrics.
- **Delete range** API supports `deleteAllDataForKeys`, `startTs/endTs`, `rewriteLatestIfDeleted` (recompute latest from remaining rows).
- **Export:** async job → CSV/NDJSON/Parquet to object store (signed URL), honours permissions and column selection.
- **GDPR delete:** entity deletion enqueues a purge job across `ts_kv*`, rollups, latest, caches.

## 10. Observability and SLOs
Metrics: `iotp_ts_write_points_total`, `…_batch_seconds`, `…_batch_rows`, `…_dedup_dropped_total`, `…_query_seconds{type}`, `…_rollup_hit_ratio`, `…_latest_cache_hit_ratio`, `…_rt_publish_lag_seconds`. SLOs: write commit p99 < 200 ms at 50 k rows/s; raw query (1 key, 24 h) p95 < 150 ms; aggregated 30-day, 5-min buckets p95 < 400 ms via rollups.

## 11. Failure modes
| Failure | Behaviour |
|---|---|
| Store slow/down | Write stream returns retryable error → rule engine retries pack; consumption slows (lag) |
| Duplicate delivery | Idempotent upsert, latest guarded by `ts` |
| Hot entity | Per-entity ordering serialises writes; bulk uploads use batch API with sorted ts |
| Cache loss | Latest falls back to store; warm-up on demand |

## 12. Testing
Conformance suite run against every `Store` implementation (ordering, upsert, aggregation correctness vs reference implementation, calendar buckets with DST); property tests for dedupe window; load: 100 k rows/s on Timescale reference box; chaos: kill DB mid-batch → no lost acks.

## 13. Task checklist
- [ ] `tsstore.Store` interface + conformance suite
- [ ] Timescale driver (hypertable, compression, retention classes)
- [ ] Write API (gRPC stream), shard/batch/dedupe, strategies
- [ ] Latest store + Redis cache + RT publish
- [ ] Query engine: raw, aggregates, calendar buckets, fill, LTTB
- [ ] Rollups (continuous aggregates) + router
- [ ] Attributes service + cache + notifications
- [ ] Retention worker, delete API, export jobs
- [ ] Units registry + conversion
- [ ] ClickHouse driver (P2) · SQLite driver (edge)
- [ ] Dashboards/alerts; load/chaos tests
