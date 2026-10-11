# Stage 13 — Scale, HA, Deployment & Observability

> **Estimate:** 28 eng-weeks (continuous track; formal close after stage 12) · **Depends on:** all earlier stages (starts at stage 0 with dashboards/alerts)
> **Specs:** `00-overview/02-architecture.md §8–12`, `services/08-message-bus.md §7`, `services/21-audit-events-usage.md`

## 1. Goal
Run the platform reliably at target scale with predictable operations: HA topologies, autoscaling, upgrades without downtime, backups/DR, and end-to-end observability with SLOs.

## 2. Scope
**In:** Helm charts + Kustomize overlays (lite/cluster/SaaS), operators/HA for Postgres (CloudNativePG/Patroni), Kafka (Strimzi/Redpanda), Redis (Sentinel/Cluster), ClickHouse (optional), object store; autoscaling (HPA + **KEDA on Kafka lag/WebSocket count/connections**), pod disruption budgets, topology spread, graceful drain for transports/realtime/edge-hub; **OpenTelemetry** everywhere (traces with trace-ID in bus envelopes, metrics, logs), Prometheus/Grafana/Loki/Tempo stack with golden dashboards, SLO definitions and burn-rate alerts, runbooks; backup/restore + DR (PITR, cross-region replicas, MirrorMaker), zero-downtime upgrade pipeline (expand/contract migrations, canary/blue-green), capacity/cost model, Terraform modules (AWS/GCP/Azure + bare-metal), air-gapped install bundle.
**P2:** multi-region active/active for stateless tiers, cell-based architecture for SaaS.

## 3. Deliverables
1. `deploy/helm` (umbrella + per-service), `deploy/terraform`, `deploy/compose`, offline bundle builder.
2. Observability pack: dashboards (ingest, rule engine, TS store, WS, edge fleet, SCADA), alert rules, SLOs-as-code (Sloth/OpenSLO).
3. Runbook library (≥ 25): lag growth, rebalance storm, DB failover, Redis loss, cert expiry, DLQ replay, partition increase, tenant isolation (L2/L3), restore drills.
4. Upgrade playbook + automated rolling-upgrade test in CI (old→new version under load).
5. Capacity model spreadsheet (inputs: devices, msg rate, retention → nodes/cost) validated by load tests.
6. DR drill reports (RPO/RTO measured).

## 4. SLOs (initial)
| SLI | Objective |
|---|---|
| Device publish → durable ack (p99) | < 100 ms |
| Device → dashboard (p99) | < 1 s |
| REST read p95 / p99 | < 200 ms / < 800 ms |
| Ingestion availability | 99.95 % monthly |
| Control-plane availability | 99.9 % monthly |
| Alarm creation → notification delivered (p95) | < 10 s |
| RPO / RTO (regional failure) | ≤ 5 min / ≤ 60 min (single-region HA: RPO 0 / RTO ≤ 5 min) |

## 5. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Packaging/IaC | Helm, Terraform, compose, offline bundle | 5 |
| Data-tier HA | PG/Kafka/Redis/ClickHouse operators, backups, PITR | 6 |
| Autoscaling & resilience | HPA/KEDA, PDBs, drains, load-shedding, circuit breakers audit | 4 |
| Observability | OTel instrumentation audit, dashboards, SLOs, alert tuning | 5 |
| Upgrade & release ops | Expand/contract tooling, canary, rolling test in CI | 3 |
| DR & chaos | Backups/restore drills, region failover, chaos experiments | 3 |
| Capacity/cost | Load model, benchmarking, cost dashboard | 2 |

## 6. Acceptance criteria
- **HA:** killing any single pod/node/AZ never breaks ingestion SLO; Postgres failover < 30 s with zero acknowledged loss; Kafka broker loss tolerated; Redis loss degrades gracefully (documented).
- **Autoscaling:** 5× step-load on ingestion scales consumers/transports within 3 min; lag recovers < 5 min; scale-down safe (no message loss, WS reconnect jitter).
- **Upgrade:** rolling upgrade under 50 k msg/s with zero failed requests above error budget; rollback verified; DB migrations expand/contract only.
- **DR:** restore from backup into a fresh cluster and replay → RPO/RTO met in drill; runbooks executed by someone who didn't write them.
- **Observability:** any request/message traceable end-to-end (device publish → rule nodes → store → WS) by trace-ID; alerts have owners and runbook links; no alert fires without a runbook.
- **Air-gapped:** offline bundle installs on isolated cluster in < 2 h following docs.

## 7. Demo (exit)
Load generator at 100 k msg/s; kill a Postgres primary, a Kafka broker and an AZ in sequence; show dashboards/SLO burn, autoscaling reaction, trace of a single message, rolling upgrade mid-load, then restore a backup into a clean environment.

## 8. Risks
| Risk | Mitigation |
|---|---|
| Operator complexity | Opinionated defaults, managed-service profiles (RDS/MSK/ElastiCache) documented |
| Rebalance storms | Cooperative-sticky assignor, static membership, rolling-update surge limits |
| Cost surprises | Cost model + per-tenant usage attribution, retention tuning guidance |
| Alert fatigue | SLO-based paging only; ticket-level for the rest |

## 9. Definition of Done
- [ ] SLO dashboards live; error budgets tracked
- [ ] DR and upgrade drills passed twice in a row
- [ ] Runbooks reviewed by on-call engineers
