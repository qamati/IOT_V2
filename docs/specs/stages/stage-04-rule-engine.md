# Stage 4 — Rule Engine

> **Estimate:** 36 eng-weeks · **Depends on:** Stage 3 · **Unlocks:** 5, 7, 8, 11
> **Specs:** `services/09-rule-engine.md`, `10-rule-nodes-catalog.md`, `frontend/03-rule-chain-editor.md`
> **MVP gate:** completing this stage + a thin slice of stage 6 = the first demo-able platform.

## 1. Goal
Programmable message processing with ThingsBoard-equivalent semantics: chains, relations, queues with submit/processing strategies, safe scripting, debug/replay, hot reload — and the **default root chain** replacing the stage-3 direct-save worker.

## 2. Scope
**In (P0):** `pkg/rules` (Message, Node SPI, descriptors), shard executor + tracker, chain compiler/validator, revisions (draft/publish), queues (strategies, retries, DLQ), `expr` + `goja` engines, durable timers, P0 nodes (input/output/rule-chain, msg-type(+switch), filter/transform script, enrich originator attrs/telemetry, save telemetry/attributes, create/clear alarm hooks (stubbed until stage 5), log, REST call), debug capture, rule-chain editor v1 (canvas, inspector, validation, publish), device-profile → chain mapping.
**P1 (spillover):** remaining filters/enrichments/transforms/actions, replay-in-sandbox, tenant fairness/isolation, WASM host.
**Out:** alarm rules, external connectors beyond REST/MQTT, plugin SDK (stage 16).

## 3. Deliverables
1. `cmd/ruleengine` + `pkg/rules` with property-tested ordering guarantees.
2. Queue configuration API/UI (`queue` table) + consumer wiring from transports.
3. Script test endpoint + Monaco integration.
4. Default root chain seeded per tenant (msg-type switch → save TS / save attrs → log).
5. Debug stream (`RULE_DEBUG` via realtime stub or SSE) + node counters in UI.
6. Metrics, dashboards, runbook (DLQ replay, hot-reload rollback).
7. `docs/rules/` author guide with 10 example chains.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Runtime core | Executor, tracker, nodeCtx, hop guard, panic isolation, backpressure | 6 |
| Compiler & revisions | Schema, validation, hot swap, publish events, rollback | 4 |
| Queues | Pack consumer, 5 submit + 6 processing strategies, DLQ, replay CLI | 5 |
| Scripting | `expr` library (TBEL-like), `goja` sandbox/timeouts, test endpoint | 4 |
| Durable timers | Table + pollers + delay node | 2 |
| Nodes (P0) | ≈ 20 nodes + descriptors + golden tests | 6 |
| Debug/trace | Capture, sampling, store, stream, replay (P1) | 3 |
| Fairness/limits | DRR, tenant concurrency/CPU budgets, usage counters | 2 |
| Frontend | Editor v1 (descriptor-driven), publish flow, debug overlay | 4 |

## 5. Acceptance criteria
- **Ordering:** property test (random delays, rebalances) never reorders messages of one originator within a queue.
- **Throughput:** 3-node chain (switch → script transform → save TS) ≥ 20 k msg/s per 4 vCPU; added p99 latency < 5 ms (no external calls).
- **Retry semantics:** each processing strategy verified with injected failures/timeouts; DLQ receives poison messages after the configured retries; replay re-injects them idempotently.
- **Hot reload:** publishing a new revision under 10 k msg/s loses/duplicates nothing beyond at-least-once; in-flight messages finish on the old revision.
- **Isolation:** a tenant with an infinite-loop `goja` script cannot degrade other tenants' p99 by > 10 % (timeouts + DRR + limits).
- **Timers:** delay node fires after restart/rebalance; no double-fire under concurrent pollers.
- **Editor:** a new user builds, tests and publishes a 6-node chain in < 5 minutes (usability test).

## 6. Demo (exit)
Create a chain: temperature > 80 → enrich with device attribute `limit` → transform → REST call to a webhook; send test messages, watch live debug overlay, break the webhook (breaker + failure relation), publish a fix, replay a failed message in sandbox.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Semantic drift from TB expectations | Golden-chain suite derived from TB docs behaviours; documented deviations |
| JS sandbox escapes/resource abuse | `expr` default; JS opt-in with interrupts + memory budget; WASM for untrusted |
| Debug storage blow-up | Sampling, per-tenant caps, auto-off windows |
| Editor/server validation divergence | Shared JSON fixtures asserting identical errors |

## 8. Definition of Done
- [ ] Direct-save worker removed; root chain is the only persistence path
- [ ] Ordering/hot-reload/timer property & chaos tests in CI
- [ ] Node authoring guide + descriptors verified by editor snapshot tests
- [ ] Benchmarks recorded as regression gates
