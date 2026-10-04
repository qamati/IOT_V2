# Stage 11 — Analytics (calculated fields, stream analytics, AI)

> **Estimate:** 28 eng-weeks · **Depends on:** Stages 3–4 (data + rules), 6 (UI) · **Unlocks:** KPI solutions, predictive features
> **Specs:** `services/25-analytics.md` (+ `10-rule-nodes-catalog.md` for `send_to_cf` and `ai_request` nodes)

## 1. Goal
Turn raw signals into **derived, trustworthy data** — per-entity formulas, relation-based aggregation/propagation, geofencing, windowed analytics, anomaly detection and forecasts — and make them easy to author (with optional AI assistance).

## 2. Scope
**In (P1):** calculated-field engine (SIMPLE, SCRIPT, GEOFENCING, PROPAGATION, RELATED_ENTITIES_AGGREGATION, ENTITY_AGGREGATION), argument store (latest + rolling windows), DAG validation, backfill/recompute, quotas, debug/test tooling, UI editor, `send_to_cf` node, KPI templates (OEE, MTBF/MTTR, energy intensity).
**P2:** stream windows/CEP-lite via continuous aggregates, anomaly detectors (z-score/MAD/EWMA/STL), model registry + ONNX model server, forecasting jobs, AI authoring (NL → drafts) + `ai_request` node with guardrails.
**Out:** model training platform (external), full BI/notebook environment.

## 3. Deliverables
1. `cmd/analytics` + `pkg/analytics` (embedded at edge for simple/script/geofence).
2. CF editor with live **test on history**, dependency graph view, debug traces.
3. Backfill job runner with progress/cancel; per-tenant quotas and auto-pause + notifications.
4. KPI solution templates packaged for stage 12 (OEE, energy).
5. (P2) model server, anomaly alarms integration, forecast widgets, AI authoring with approval flow.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| CF core | Model, registry, index, DAG checks, argument store, snapshots/rebuild | 6 |
| Evaluators | SIMPLE/SCRIPT (`expr`), outputs (DIRECT / rule-engine), event-time/late data | 4 |
| Geofencing | Zones, R-tree, hysteresis/debounce, events | 3 |
| Propagation & aggregation | Path resolver, incremental accumulators, reconcile, entity-wide partials | 5 |
| Backfill/quotas/debug | Jobs, quotas, pause/notify, test API, traces | 3 |
| Frontend | Editor, tester, graph, KPI gallery | 3 |
| Anomaly/forecast (P2) | Detectors, baselines, model registry/server, forecast keys | 3 |
| AI authoring (P2) | Tool schemas, draft pipeline, guardrails, `ai_request` node | 1 (seed) |

## 5. Acceptance criteria
- **Correctness:** incremental aggregation equals from-scratch recompute (property tests with random add/remove/update streams); late data within tolerance corrects outputs idempotently.
- **Performance:** 200 k SIMPLE evaluations/s/node; REL_AGG across 10 k children updates parent within 1 s p95; rolling-window memory bounded per tenant quota.
- **Geofencing:** zero false ENTERED/LEFT flaps within configured hysteresis in jitter datasets; correct at poles/anti-meridian.
- **Safety:** cycles rejected at save; runaway CFs auto-paused with notification; quotas enforced under load.
- **UX:** author a derived KPI (e.g., OEE) from a template in < 10 minutes, validated against 24 h of history before enabling.
- **(P2) ML:** model scoring p99 < 50 ms with batching; shadow-mode divergence report works; AI drafts never activate without approval and fail closed on schema violations (prompt-injection suite).

## 6. Demo (exit)
Create a geofence CF for a vehicle fleet (ENTERED/LEFT notifications), a propagation CF summing child-meter power to building level, and an OEE KPI with backfill; show test-on-history before enabling.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Hidden evaluation cost | Per-CF cost accounting + quotas, dashboards of top consumers |
| Semantics confusion (event-time vs arrival-time) | Clear docs + preview tooling + defaults that avoid surprises |
| AI misuse/cost | Opt-in, budgets, redaction, approval gate, audit |

## 8. Definition of Done
- [ ] Property/time-travel tests in CI; load benchmarks recorded
- [ ] CF cookbook (10 patterns) + KPI templates documented
- [ ] Edge parity matrix (which CF types run locally) published
