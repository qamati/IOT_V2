# Service 08 — Message Bus (`pkg/bus` + topic catalog)

> **Stage:** 2 · **Deliverable:** library + ops runbook (no standalone binary) · **Priority:** P0

## 1. Purpose
A small, stable abstraction over Kafka (cluster), NATS JetStream (lite/edge) and an in-memory bus (tests) so every service speaks one contract: **keyed, partitioned, durable, at-least-once** messaging with consumer groups.

## 2. Interfaces
```go
type Message struct {
    Topic     string
    Key       []byte            // partition key: originator/entity id bytes
    Value     []byte            // protobuf Envelope
    Headers   map[string]string // trace, tenant, schema version, attempt, dlq reason
    Timestamp time.Time
}
type Producer interface {
    Produce(ctx context.Context, m Message) error           // waits for durable ack (acks=all)
    ProduceAsync(m Message, cb func(error))                  // for batching callers
    Close() error
}
type Handler func(ctx context.Context, batch []Message) error   // return nil => commit
type Consumer interface {
    Subscribe(ctx context.Context, topics []string, group string, h Handler, opts ConsumeOpts) error
}
type ConsumeOpts struct {
    MaxBatch int; MaxWait time.Duration; Concurrency int
    OnAssigned, OnRevoked func(partitions []int32)   // state hand-off hooks (rule engine, devstate)
    StartOffset Offset                                  // latest | earliest | compacted-snapshot
}
type Admin interface {
    EnsureTopic(ctx context.Context, spec TopicSpec) error  // partitions, retention, compaction, replication
}
```
**Partitioner:** `murmur3_32(key) % partitions`, identical in all producers (in Kafka set a custom partitioner so non-Go producers and tools agree).

## 3. Implementations
| Driver | Use | Notes |
|---|---|---|
| `kafka` (franz-go) | Cluster | idempotent producer, `acks=all`, zstd/lz4, `linger.ms` 5–20, cooperative-sticky assignor, manual commit per batch after handler success |
| `nats` (JetStream) | Lite, edge | Stream per topic with subjects `topic.{p}`; durable pull consumers; `p = hash(key) % N`; ack after handler |
| `mem` | Unit/integration tests | Same semantics incl. rebalance simulation hooks |
All drivers run the **same conformance suite**: ordering per key, at-least-once under crash, consumer-group rebalance callbacks, batch commit, DLQ routing, header propagation, backpressure (producer blocks/queues bounded).

## 4. Topic catalog
| Topic | Producers → Consumers | Key | Retention | Notes |
|---|---|---|---|---|
| `re.{queue}` (e.g. `re.main`, `re.highprio`, `re.tenant.{id}`) | transports, integrations, edge-hub, scheduler, core → `ruleengine` | originator id | 24 h–3 d | Partitions 48–96; per-queue config from `queue` table |
| `core.events` | transports/rule engine → `core`/`devreg`/`alarm` | entity id | 24 h | Session events, entity actions needing core |
| `entity.events` | `core` outbox relay → all caches, `edge-hub`, `scada`, `analytics`, `audit` | entity id | 3 d | CREATED/UPDATED/DELETED/ASSIGNED |
| `devstate.activity` | transports → `devstate` | device id | 6 h | coalesced |
| `devstate.snapshot` | `devstate` → `devstate` | device id | **compacted** | rebuild on assignment |
| `notify.transport.{nodeId}` | `rpc`/`telemetry`/`ota` → one transport node | device id | 1 h | per-node downlink |
| `alarm.events` | `alarm` → `notify`, `ruleengine`, `scada`, `audit` | alarm id | 3 d | |
| `rpc.events` | `rpc` → `ruleengine`, `audit`, `realtime` | rpc id | 3 d | |
| `audit.events` | all → `audit` | tenant id | 7 d | |
| `usage.stats` | transports/rule engine/api → `audit-usage` | tenant id | 24 h | counters, aggregated |
| `analytics.cf.{n}` | `ruleengine`/`telemetry` → `analytics` | target entity id | 24 h | calculated-field triggers |
| `scada.samples` | drivers → `scada` | tag id | 24 h | |
| `integration.{id}` | external → integration runtime | partition key from connector | 24 h | |
| `dlq.{domain}` | any → ops tooling | original key | 14 d | headers: reason, attempts, first_seen |
| `retry.{domain}.{5s|1m|15m}` | any → same consumer | original key | 1 d | delayed retries (consumer pauses until `not_before`) |
**Realtime fan-out does not use the durable bus:** `rt.{tenant}.{entity}` NATS core subjects (ephemeral) or in-process in lite mode (see doc 16).

## 5. Envelope and schema evolution
`Envelope` protobuf (see doc 04 §6) with `trace_context`, `tenant_id`, `originator`, `type`, `ts`, `metadata`. Schemas under `api/proto`, validated by `buf breaking` in CI; consumers ignore unknown fields; breaking changes use a new `type`/topic version. Optional schema registry (Confluent-compatible) for external producers.

## 6. Delivery semantics and patterns
| Pattern | Guidance |
|---|---|
| At-least-once | Handlers must be idempotent (natural keys, upserts) |
| Ordering | Only per partition key; never assume global order |
| Poison messages | Retry with backoff (retry topics) → DLQ with reason; replay tool `iotpctl dlq replay --topic dlq.re.main --since 1h` |
| Delayed work | Not native to Kafka: use retry topics for ≤ 15 min; **durable timers table** for longer (see rule engine) |
| Backpressure | Consumer `Pause()` partitions when downstream saturated; producers bounded + timeout → error to caller |
| Transactions | Avoid Kafka transactions; use outbox for DB↔bus atomicity |
| Large payloads | > 512 KB → object store reference in message (claim-check pattern) |

## 7. Capacity and ops
- Partition count: ≥ 3× expected max consumers (start 48 for hot topics); increase = key remap → plan as migration (drain, switch, or use new topic generation).
- Replication factor 3, `min.insync.replicas=2`, rack awareness, unclean leader election off.
- Tenant isolation (L2): dedicated `re.tenant.{id}` topics — limit count (Kafka metadata overhead) and reuse via **tenant→topic shards** (e.g., 64 shards by tenant hash) for most tenants.
- Monitoring: consumer lag per group/partition, produce latency p99, ISR shrink, disk usage, rebalance frequency; alerts on lag growth and rebalance storms.
- DR: MirrorMaker 2/cluster linking for multi-region; `entity.events` replay can rebuild caches/read models.

## 8. Testing
Conformance suite across drivers; chaos (broker kill, network partition, slow consumers); soak with rebalances every 30 s while verifying per-key ordering and no gaps; benchmark producer throughput (≥ 200 k msg/s/node, p99 < 20 ms).

## 9. Task checklist
- [ ] Interfaces + `mem` driver + conformance suite
- [ ] Kafka driver (producer/consumer/admin, custom partitioner, rebalance hooks)
- [ ] NATS JetStream driver
- [ ] Topic provisioning as code (`deploy/topics.yaml`) + `iotpctl topics apply`
- [ ] Envelope + proto + `buf` CI gates
- [ ] Retry/DLQ helpers + replay CLI
- [ ] Lag/ISR dashboards + alerts
- [ ] Runbooks (partition increase, tenant topic sharding, DR)
