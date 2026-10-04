# Stage 0 — Foundations

> **Estimate:** 8 eng-weeks · **Depends on:** nothing · **Unlocks:** every other stage
> **Specs:** `00-overview/05-tech-stack-and-repo-layout.md`, `00-overview/02-architecture.md`

## 1. Goal
A repository where a new engineer can clone, run `task dev`, see a green CI, and ship a vertical slice behind feature flags on day one. Establish the **shared libraries** that every service reuses so later stages never reinvent them.

## 2. Scope
**In:** monorepo, tooling, CI/CD skeleton, shared `pkg/*` libs, local environment, ADR process, coding standards, seed/demo data skeleton, docs site.
**Out:** any business feature.

## 3. Deliverables
| # | Deliverable | Notes |
|---|---|---|
| 1 | Monorepo (Go module + pnpm workspace) matching the layout in doc 05 | Empty service skeletons with health endpoints |
| 2 | `Taskfile.yml` targets: `dev`, `test`, `lint`, `gen`, `migrate`, `seed`, `build`, `e2e` | |
| 3 | `pkg/config` (koanf), `pkg/obs` (slog + OTel + Prometheus), `pkg/errs`, `pkg/ids` (UUIDv7), `pkg/db` (pgx pool, tx helper, tenant GUC), `pkg/testkit` (testcontainers helpers) | Each with tests |
| 4 | `pkg/bus` interface + in-memory implementation (Kafka/NATS come in stage 2) | |
| 5 | CI pipeline (lint, unit, integration, security scans, build matrix) | See doc 05 §6 |
| 6 | Dev stack in `deploy/compose` (Postgres+Timescale, Redis, NATS, MinIO, Mailpit, Grafana stack) | |
| 7 | Codegen pipeline: `buf`, `sqlc`, `oapi-codegen`, `openapi-typescript` | CI fails on drift |
| 8 | ADR folder with D1–D14 recorded | |
| 9 | Docs site skeleton (this spec pack rendered) | |

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Repo & tooling | Module layout, Taskfile, pre-commit, editorconfig, dependabot/renovate | 2 |
| CI/CD | Pipelines, caching, container builds (ko), SBOM + cosign wiring | 2 |
| Shared libs | config, obs, errs, ids, db, testkit, ratelimit stub, tenancy ctx | 2 |
| Dev env & seed | Compose stack, seed CLI skeleton, hot-reload | 1 |
| Process | ADR template, PR template, CODEOWNERS, definition of done | 1 |

## 5. Acceptance criteria
- `git clone && task dev` yields a running stack in < 5 minutes on a clean laptop.
- CI on an empty PR completes in < 10 minutes; fails on lint error, proto breaking change, sqlc drift, known-vulnerable dependency.
- Every service skeleton exposes `/healthz`, `/readyz`, `/metrics`, and emits a trace span for a sample request visible in Tempo/Jaeger.
- `pkg/db` test proves `app.tenant_id` GUC is set per transaction and RLS blocks cross-tenant reads.
- `go build` succeeds for `linux/amd64`, `linux/arm64`, `linux/arm` for packages under `pkg/` (edge portability gate).

## 6. Demo (exit)
Start the stack, hit each service health endpoint, show a trace across two skeleton services in Grafana, show CI blocking a deliberately broken proto.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Over-engineering the skeleton | Time-box; only create what stage 1 needs immediately |
| Toolchain drift between devs | Pin versions via `tools.go`/`mise`; devcontainer |
| Slow CI | Layer caching, test sharding, path-filtered jobs |

## 8. Definition of Done
- [ ] Repo + CI green on `main`
- [ ] All shared libs have ≥ 80 % coverage
- [ ] Runbook "how to add a service" written
- [ ] ADRs D1–D14 merged
