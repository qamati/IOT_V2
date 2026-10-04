# 00 — Vision, Scope & Success Metrics

## 1. Vision

Build an **open, multi-tenant IoT platform** that a small team can run on a laptop (single binary, < 1 GB RAM), a Raspberry-class edge box (< 150 MB RSS agent), or a Kubernetes cluster ingesting hundreds of thousands of messages per second — with the *same codebase, same data model, same APIs*.

It must cover the full ThingsBoard functional surface (device management, telemetry, rule engine, alarms, dashboards, RPC, OTA, integrations, edge, notifications, enterprise features) and then differentiate with: **real SCADA semantics**, **first-class edge autonomy**, **safe extensibility (WASM/plugins)**, and **operational simplicity**.

## 2. Positioning

| Alternative | Strength | Gap we exploit |
|---|---|---|
| ThingsBoard CE/PE | Broad feature set, mature | JVM footprint, heavy edge, visualization-only SCADA, limited extensibility without rebuilds, DIY time-series rollups |
| Node-RED + Grafana + TSDB | Flexible, cheap | No multi-tenancy, no device lifecycle, no alarms/RBAC, glue-code sprawl |
| AWS IoT / Azure IoT | Elastic, managed | Lock-in, per-message cost, weak on-prem/edge autonomy, dashboards are separate products |
| Ignition / WinCC | Real SCADA | Not cloud-native/multi-tenant, licence-heavy, weak IoT device lifecycle |
| EMQX / HiveMQ + custom | Broker scale | Not a platform (no registry, rules-to-storage, UI) |

## 3. Personas

| Persona | Needs | Primary surfaces |
|---|---|---|
| **Platform operator** (sysadmin) | Tenants, quotas, health, upgrades, billing hooks | Admin UI, Grafana, CLI |
| **Tenant admin** | Devices, profiles, rule chains, dashboards, users | Web UI, REST |
| **Solution engineer / integrator** | Onboard fleets fast, converters, integrations, templates | UI wizards, Terraform/CLI, Git sync |
| **End customer / operator** | Dashboards, alarms, controls, reports | Dashboards, mobile PWA |
| **Firmware engineer** | Simple protocols, SDKs, OTA, provisioning | MQTT/HTTP/CoAP, SDKs |
| **OT / SCADA engineer** | Tags, drivers, HMI, ISA-18.2 alarms, safe control | SCADA studio, field gateway |
| **Data analyst / ML engineer** | Query, export, derived signals | SQL/REST export, calculated fields |
| **Extension developer** | Custom nodes/widgets/integrations | Plugin SDK, WASM, widget CLI |

## 4. Goals

| ID | Goal | Measure |
|---|---|---|
| G1 | Functional parity with ThingsBoard 4.3 CE + selected PE features | Parity matrix (analysis doc §18) ≥ 95 % of P0+P1 |
| G2 | Device-protocol compatibility with ThingsBoard (MQTT/HTTP/gateway) | Stock TB-compatible devices and gateways connect unchanged |
| G3 | Ingestion performance | ≥ 100 k msg/s sustained on a 3-node cluster; p99 device→dashboard < 1 s |
| G4 | Footprint | Lite mode ≤ 1 GB RAM for 10 k devices; edge agent ≤ 150 MB RSS for 10 k tags |
| G5 | Reliability | 99.9 % control-plane, 99.95 % ingestion; zero acknowledged-message loss (at-least-once) |
| G6 | Time-to-first-data | New tenant to first chart < 10 minutes |
| G7 | Extensibility | Add a rule node, widget and integration without forking core |
| G8 | Security | Pass independent pentest; IEC 62443-4-1 aligned SDLC; SOC 2-ready controls |

## 5. Non-goals (v1)

- General-purpose MQTT broker competing with EMQX at 10 M connections (we embed a broker for transport; offer bridging to external brokers).
- Native iOS/Android apps (ship a PWA first; native later).
- MES/ERP functionality (batch execution beyond recipe download, scheduling, inventory).
- Proprietary hardware.
- Built-in ML training (we serve/orchestrate models; training stays external).

## 6. Differentiators to build ("your own features" hooks)

1. **Lite mode:** all services as goroutines in one process, in-memory bus, SQLite/Postgres.
2. **Go edge runtime:** single static binary, offline autonomy, store-and-forward with priorities, A/B self-update.
3. **True SCADA subsystem:** tag model with quality codes, UDT templates, drivers, ISA-18.2 alarm lifecycle, select-before-operate control, historian compression.
4. **Rule chains as code:** versioned drafts, diff, Git sync in core, durable timers, replayable message traces.
5. **OTA campaigns:** waves, canary, health gates, auto-rollback, signed artifacts.
6. **Plugin platform:** WASM nodes, widget SDK, integration SDK, marketplace metadata.
7. **Digital-twin graph:** typed schemas on entities, graph queries beyond flat relations.
8. **AI-assisted authoring:** natural-language → rule chain / calculated field / dashboard (human-approved).
9. **Open metering:** usage events exportable to any billing system.

## 7. Scope boundaries by deployment

| Mode | Included | Excluded |
|---|---|---|
| **Lite** | Core, MQTT/HTTP, rule engine, ts (SQLite/Postgres), UI | HA, Kafka, multi-region |
| **Cluster** | All cloud services, Kafka, Redis, Timescale/ClickHouse | — |
| **Edge** | Connectors, local broker, rules, store-forward, mini-UI, sync agent | Multi-tenant admin, heavy analytics |
| **SaaS** | Cluster + billing, regions, white-label | — |

## 8. Top risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | Scope explosion (ThingsBoard has a decade of features) | Strict priority tags, MVP gate at stage 4 |
| R2 | Rule-engine semantics drift (ordering, retries) | Property-based tests, formal message-ordering contract |
| R3 | Time-series scale surprises | Pluggable store, early load tests (stage 3) |
| R4 | Frontend widget ecosystem takes years | Ship ~60 first-party widgets, import path for TB dashboards (stretch) |
| R5 | Protocol long tail (LwM2M, BACnet, DNP3) | Defer P2 protocols, reuse sidecars initially |
| R6 | Security of user scripts/plugins | Sandbox (expr/WASM), quotas, no host access by default |
| R7 | Edge/cloud sync conflicts | Single source of truth per data class, versioned deltas |
| R8 | Licensing/legal confusion with ThingsBoard | Clean-room process, legal review |

## 9. Glossary

| Term | Meaning |
|---|---|
| Tenant | Isolated organisation; owns all entities |
| Customer | Sub-organisation of a tenant (end customer) |
| Device / Asset | Thing that emits data / logical object (building, pump station) |
| Profile | Shared configuration for devices/assets/tenants |
| Telemetry | Time-series key-values |
| Attribute | Key-value with scope: client, server, shared |
| Rule chain | Directed graph of nodes processing messages |
| Transport | Protocol-facing service (MQTT, HTTP, CoAP…) |
| Edge | Autonomous node near devices syncing with cloud |
| Tag | SCADA data point with quality, units, scaling, alarms |
| UDT | User-defined type (tag template) |
| SBO | Select-before-operate control pattern |
| EDQL | Entity data query language/API |
