# iotp Prompt Library (copy-paste)

> Use with `CLAUDE.md` in the repo root (it carries the permanent rules). Start a **new agent session per prompt**. Replace `<angle brackets>`. Paths assume specs in `docs/specs/`.

## 0. Prompt anatomy (write your own the same way)
**Context** (files to read) → **Goal** (one sentence) → **Scope in/out** → **Constraints** → **Acceptance** (testable, with commands) → **Process** (plan first) → **Output** (what to show).

---

## 1. Kickoff prompts

### P0 — Bootstrap the repo (Stage 0)
```text
Follow CLAUDE.md. Implement STAGE 0 of iotp.

READ FIRST (fully): docs/specs/stages/stage-00-foundations.md; docs/specs/00-overview/05-tech-stack-and-repo-layout.md; docs/specs/00-overview/02-architecture.md (sections 1 and 10).

GOAL: a repo where "git clone && task dev" starts the dev stack and CI is green.

BUILD:
1. Monorepo skeleton exactly as doc 05 section 4 (create only what Stage 0 needs now; add .gitkeep for the rest). Go module path: github.com/qamati/IOT_V2. Go 1.24+.
2. Taskfile.yml tasks: dev, down, test, lint, gen, migrate:up, migrate:down, seed (stub), build, e2e (stub).
3. Shared libs WITH unit tests: pkg/config (koanf: env + file, typed structs); pkg/obs (slog JSON with trace_id/tenant_id, OpenTelemetry tracer, Prometheus registry + /metrics handler); pkg/errs (typed errors + HTTP/gRPC status mapping table); pkg/ids (UUIDv7); pkg/db (pgxpool, tx helper, WithTenant that sets app.tenant_id per transaction); pkg/svc (service runner: graceful shutdown, errgroup, /healthz /readyz /metrics); pkg/testkit (testcontainers Postgres helper).
4. Skeleton binaries under cmd/: apigw, identity, core, devreg, telemetry, ruleengine, tr-mqtt, tr-http, iotp-lite, iotpctl. Each uses pkg/svc and exposes health + metrics only.
5. deploy/compose: TimescaleDB (pg16), Redis/Valkey, NATS with JetStream, MinIO, Mailpit, all with healthchecks.
6. GitHub Actions: golangci-lint, go test -race, govulncheck, gitleaks, build matrix for pkg/... on linux/amd64, arm64, arm (no cgo), web lint/typecheck/test.
7. docs/adr/0001..0014: one short ADR per decision in doc 05 section 1.

DO NOT: add business logic, extra frameworks, cgo, or dependencies not listed in doc 05 section 2.

ACCEPTANCE (prove each with a command and its output): task dev works from a clean clone in under 5 minutes; every skeleton binary serves /healthz /readyz /metrics; a pkg/db test proves RLS blocks cross-tenant reads; CI fails on a deliberate lint error; pkg/... builds for amd64, arm64 and arm.

PROCESS: first reply with (a) your understanding in <=10 lines, (b) assumptions/questions, (c) a numbered step plan with one commit per step. Wait for my OK. Then implement step by step, running "task lint test" after each step. Finish with a checklist mapping every acceptance item to evidence.
```

### P1 — Walking skeleton (end-to-end in one binary)
```text
Follow CLAUDE.md. Build the WALKING SKELETON: one process (cmd/iotp-lite), in-memory bus, PostgreSQL, thin web UI.

READ FIRST: docs/specs/stages/stage-01-identity-tenancy-entities.md, stage-02-device-connectivity.md and stage-03-data-pipeline.md (P0 items only); docs/specs/00-overview/03-data-model.md (tables: tenant_profile, tenant, users, user_credentials, device_profile, device, device_credentials, key_dictionary, ts_kv, ts_kv_latest); docs/specs/00-overview/04-protocols-and-api.md (sections 1, 4, 5 limited to telemetry); docs/specs/services/05-transport-mqtt.md (5.1, 5.2, 5.4); docs/specs/services/11-telemetry-attributes.md (write path, raw query); docs/specs/services/02-identity.md (tokens); docs/specs/services/04-device-registry.md (hot-path auth).

GOAL: login -> create device -> device publishes MQTT telemetry -> stored -> REST query -> UI chart.

SCOPE IN:
- goose migrations for the tables above with RLS policies and tenant GUC use.
- identity: login with argon2id, JWT (EdDSA) access token only.
- core: devices CRUD + default device profile; device credentials (ACCESS_TOKEN) with lookup_id = HMAC-SHA256.
- devreg: token authentication with in-process LRU + negative cache.
- pkg/bus: interface + in-memory driver (ordering per key, ack semantics).
- tr-mqtt on mochi-mqtt: token auth via username, ACL limited to v1/devices/me/*, publish v1/devices/me/telemetry (JSON, QoS 0/1); PUBACK only after bus ack.
- direct-save worker: bus -> telemetry store; Postgres ts_kv + ts_kv_latest via the unnest upsert; REST raw timeseries + latest.
- OpenAPI for: POST /api/v1/auth/login, GET/POST /api/v1/devices, GET /api/v1/devices/{id}/credentials, GET /api/v1/telemetry/DEVICE/{id}/values/timeseries, latest values. Generated server (oapi-codegen) and typed TS client.
- web: login page, device list, device page with latest values and a uPlot line chart (poll every 2 s).
SCOPE OUT: refresh tokens, 2FA, RBAC, rule engine, WebSocket, alarms, attributes, other protocols.

ACCEPTANCE: tools/smoke.sh passes against a clean "task dev": (1) login returns a token; (2) create device returns id + device token; (3) mosquitto_pub -u <device token> -t v1/devices/me/telemetry -q 1 -m '{"temperature":22.5}' succeeds; (4) the REST query returns that point; (5) a second tenant's token receives 404/403 for the same device and telemetry; (6) an invalid device token is rejected and rate-limited; (7) unit tests cover bus ordering, credential lookup, tenancy isolation.

PROCESS: plan first (understanding, assumptions, steps <=300-line diffs, one commit each), wait for OK, then implement test-first where stateful. Finish with evidence for every acceptance item and a list of spec deviations.
```

---

## 2. Generic stage-workstream prompt (fill in from the table in §3)
```text
Follow CLAUDE.md. Implement STAGE <N> workstream "<WORKSTREAM>" of iotp.

READ FIRST (fully): docs/specs/stages/<stage file>; <service specs>; docs/specs/00-overview/03-data-model.md (tables: <tables>); docs/specs/00-overview/04-protocols-and-api.md (sections: <sections>).

GOAL: <one sentence>.
SCOPE IN: <P0 items for this workstream from the stage doc>.
SCOPE OUT: everything else in the stage (do not start it).
CONSTRAINTS: CLAUDE.md rules plus: <service-specific constraints>.
ACCEPTANCE: <acceptance criteria from the stage doc that this workstream owns, each with the test/command that proves it>.

PROCESS: reply first with understanding (<=10 lines), assumptions/questions, and a numbered step plan (steps <=300 lines of diff). Wait for my OK. Implement test-first for state machines, parsers, authz and tenancy. Run "task lint test" after every step. Finish with an acceptance evidence checklist and any spec deviations (propose spec edits).
```

## 3. Fill-in table for Stages 1–4
| Stage / workstream | Read (specs) | Scope in (one line) | Key acceptance |
|---|---|---|---|
| **1 / Identity** | `services/02-identity`, `01-apigw` | argon2id, JWT+JWKS, rotating refresh, lockout, activation/reset, `Authorizer` v1 | refresh reuse revokes family; lockout after 5 fails; cross-tenant denied |
| **1 / Core entities** | `services/03-core`, `03-data-model` | CRUD tenants/customers/devices/assets/profiles, versioning, outbox, quotas | optimistic-lock 409s; outbox crash test; quota enforced |
| **1 / Relations + EDQL** | `services/03-core §6–7` | relations CRUD + traversal, EDQL compiler/executor, counts | cycle/depth guards; golden SQL; p95 < 200 ms on 100 k entities |
| **1 / API gateway** | `services/01-apigw` | middleware chain, limiter grammar `N:T,…`, error mapping, OpenAPI validate | 429 + headers; schemathesis passes |
| **1 / Frontend shell** | `frontend/01` | auth flows, nav by role, tables, schema forms, typed client | login → CRUD devices e2e (Playwright) |
| **2 / Message bus** | `services/08-message-bus` | interfaces, mem + NATS + Kafka drivers, conformance suite, DLQ | same suite green on all drivers; rebalance hooks fire |
| **2 / devreg** | `services/04-device-registry` | credentials, caches, provisioning (3 strategies), claiming | cache hit > 99 %; provisioning contract tests |
| **2 / tr-mqtt** | `services/05-transport-mqtt`, `04-protocols §1` | hooks, session registry, rate limits, TLS/mTLS, gateway later | ack-after-durable fault test; 50 k idle conns |
| **2 / tr-http** | `services/06 §2`, `04-protocols §2` | endpoints, long-poll, limits | contract transcripts pass |
| **3 / tsstore + Timescale** | `services/11 §4–6`, `03-data-model §4` | Store interface, conformance suite, hypertable, compression, retention classes | conformance green; 100 k rows/s |
| **3 / Write + Query** | `services/11 §5, §7` | streaming write, dedupe, strategies, raw/aggregate/calendar buckets, latest | idempotent replay; DST golden tests |
| **3 / Attributes** | `services/11 §8` | 3 scopes, cache, notifications, shared push downlink | shared update reaches device < 500 ms |
| **3 / devstate** | `services/07-device-state` | timer wheel, shards, snapshots, events | 1 M devices ≤ 150 MB; rebuild < 60 s |
| **4 / Runtime core** | `services/09 §3–6` | Message/Node SPI, shard executor, tracker, hop guard | ordering property test passes |
| **4 / Compiler + revisions** | `services/09 §5, §9` | validate, draft/publish, hot swap | no loss under publish at 10 k msg/s |
| **4 / Queues** | `services/09 §7`, `08` | pack consumer, 5 submit + 6 processing strategies, DLQ, replay CLI | each strategy tested with injected failures |
| **4 / Scripting** | `services/09 §8` | `expr` library, `goja` sandbox, test endpoint | infinite loop contained; tenant isolation |
| **4 / P0 nodes** | `services/10 §2–7` | ~20 nodes + descriptors + golden tests | golden suite green |

---

## 4. "Hard part" prompts (use before the risky components)

### H1 — MQTT hook semantics (tr-mqtt)
```text
Follow CLAUDE.md. Before implementing tr-mqtt, de-risk the embedded broker.
Pin the exact mochi-mqtt version. Read its source/docs for hook ordering. Write a spike test that proves: (a) when PUBACK is written relative to OnPublish returning; (b) how to stop fan-out of a device PUBLISH to other subscribers; (c) what happens when OnPublish blocks for 500 ms; (d) behaviour of OnACLCheck for subscribe vs publish; (e) MQTT v5 reason codes available for rate limiting.
Report findings in docs/adr/ as an ADR with the recommended design (block-in-hook vs patched acks) and the test as a permanent conformance test. Do not write production code yet.
```

### H2 — Rule-engine ordering (property tests first)
```text
Follow CLAUDE.md. Implement docs/specs/services/09-rule-engine.md sections 3-6 TEST-FIRST.
Step 1: write a rapid property test that injects random async delays, branch fan-outs, retries and partition rebalances, and asserts per-(queue, originator) order is never violated and every message is acked exactly once logically.
Step 2: implement the shard executor and tracker until the property test passes with -race.
Step 3: add benchmarks (3-node chain) and record numbers. Show the failing test output first, then the passing run.
```

### H3 — Telemetry store conformance suite
```text
Follow CLAUDE.md. Implement the tsstore.Store interface and a CONFORMANCE SUITE (pkg/tsstore/conformance) before any driver, covering: idempotent upsert on (entity,key,ts); in-batch duplicate collapse; latest only advances when ts is newer; raw query order/limit; aggregates MIN/MAX/AVG/SUM/COUNT vs a reference in-memory implementation; calendar buckets (WEEK, MONTH, QUARTER) across DST in several timezones; deletion with latest rewrite. Then implement the Timescale driver and make the suite pass. Report any behaviour the spec leaves undefined.
```

### H4 — Edge queue power-loss harness
```text
Follow CLAUDE.md. Implement the store-and-forward queue from docs/specs/services/23-edge-runtime.md section 5 with a power-loss test harness: a child process writes events with up_seq while a parent randomly SIGKILLs it 1,000 times; after each restart verify no corruption, no lost L0/L1 events, and exactly-once replay after dedupe by up_seq. Implement lanes, group commit, capacity policies (never drop L0; downsample then drop L2) and show the harness results.
```

### H5 — ISA-18.2 alarm state machine
```text
Follow CLAUDE.md. Implement the SCADA alarm state machine from docs/specs/services/24-scada.md section 7 as a pure Go package with a table-driven test covering EVERY transition (including shelve, suppress, out-of-service, on/off delays, chatter suppression). No I/O in the package. Then add the KPI calculations (alarms per operator per hour, peak 10-minute count, stale alarms, top chattering) with hand-computed fixtures.
```

### H6 — Edge sync convergence property test
```text
Follow CLAUDE.md. Implement the edge-hub/edge sync protocol from docs/specs/services/22-edge-hub.md section 3 with an in-memory fake network. Write a property test over random disconnect/duplicate/reorder/delay schedules asserting: applied downlink state converges to the cloud state; up_seq has no gaps or duplicates after dedupe; cursors never move backwards. Then implement against real gRPC and keep the property test as a regression gate.
```

---

## 5. Review prompts (run in a separate session after implementing)

### R1 — Spec compliance
```text
You are a strict reviewer. Compare the code changed in <commit range or paths> against docs/specs/<service or stage file>. For every requirement, acceptance criterion and checklist item, report: Met / Partially / Missing / Deviates, with file:line evidence. List undocumented behaviours and spec statements that look wrong. Do not modify code.
```

### R2 — Security review
```text
You are an application security engineer. Review <paths> for: cross-tenant access (every query, cache key, bus topic/header, WebSocket subscription); authN/authZ gaps (routes without declared permissions); token/secret handling and logging; SQL injection; SSRF; unsafe deserialization; resource exhaustion (unbounded input, goroutines, queues); race conditions; timing attacks; error leakage. For each finding give severity, exploit scenario, file:line, and a minimal fix with a regression test. Then write the missing tests. Do not refactor unrelated code.
```

### R3 — Concurrency and performance
```text
Review <paths> for goroutine leaks, missing stop signals, unbounded channels/maps, lock contention, hot-path allocations, blocking calls on shard goroutines, and missing backpressure. Run go test -race and write benchmarks for the hot paths named in docs/specs/<file>. Report ns/op, allocs/op and p99 estimates versus the spec targets, and propose targeted fixes with before/after numbers.
```

### R4 — Migration safety
```text
Review the new migrations in db/migrations for: expand/contract compliance (no destructive change in the same release), lock duration on large tables, index creation CONCURRENTLY, RLS policies present, backfills batched, reversibility, and compatibility with the previous application version running during rollout. Produce a rollout plan and flag any step needing downtime.
```

### R5 — Test-gap analysis
```text
List the behaviours in docs/specs/<file> that have no test, ranked by risk (data loss, security, ordering, money). Write the top 10 missing tests (table-driven or property-based) and make sure each fails when the behaviour is broken (show a mutation that breaks it).
```

---

## 6. Frontend prompts

### F1 — Console shell (Stage 1)
```text
Follow CLAUDE.md. Build the web console shell per docs/specs/frontend/01-architecture-and-pages.md sections 1-6 and 11 (stage 1 scope only).
Use pnpm workspaces: apps/console, packages/ui (shadcn/ui + CSS-variable tokens), packages/api-client (generated from api/openapi/openapi.yaml with openapi-typescript + openapi-fetch), packages/sdk (auth + permissions hooks only).
Deliver: login/logout with in-memory access token + single-flight refresh, route guards, role-aware navigation, server-paginated entity tables (TanStack Table), schema-driven forms (react-hook-form + zod; RJSF for profile editors), i18n scaffold, light/dark tokens, error boundaries, problem+json error mapping.
ACCEPTANCE: Playwright e2e (login -> create customer -> create device profile -> create device -> see it in the list), axe has no serious violations, initial JS <= 250 KB gzip, Storybook stories for ui components.
Plan first, then implement.
```

### F2 — RealtimeClient and thin dashboard slice (Stage 6 thin slice)
```text
Follow CLAUDE.md. Implement packages/sdk RealtimeClient per docs/specs/frontend/01 section 3 and the subscription pipeline in docs/specs/frontend/02 sections 3-4: ref-counted dedupe of identical subscriptions, batched cmds[], jittered reconnect with resubscribe, REAUTH, per-frame coalescing, Web Worker for LTTB downsampling, columnar DataFrame.
Then build five widgets (value card, line chart on uPlot, gauge, entities table, switch) and a minimal dashboard grid (react-grid-layout) bound to an entity alias.
ACCEPTANCE: unit tests for merge/trim/late-data logic, a Playwright test that kills the socket and verifies resubscription without data loss, 20 widgets at 1 update/s stay under 8 ms per frame in a perf test.
Plan first.
```

---

## 7. Utility prompts

### U1 — Learn a library safely
```text
I need to use <library@version>. Read its source and docs at that exact version. Summarise the API surface relevant to <use case> in <= 15 lines, list gotchas and version-specific behaviour, then write a tiny spike test that proves the three riskiest assumptions. Only after the spike passes propose the integration design.
```

### U2 — Write an ADR
```text
Write an ADR (docs/adr/NNNN-<title>.md: Context, Decision, Alternatives considered, Consequences, Status) for <decision>. Link the affected spec sections and list the exact spec edits required. Keep it under one page.
```

### U3 — Update the spec after a deviation
```text
We deviated from docs/specs/<file> section <n> because <reason>. Propose the minimal edit to the spec (show a diff), update any dependent docs (list them), and add the entry to docs/adr/. Do not change code.
```

### U4 — Session handoff summary
```text
Summarise this session for the next one: goal, what is done (with commit hashes), what is half-done, failing tests, decisions made (and ADRs), open questions, exact next 3 steps, and the commands to reproduce the current state. Keep it under 40 lines and write it to docs/handoff/<date>-<topic>.md.
```

### U5 — Debug a failing test properly
```text
Test <name> fails: <paste output>. Do NOT change the test or add sleeps. Form three hypotheses ranked by likelihood, add targeted logging or a minimal reproducer to confirm the root cause, then fix the cause and show the test passing with -race and -count=20.
```
