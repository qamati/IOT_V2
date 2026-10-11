# Stage 10 — SCADA

> **Estimate:** 64 eng-weeks · **Depends on:** Stages 5, 6, 8, 9 · **Unlocks:** industrial customers; stage 14 (62443 mapping)
> **Specs:** `services/24-scada.md`, `frontend/04-scada-hmi-editor.md`

## 1. Goal
Deliver **real SCADA semantics** — tags with quality, templates, drivers, historian compression, ISA-18.2 alarm management, safe control — running in the cloud and on the edge, with an ISA-101-style HMI.

## 2. Scope
**Phase A (parity, ≈ 12 EW; can run during stage 6):** SVG symbol format + sanitiser, symbol runtime widget, first 40 symbols, tag/behaviour binding to device keys.
**Phase B (core, ≈ 30 EW):** tag server (namespace, UDTs, scaling, deadband, quality, derived tags), driver integration (`pkg/connectors` via scan classes/subscriptions), historian (compression + interpolation + aggregates), ISA-18.2 alarm manager (state machine, shelving/suppression/OOS, KPIs), control service (SBO, interlocks, approvals, verify, audit chain), expression dialect shared Go/TS.
**Phase C (studio & operations, ≈ 22 EW):** HMI runtime (screens, faceplates, alarm banner/summary, trends), HMI studio (binding/animation designers, simulator, lint), OPC UA server (read-mostly), Sparkplug host, recipes, redundancy, SOE journal, FAT/SAT generators, edge-local operation.
**Out:** batch/MES, DNP3/IEC 61850 (on demand).

## 3. Deliverables
1. `cmd/scada`, `pkg/scada` (shared with edge), tag/UDT/alarm/control schemas + APIs.
2. Shared expression dialect with conformance vectors (Go `expr` ↔ TS interpreter).
3. HMI runtime + studio; 100+ symbols (standard + ISA-101 sets); faceplate templates for common equipment.
4. Alarm KPI report (EEMUA 191 metrics), control log with hash chain, SOE view.
5. Simulation driver + scenario library; FAT/SAT document generator.
6. Operator and engineering manuals; alarm philosophy template.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Symbols & runtime (A) | Format, sanitiser, binding engine, widget, 40 symbols | 12 |
| Tag server | Model, namespace API, UDT engine, scaling/deadband/quality, derived tags | 8 |
| Drivers/scan | Scan classes, subscriptions, write path with verify | 5 |
| Historian | Compression (deadband, SDT), interpolation, aggregates, backfill | 6 |
| Alarm manager | State machine, properties, shelving/suppression/OOS, first-out, KPIs | 8 |
| Control | SBO, interlocks, modes, approvals/e-sign, audit chain, recipes | 7 |
| HMI runtime/studio | Screens, faceplates, banner/summary, trends, designers, simulator, lint | 12 |
| Integration/ops | OPC UA server, Sparkplug host, redundancy, SOE, edge-local | 4 |
| Validation | FAT/SAT generator, simulation scenarios, chaos | 2 |

## 5. Acceptance criteria
- **Scale:** 200 k tags / 20 k updates/s per cloud node; 10 k tags / 2 k updates/s on a 1 GB edge; alarm evaluation p99 < 5 ms/update.
- **Alarm correctness:** every ISA-18.2 transition covered by table-driven tests; shelved alarms auto-unshelve; chatter suppression verified; KPI report matches hand-computed fixtures.
- **Control safety:** SBO never operates without a valid selection; interlock violations denied with reason; four-eyes enforced; write verification flags mismatches; duplicate commands suppressed; cloud/edge arbitration test during partition passes.
- **Historian:** SDT-compressed signals reconstruct within tolerance (property test); storage reduction ≥ 5× on typical analog data; time-aligned multi-tag queries correct with quality filtering.
- **HMI performance:** 1 000 animated elements @ 30 fps (laptop), 300 on Pi 4 kiosk; screen switch < 300 ms; no DOM growth over 24 h.
- **Usability:** operator completes alarm ack → shelve → control with approval in a scripted scenario within target times; ISA-101 lint reports zero violations on shipped templates.
- **Security:** deny-by-default writes; step-up auth for critical tags; tamper-evident control log verified.

## 6. Demo (exit)
Water-treatment simulation with 5 000 tags: UDT-instantiated pumps/valves, HMI overview → area → faceplates, alarm flood scenario with first-out and shelving, SBO start of a pump with approval, historian trends with compression, edge node continuing control display during a cloud outage.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Scope breadth | Phase gates; ship A early; domain advisors review alarm/control design |
| Safety liability | Clear non-safety-function statement; SIS stays independent; audit/e-sign; deny-by-default |
| Driver reliability | Interop lab with real PLCs, recorded traces, soak tests |
| Expression dialect divergence | Shared conformance vectors in CI for both implementations |

## 8. Definition of Done
- [ ] FAT/SAT generator used on the demo project
- [ ] Alarm management KPIs validated by an external reviewer
- [ ] Security review against IEC 62443-3-3 SR mapping (see stage 14)
