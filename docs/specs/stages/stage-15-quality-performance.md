# Stage 15 — Quality, Performance & Release Engineering

> **Estimate:** 16 eng-weeks of dedicated platform work (plus per-stage testing effort that is already inside each stage) · **Starts:** stage 0 · **Closes:** before GA
> **Specs:** `00-overview/05-tech-stack-and-repo-layout.md §6`, each service's *Testing* section

## 1. Goal
Make quality **measurable and automatic**: a test pyramid that catches regressions at the cheapest layer, performance budgets enforced in CI, chaos/soak confidence, compatibility guarantees (devices, edge N-2, API), and a predictable release process.

## 2. Test strategy
| Layer | What | Tools | Gate |
|---|---|---|---|
| Unit | Pure logic, state machines, expression engines, parsers | `go test -race -shuffle`, vitest, `rapid` property tests | PR, coverage ≥ 80 % on `pkg/*` and domain packages |
| Component/integration | Service + real deps (Postgres/Timescale/Kafka/NATS/Redis/MinIO) | testcontainers-go | PR |
| Contract | MQTT/HTTP/CoAP transcripts, OpenAPI diff, protobuf `buf breaking`, bus envelope schemas, TB-compat device suite | custom harness, schemathesis, buf | PR |
| End-to-end | Browser flows across the stack | Playwright against compose/kind stack | PR (smoke) / nightly (full) |
| Performance | Ingest, rule engine, queries, WS fan-out, edge footprint | `tools/loadgen`, k6, pprof/benchstat | nightly + release; regression > 10 % fails |
| Chaos/resilience | Kill pods/nodes/brokers, partition networks, clock jumps, disk full | Chaos Mesh/Litmus, toxiproxy | weekly + pre-release |
| Soak | 24–72 h steady load with rebalances | loadgen + dashboards | pre-release |
| Fuzz | Decoders, sanitisers, expression engines, protocol parsers | `go test -fuzz`, OSS-Fuzz-style corpora | nightly |
| Security | See stage 14 | — | PR/nightly |
| Accessibility/visual | Storybook + Playwright snapshots, axe | Chromatic/Playwright | PR |
| Upgrade/compat | Old→new under load; edge N-2; DB migrations up/down | CI matrix | release |

## 3. Performance program
- **Reference rigs** (documented hardware/cloud shapes) for ingest, rule engine, TS store, realtime, edge (ARM A53 board + QEMU).
- **Scenario catalogue** (YAML): `ingest-steady`, `ingest-burst`, `reconnect-storm`, `rule-heavy`, `dashboard-heavy`, `alarm-flood`, `edge-offline-72h`, `ota-storm`, `scada-5k-tags`.
- **Budgets-as-code:** p50/p95/p99 latency, throughput, RSS/CPU per component; stored with history (Grafana + benchstat); PRs touching hot paths run micro-benchmarks automatically.
- **Profiling:** continuous profiling (Pyroscope/Parca) in staging; flame-graph review per release.
- **Capacity validation:** results feed the stage-13 capacity model.

## 4. Compatibility guarantees
| Surface | Guarantee | Verification |
|---|---|---|
| Device API v1 | Never breaks (additive only) | Recorded transcripts from real devices + TB-compatible simulators run on every build |
| REST API | Semver; deprecations ≥ 2 minor versions with `Sunset` | OpenAPI diff gate, generated-client compile tests |
| Bus contracts | Backward-compatible protobuf evolution | `buf breaking` |
| Edge ↔ hub | N-2 protocol window | Matrix tests with old edge binaries |
| DB schema | Expand/contract across 2 releases | Migration tests with data fixtures |
| Dashboards/rule chains | Schema migrators forever | Golden corpus replay |

## 5. Release engineering
Trunk-based development; release trains (monthly minor, LTS every 6–12 months with 18 months support); automated changelog from conventional commits; **feature flags** (global + per-tenant) with expiry; canary → progressive rollout in SaaS; release checklist (security scan, perf gate, upgrade test, docs, migration notes); reproducible builds + SBOM + signatures; hotfix process (< 24 h for critical); deprecation policy published; public docs versioned per release.

## 6. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Test infrastructure | Compose/kind stacks, fixtures, seed data, flaky-test quarantine | 3 |
| Contract & compat suites | Transcripts, TB-compat simulators, OpenAPI/buf gates | 3 |
| Performance rigs | Loadgen scenarios, rigs, budgets, dashboards, continuous profiling | 4 |
| Chaos & soak | Experiments library, scheduling, reports | 3 |
| Release tooling | Changelog, flags, canary, checklists, docs versioning | 2 |
| Fuzz/visual/a11y | Harnesses, corpora, snapshot infra | 1 |

## 7. Acceptance criteria
- PR pipeline < 15 min (p90) with flaky rate < 1 %; nightly full suite green for 14 consecutive days before GA.
- Every service has a perf scenario with recorded baseline; regression gates active on hot paths.
- Chaos suite passes weekly without manual intervention for 4 consecutive weeks.
- Compatibility suites cover every v1 device topic/path and the last two edge versions.
- Release process executed end-to-end for two release candidates, including rollback.

## 8. Risks
| Risk | Mitigation |
|---|---|
| Flaky tests erode trust | Quarantine policy, deterministic clocks/ports, hermetic fixtures |
| Perf rigs drift from production | Pin versions/shapes, periodic calibration against staging |
| Testing debt accumulates | Test requirements in every stage DoD; dashboards for coverage/flake/lag |

## 9. Definition of Done
- [ ] Budgets and scenarios documented, automated, owned
- [ ] Release runbook rehearsed; rollback proven
- [ ] Quality dashboard (coverage, flake rate, perf trend, escape rate) reviewed monthly
