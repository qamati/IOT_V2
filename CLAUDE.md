# iotp — Agent Instructions (system prompt for AI coding agents)

> Save as `CLAUDE.md` in the repo root (symlink `AGENTS.md`; paste into Cursor/Copilot rule files). These rules apply to **every** task. Specs live in `docs/specs/`.

## Mission
Implement **iotp**, a multi-tenant IoT platform (ThingsBoard-class: devices, telemetry, rule engine, alarms, dashboards, RPC, OTA, integrations, edge, SCADA) with a **Go** backend/edge/gateway and a **React + TypeScript** frontend, exactly as specified in `docs/specs/`. **The spec is the source of truth.**

## Where to look
- Index: `docs/specs/README.md`. Data model: `00-overview/03-data-model.md`. Protocols/APIs: `00-overview/04-protocols-and-api.md`. Stack and layout: `00-overview/05-tech-stack-and-repo-layout.md`.
- Per component: `docs/specs/services/NN-*.md`; per delivery stage: `docs/specs/stages/stage-NN-*.md`; UI: `docs/specs/frontend/*.md`.
- Before any task, **read the referenced spec files completely**. If the spec is ambiguous or contradicts itself, **stop and ask**; never invent APIs, topics or schemas.

## How to work
1. **Plan first.** Reply with: understanding (≤ 10 lines) → assumptions/open questions → numbered steps (each ≤ ~300 lines of diff). Wait for approval on anything non-trivial.
2. **One step at a time.** After each step: format (`gofmt`/`goimports`, prettier), `task lint`, `task test`. Do not continue on red.
3. **Tests first** for state machines, parsers/decoders, authorisation, tenancy isolation, ordering/idempotency, expression/script engines.
4. **Small, focused commits** with conventional prefixes (`feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `chore:`). No drive-by refactors.
5. **Fix causes, not symptoms.** Never weaken a test, add `sleep` to hide a race, or silence a linter without a written reason.
6. **Spec changes:** if you must deviate, state why, get approval, then update the spec (or add an ADR in `docs/adr/`) **in the same change**.
7. **Dependencies:** only those listed in doc 05 §2, or approved via ADR. Prefer the standard library. Run `govulncheck`. Pin versions.
8. **Unfamiliar library?** Read its source/docs at the pinned version and write a tiny spike test **before** designing around it (e.g., MQTT hook ordering, Kafka rebalance callbacks).

## Architecture rules (non-negotiable)
- **Modular monolith:** `cmd/<name>/main.go` is a composition root only; logic in `internal/<svc>/{domain,app,infra,api}`; shared code in `pkg/` (no cgo; must build for `linux/amd64`, `arm64`, `arm`). `internal/*` never imports another service's `internal`. Domain packages don't import DB/transport packages.
- **Tenancy:** `tenant_id` in every tenant-owned table and query, Postgres RLS as a safety net, tenant GUC set per transaction (`pkg/db.WithTenant`), tenant in bus headers/topics. Cross-tenant tests are mandatory for every repository and endpoint group.
- **IDs & time:** UUIDv7; timestamps are Unix epoch **milliseconds** (int64, UTC); JSONB configs carry `schema_version`; entities carry `version` for optimistic concurrency.
- **Messaging:** at-least-once; consumers **idempotent**; ordering only per partition key (originator id); device ingress acks **after** the bus confirms durability; poison messages go to DLQ.
- **Device API v1 is ThingsBoard wire-compatible** (topics/paths/payloads in doc 04). Never break it; additive changes only.
- **APIs:** OpenAPI-first (`oapi-codegen`), RFC 9457 problem+json errors, cursor/page pagination as specced, `Idempotency-Key` and `If-Match` where specced.
- **Errors:** typed domain errors in `pkg/errs`, mapped to HTTP/gRPC once at the edge; wrap with `%w`; no panics across package boundaries; `context.Context` first parameter.
- **Concurrency:** every goroutine has an owner and a stop signal (`errgroup`/context); channels are bounded; no unbounded fan-out; run tests with `-race`.
- **Observability:** `log/slog` JSON with `trace_id`/`tenant_id`; metrics `iotp_<service>_<noun>_<unit>` with **low-cardinality labels** (never device/tenant ids); OpenTelemetry traces; every service exposes `/healthz`, `/readyz`, `/metrics`.

## Security rules
- Passwords: argon2id. Secrets/tokens: constant-time comparison, never logged, never in URLs (except device-token paths that the spec defines), stored hashed/encrypted per specs.
- SQL: parameterised only (sqlc); no string-built SQL. Validate and bound **all** external input (size, depth, count). JSON/proto decoders are fuzzed.
- Outbound calls to user-configurable URLs go through the SSRF guard. User scripts run only in the sandboxed engines (`expr`, `goja` with interrupts, WASM) with CPU/memory/time limits.
- Deny by default for authorisation; every route/RPC declares its permission (resource + operation).
- Never commit secrets; `.env` files are git-ignored; use `.env.example`.

## Tech stack (fixed unless an ADR changes it)
Go ≥ 1.24 · chi · pgx/v5 + sqlc + goose · PostgreSQL 16 (+ TimescaleDB) · Redis/Valkey (go-redis v9) · franz-go (Kafka) / nats.go · mochi-mqtt · go-coap · slog · koanf · testify + testcontainers-go + rapid · React 19 + TypeScript (strict) + Vite + pnpm · TanStack Query · Zustand · shadcn/ui · uPlot/ECharts · MapLibre · React Flow · Monaco · Vitest + Playwright.

## Definition of done (every task)
- Code + tests + docs updated; `task lint test` green; `govulncheck` clean; migrations are expand-only and reversible.
- Tenancy/authz tests included for new endpoints or queries.
- Metrics/logs/traces added where the spec asks; dashboards/alerts if a new SLO-relevant path.
- The relevant spec checklist item is ticked and any deviation recorded.
- Acceptance evidence provided: the exact command(s) run and their output.

## Communication style
- Be concise. Show the plan, then the changes. List files touched and commands run.
- When blocked or unsure, ask **one precise question** with options and a recommendation.
- Do not claim something works unless you ran it; report failures honestly with the error.
- Do not paste large unchanged files; show diffs or new files.
