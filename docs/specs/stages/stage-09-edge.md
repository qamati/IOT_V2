# Stage 9 — Edge (hub + runtime)

> **Estimate:** 48 eng-weeks · **Depends on:** Stages 3–5, 8 (connectors) · **Unlocks:** 10 (edge SCADA)
> **Specs:** `services/22-edge-hub.md`, `23-edge-runtime.md`, `15-ota.md` (self-update), `19-field-gateway.md` (shared connectors)

## 1. Goal
Sites keep running when the cloud doesn't: an autonomous Go edge node that collects, decides and stores locally, then synchronises with the cloud losslessly — managed at fleet scale.

## 2. Scope
**In (P1):** `edge-hub` (enrollment + internal CA, mTLS stream, sequence/ack protocol, assignment resolver, downlink pump + compaction, uplink handlers, fullSync, versioning N-2, status/metrics, remote commands), `edge` runtime (lite + full profiles: config DB, Pebble store-and-forward with priority lanes, sync agent, local broker, local rules/alarms, local API/auth/UI, connectors via `pkg/connectors`), A/B self-update with health gate and rollback, packaging, edge pages in console, edge rule-chain templates and push-to-cloud/edge nodes.
**P2:** edge clustering (HA pair), edge-of-edge hierarchy, edge groups, local analytics/ML (ONNX/WASM), local OPC UA server, secure remote tunnel UX.
**Out:** SCADA specifics (stage 10 — but edge must host its engine).

## 3. Deliverables
1. `cmd/edge-hub`, `cmd/edge`, `proto/iotp/edge/v1`, assignment resolver library.
2. Store-and-forward queue library with capacity/downsample policies and power-loss tests.
3. Supervisor + updater + signed release pipeline; `.deb/.rpm/.apk`, Docker, systemd units.
4. Console: edge list/detail (status, backlog, assignments, commands), enrollment wizard, edge templates.
5. Constrained-hardware CI (QEMU ARM, cgroup memory limits) and network-chaos suite (toxiproxy).
6. Operator docs: install, enroll, upgrade, troubleshoot, sizing guide.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Hub protocol/core | Proto, streams, leases/fencing, cursors, acks, flow control | 8 |
| Enrollment/PKI | Tokens, internal CA, renewal/revocation, TPM-optional keys | 4 |
| Assignment & downlink | Resolver (closure/refcount), outbox filter, compaction, fullSync pager | 6 |
| Uplink handling | Telemetry/attrs/alarms/device-create/RPC/events, dedupe cursor, conflict rules | 5 |
| Edge runtime core | Composition, `config.db`, hot reload, local auth/API/UI embed | 8 |
| Queue & sync agent | Lanes, group commit, policies, downsampler, backoff/handshake, transactional apply | 6 |
| Local intelligence | Rules/alarms/CF integration, local broker, time quality | 4 |
| Update/supervision | A/B updater, watchdog, rollback, signing, packaging | 4 |
| Frontend | Edge pages, wizard, backlog views | 2 |
| Test infra | Power-loss, offline soak, chaos, hardware lab | 1 (continuous) |

## 5. Acceptance criteria
- **Autonomy:** 72 h offline at 5 k tags/1 Hz → local rules/alarms keep working, disk caps respected, **L0/L1 never dropped**, telemetry downsampled per policy, ordered drain after reconnect, **zero duplicates** after dedupe.
- **Power loss:** 1 000 randomized hard resets during writes → no DB corruption; queue replays exactly once.
- **Sync correctness:** property test over random disconnect/reorder/duplicate schedules converges config state and yields gapless up-seq.
- **Footprint:** edge-lite ≤ 64 MB RSS; edge-full ≤ 150 MB RSS at 10 k tags (ARM A53); cold start < 3 s.
- **Scale:** one hub instance sustains 10 k idle edges and 2 k active streams at 20 k up-events/s.
- **Upgrade:** signed update with injected failure auto-rolls back within 3 min; fleet upgrade campaign of 500 virtual edges completes with gates.
- **Security:** revoked cert rejected within 5 min; edge cannot access unassigned entities (authz tests); enrollment tokens single-use.

## 6. Demo (exit)
Enroll an edge from the UI, assign a device profile + rule chain + dashboard, collect from a Modbus simulator, cut the network for 30 minutes while an alarm fires locally, restore and show cloud alarm + backfilled telemetry, then push a signed upgrade with automatic rollback on a bad build.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Conflict semantics surprising users | Documented per-class source of truth + conflict log UI |
| Disk-full/long-offline behaviours | Lane policies, quotas, alerts, replay throttling |
| Clock problems on RTC-less boards | Time-quality flags, drift detection, boot-ID based correction |
| Hub statefulness | Leases + fencing; stateless apart from cursors in Postgres |

## 8. Definition of Done
- [ ] Power-loss, offline-soak, chaos and N-2 compatibility suites in CI/nightly
- [ ] Sizing guide with measured numbers per hardware class
- [ ] Signed release + rollback runbook exercised in staging
