# iotp — IoT Platform Blueprint (ThingsBoard-class, Go + React)

> **What this is:** a complete build specification for your own IoT platform that matches the functional surface of ThingsBoard (Community, Professional, Edge, Gateway, SCADA-style dashboards, calculated fields, alarm rules, notifications, OTA, integrations) and then goes further (real SCADA, lightweight Go edge, extensibility SDK).
> **Baseline analysed:** ThingsBoard 4.3 line (Alarm Rules 2.0, Calculated Fields incl. Geofencing/Propagation/Aggregation, AI rule node from 4.2, EDQS from 4.0, SCADA from 3.8). Re-verify against upstream docs before you freeze scope.
> **Stack:** Go (backend, edge, gateway) · React + TypeScript (web) · PostgreSQL/Timescale · Kafka (or NATS) · Redis/Valkey · S3-compatible object store.
> **Code-name:** `iotp` is a placeholder. Rename the Go module (`github.com/yourorg/iotp`), binaries and topics freely.

---

## 1. How to read this pack

| If you want to… | Read |
|---|---|
| Understand exactly what ThingsBoard does and where it is weak | `00-overview/01-thingsboard-deep-analysis.md` |
| See the target architecture and data flows | `00-overview/02-architecture.md` |
| Start coding the data layer | `00-overview/03-data-model.md` |
| Implement device protocols / REST / WebSocket | `00-overview/04-protocols-and-api.md` |
| Set up the monorepo and tooling | `00-overview/05-tech-stack-and-repo-layout.md` |
| Plan delivery (what to build in which order) | `stages/` (17 stage files) |
| Implement one component in depth | `services/` (26 service specs) |
| Build the web app | `frontend/` (4 specs) |

Recommended order for a new team: **00-vision → 01-analysis → 02-architecture → 03-data-model → stage-00 → stage-01 …**, reading each stage together with the service docs it lists.

---

## 2. File map

### 00-overview
| File | Content |
|---|---|
| `00-vision-and-scope.md` | Goals, personas, non-goals, differentiation, success metrics |
| `01-thingsboard-deep-analysis.md` | Full feature inventory, architecture teardown, weaknesses, parity matrix |
| `02-architecture.md` | Logical/physical architecture, partitioning, flows, deployment topologies |
| `03-data-model.md` | Entities, ER diagram, PostgreSQL DDL, JSON schemas |
| `04-protocols-and-api.md` | MQTT/HTTP/CoAP device APIs, REST conventions, WebSocket, gRPC |
| `05-tech-stack-and-repo-layout.md` | Decision records, libraries, monorepo layout, conventions |

### stages (delivery roadmap)
| Stage | File | Theme | Est. (eng-weeks) |
|---|---|---|---|
| 0 | `stage-00-foundations.md` | Repo, CI, shared libs, local env | 8 |
| 1 | `stage-01-identity-tenancy-entities.md` | Auth, tenants, entities, relations | 24 |
| 2 | `stage-02-device-connectivity.md` | Registry, MQTT/HTTP, bus, provisioning | 24 |
| 3 | `stage-03-data-pipeline.md` | Telemetry, attributes, device state | 28 |
| 4 | `stage-04-rule-engine.md` | Rule engine + core nodes | 36 |
| 5 | `stage-05-alarms-notifications.md` | Alarms, alarm rules, notifications | 24 |
| 6 | `stage-06-realtime-dashboards.md` | WebSocket, dashboards, widgets, maps | 56 |
| 7 | `stage-07-device-management.md` | RPC, OTA, gateways, bulk ops | 24 |
| 8 | `stage-08-integrations-protocols.md` | Integrations, converters, CoAP/LwM2M/SNMP, field gateway | 40 |
| 9 | `stage-09-edge.md` | Edge hub + edge runtime | 48 |
| 10 | `stage-10-scada.md` | Tags, HMI, ISA-18.2 alarms, control | 64 |
| 11 | `stage-11-analytics.md` | Calculated fields, stream analytics, AI | 28 |
| 12 | `stage-12-enterprise.md` | RBAC groups, white-label, reports, VCS, usage | 40 |
| 13 | `stage-13-scale-ha-deploy.md` | HA, Kubernetes, observability | 28 |
| 14 | `stage-14-security-compliance.md` | Threat model, IEC 62443, SOC2/GDPR | 24 |
| 15 | `stage-15-quality-performance.md` | Test pyramid, load, chaos, release | 16 |
| 16 | `stage-16-extensibility.md` | Plugin SDK, your own features | 28 |
| | | **Total** | **≈ 540** |

### services (one spec per component)
`01-apigw` · `02-identity` · `03-core` · `04-device-registry` · `05-transport-mqtt` · `06-transports-http-coap-lwm2m-snmp` · `07-device-state` · `08-message-bus` · `09-rule-engine` · `10-rule-nodes-catalog` · `11-telemetry-attributes` · `12-alarm` · `13-notification` · `14-rpc` · `15-ota` · `16-realtime` · `17-dashboard-resources` · `18-integration-converter` · `19-field-gateway` · `20-scheduler-reporter` · `21-audit-events-usage` · `22-edge-hub` · `23-edge-runtime` · `24-scada` · `25-analytics` · `26-enterprise`

### frontend
`01-architecture-and-pages` · `02-widget-runtime-sdk` · `03-rule-chain-editor` · `04-scada-hmi-editor`

---

## 3. Conventions used in every document

- **MUST / SHOULD / MAY** follow RFC 2119.
- **TB reference** blocks describe what ThingsBoard does so you can reproduce behaviour; **iotp design** blocks describe what we do (often better).
- **Priority tags:** `P0` MVP-critical · `P1` parity · `P2` differentiator / later.
- All timestamps are **Unix epoch milliseconds (int64, UTC)** in APIs and storage; ISO-8601 only at UI edges.
- All IDs are **UUIDv7** (time-ordered), serialised as lowercase strings.
- Every service doc ends with a **task breakdown checklist** you can paste into your issue tracker.

---

## 4. Roadmap in one paragraph

An **MVP** (stages 0–4 plus a thin slice of 6: login, device list, latest-value cards, one chart) is roughly 150–200 engineer-weeks, i.e. 5–6 months for a team of 8. **Full parity** with ThingsBoard PE-class features plus Edge, SCADA and extensibility is ≈ 540 engineer-weeks, i.e. 14–18 months for 10–12 engineers once parallel tracks are considered (frontend, protocols, edge/SCADA can run alongside the core). Stages 13–15 are continuous cross-cutting tracks that start at stage 0 and are formally closed late.

---

## 5. Licensing & legal note (not legal advice)

ThingsBoard CE is Apache-2.0; PE is proprietary. This blueprint describes **behaviour and public interfaces**, not source code. Build clean-room: do not copy PE code, UI assets or documentation text; do not use the ThingsBoard name or logos. If you choose device-protocol wire-compatibility (recommended, see `04-protocols-and-api.md`), have counsel confirm how you may describe it ("compatible with…"). Check licences of every third-party Go/JS dependency in CI (see stage 0).

---

## 6. Using this pack with issue trackers and AI coding agents

1. Create one epic per stage; one story per checklist line in each service doc.
2. Feed an agent **one service doc + `03-data-model.md` + `04-protocols-and-api.md`** per task; they are written to be self-contained specs.
3. Keep decisions in `docs/adr/` (template in `05-tech-stack-and-repo-layout.md`) so the specs stay stable while code evolves.
