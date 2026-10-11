# Architecture Decision Records (ADRs D1–D14)

This record captures decisions D1 through D14 as defined in the platform technical specification (`docs/specs/00-overview/05-tech-stack-and-repo-layout.md`).

| ADR | Title | Decision | Rationale | Status |
|---|---|---|---|---|
| **D1** | Modular Monolith | Modular monolith first; services split by build target (`cmd/*`), not by separate repos | Fast iteration, single `iotp-lite` development process, split only where scaling or isolated failure domains demand | Accepted |
| **D2** | Go 1.24+ | Backend, edge runtime, and gateway services written in Go | High-throughput goroutine concurrency for IoT connection streams; static binaries for edge without VM overhead | Accepted |
| **D3** | PostgreSQL 16 | Relational data system of record | Mature ACID, Row-Level Security (RLS) for multi-tenancy, native JSONB, declarative partitioning | Accepted |
| **D4** | Pluggable Time-Series Store | TimescaleDB default, ClickHouse optional, SQLite at edge/lite behind `tsstore.Store` interface | Operational familiarity first with PostgreSQL hypertable compression; clean scale path | Accepted |
| **D5** | Pluggable Event Bus | NATS JetStream (lite/edge) / Kafka (cluster) / In-memory (tests) behind unified `bus.Bus` | Unified semantics across lightweight local/edge deployments and large distributed clusters | Accepted |
| **D6** | Interface Contracts | Protobuf + `buf` for internal gRPC and bus envelopes; OpenAPI 3.1 for public REST | Strict type safety and contract compilation on both Go backend and TypeScript frontend | Accepted |
| **D7** | Device API Wire Compatibility | Device API v1 is wire-compatible with ThingsBoard MQTT & HTTP conventions | Instant compatibility with existing device fleets, gateways, and hardware SDKs | Accepted |
| **D8** | Sandboxed Scripting | `expr` (default high-speed), `goja` (JS compat), and `wazero` (WASM plugins) | Safe multi-tenant evaluation with CPU/memory/time bounds and zero cgo dependency | Accepted |
| **D9** | Primary Keys | UUIDv7 (time-ordered 128-bit identifiers) | Preserves B-tree index locality while allowing distributed, collision-free ID generation | Accepted |
| **D10** | Multi-Tenancy Architecture | Shared schema + mandatory `tenant_id` + PostgreSQL RLS + session GUC (`app.tenant_id`) | Optimal resource density with defense-in-depth security guarantees against cross-tenant data leaks | Accepted |
| **D11** | Frontend Web Stack | React 19 + TypeScript + Vite in a pnpm workspace | Modern ecosystem, type safety, fast Vite HMR, and component reuse | Accepted |
| **D12** | Widget Extensibility | Widgets as ESM modules with JSON manifests; untrusted widgets isolated in sandboxed iframes | Security isolation for third-party extensions with native ESM load performance | Accepted |
| **D13** | Unified Observability | OpenTelemetry traces + Prometheus metrics + `log/slog` JSON logs with injected `tenant_id` and `trace_id` | End-to-end distributed tracing across transports, event bus, and workers | Accepted |
| **D14** | Database Evolution | Expand/contract migrations using `goose`; zero downtime rolling upgrades | Safe forward-compatible schema evolution across releases | Accepted |
