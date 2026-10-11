# IOT_V2 — Next-Generation IoT Platform

An enterprise-grade, multi-tenant IoT platform (ThingsBoard-class: devices, telemetry, rule engine, alarms, dashboards, RPC, OTA, integrations, edge, SCADA) built with a high-performance **Go** backend/edge/gateway and a **React + TypeScript** web console.

## Architecture & Specifications

The complete specification pack lives in [`docs/specs/`](docs/specs/README.md):
- [00 — Vision & Scope](docs/specs/00-overview/00-vision-and-scope.md)
- [01 — ThingsBoard Deep Analysis](docs/specs/00-overview/01-thingsboard-deep-analysis.md)
- [02 — Architecture Blueprint](docs/specs/00-overview/02-architecture.md)
- [03 — Domain Data Model & DDL](docs/specs/00-overview/03-data-model.md)
- [04 — Protocols & API Reference](docs/specs/00-overview/04-protocols-and-api.md)
- [05 — Tech Stack & Repository Layout](docs/specs/00-overview/05-tech-stack-and-repo-layout.md)
- [Delivery Stages (0–16)](docs/specs/stages/)
- [Service Specifications (01–26)](docs/specs/services/)
- [Frontend Specifications](docs/specs/frontend/)

## Quick Start (Developer Setup)

```bash
# Start local development stack (TimescaleDB, Redis, NATS, MinIO, Mailpit, iotp-lite)
task dev

# Run all test suites
task test

# Run code linter
task lint
```

## Agent Guidelines

- Instructions: [CLAUDE.md](CLAUDE.md) / [AGENTS.md](AGENTS.md)
- Prompt Library: [docs/prompts/01-prompt-library.md](docs/prompts/01-prompt-library.md)
- Architecture Decision Records: [docs/adr/](docs/adr/)
