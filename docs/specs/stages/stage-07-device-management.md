# Stage 7 — Device Management (RPC, OTA, gateways, bulk ops)

> **Estimate:** 24 eng-weeks · **Depends on:** Stages 2–4, 6 (control widgets) · **Unlocks:** 8, 9
> **Specs:** `services/14-rpc.md`, `15-ota.md`, `04-device-registry.md` (claiming, bulk), `05-transport-mqtt.md` (gateway, protobuf, custom topics)

## 1. Goal
Operate fleets, not just connect them: reliable commands, firmware/software delivery with tracking, gateway-connected child devices, bulk onboarding, and complete device-profile capabilities.

## 2. Scope
**In (P0/P1):** RPC service (one/two-way, **persistent** with state machine, client-side RPC via rules), control widgets wired to RPC, OTA packages (upload/URL, checksums), assignment by profile/device, chunk delivery over MQTT/HTTP, OTA state tracking + widgets, **gateway MQTT topics** (`v1/gateway/*`) with child-device auto-creation, claiming, bulk CSV import/export/rotation, connectivity snippet generator, profile features: **protobuf payloads, custom topic filters**, unit registry hooks.
**P2:** OTA campaigns (waves, gates, rollback), signed packages/delta updates, RPC broadcast campaigns, MQTT v5 batch topic.
**Out:** CoAP/LwM2M OTA (stage 8).

## 3. Deliverables
1. `cmd/rpc`, `cmd/ota`; gateway handlers in `tr-mqtt`; profile validator extensions.
2. Persistent-RPC scheduler (session-connected triggers + sweeper) and idempotency keys.
3. Chunk server with cache + bandwidth shaping; OTA state tracker.
4. UI: RPC history/terminal, OTA package manager + progress, bulk import wizard, gateway view (children tree).
5. Device-simulator scenarios: sleepy devices, gateway with 500 children, firmware agent.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| RPC | State machine, routing/waiters, persistent scheduler, client-side path, limits/allow-lists, API | 5 |
| OTA | Packages, presigned upload, assignment, chunk server, state tracker, widgets | 6 |
| Gateway support | Child sessions, auto-create, RPC/attrs to children, rate limits | 3 |
| Profiles | Protobuf descriptors, custom topics, validation dry-run, Sparkplug flag plumbing | 3 |
| Bulk & claim | CSV import/export jobs, credential rotation, claiming flow | 2 |
| Frontend | RPC terminal/history, OTA UI, bulk wizard, gateway tree | 4 |
| Campaign MVP (P2) | Waves + gates + rollback simulator | 1 (seed only) |

## 5. Acceptance criteria
- **RPC:** state machine invariants hold under chaos (kill transport/rpc mid-delivery); persistent RPC delivered when an offline device reconnects, in creation order with `maxInFlight=1`; p95 two-way latency < 300 ms for connected MQTT device.
- **OTA:** interrupted download resumes (Range/chunk) and verifies checksum; 10 k simulated devices fetch a 20 MB image with tenant bandwidth shaping honoured; stalled devices flagged after `stallTimeout`.
- **Gateway:** 500 children per gateway connection sustain 5 k msg/s; child connect/disconnect events and state correct; RPC reaches the right child.
- **Bulk:** 50 k-row CSV import completes in < 5 min with per-row error report and resumability.
- **Security:** chunk requests authorised by assignment; RPC method allow-lists enforced; audit entries for every command and assignment.

## 6. Demo (exit)
Upload firmware v1.2, assign to a profile, watch 100 simulated devices progress to UPDATED (one forced FAILED); send a persistent RPC to an offline device, bring it online and see delivery; import 1 000 devices via CSV; operate a gateway with 50 children.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Device-side idempotency assumptions | Document `rpcId` in params + at-least-once semantics; sample firmware code |
| Large-fleet download storms | Chunk cache, shaping, jittered rollout (campaigns in P2) |
| Gateway child explosion | Per-gateway child limits, auto-create rate limits |

## 8. Definition of Done
- [ ] RPC/OTA state machines model-checked (property tests) and chaos-tested
- [ ] Firmware/agent sample code (C/Arduino/Python/Go) published with docs
- [ ] Dashboards for RPC failures and OTA fleet versions
