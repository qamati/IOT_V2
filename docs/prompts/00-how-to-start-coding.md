# How to Start Coding iotp (AI-assisted, spec-driven)

> You have the spec pack (architecture, data model, 17 stages, 26 service specs, 4 frontend specs). This guide turns it into a working loop: **set up → bootstrap → walking skeleton → stages**. Companion files: `CLAUDE.md` (agent instructions) and `01-prompt-library.md` (copy-paste prompts).

---

## 1. Set up your machine (macOS / Linux / WSL2)

| Need | Install |
|---|---|
| Go 1.24+ | go.dev/dl |
| Docker + Compose | Docker Desktop or engine + compose plugin |
| Node 22 LTS + pnpm | `corepack enable && corepack prepare pnpm@latest --activate` |
| Git (+ optional `gh`) | your package manager |
| MQTT test client, JSON, load tool | `mosquitto-clients`, `jq`, `k6`, `psql` (postgresql-client) |

Go tooling (one-time):
```bash
go install github.com/go-task/task/v3/cmd/task@latest              # task runner
go install github.com/bufbuild/buf/cmd/buf@latest                  # protobuf lint/codegen
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest                # typed SQL
go install github.com/pressly/goose/v3/cmd/goose@latest            # migrations
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go install github.com/air-verse/air@latest                         # hot reload (optional)
go install golang.org/x/vuln/cmd/govulncheck@latest
# golangci-lint: use the official install script from golangci-lint.run
```
Pick one AI coding agent (a terminal agent such as Claude Code, or an IDE agent such as Cursor/Copilot). The prompts are tool-agnostic.

---

## 2. Prepare the repository

Keep the specs **inside the code repo** so the agent can read them: `docs/specs/`.

```bash
git clone https://github.com/qamati/IOT_V2.git && cd IOT_V2
mkdir -p docs/specs docs/prompts docs/adr

# If you already uploaded the pack at the repo root:
git mv 00-overview stages services frontend docs/specs/
git mv README.md docs/specs/README.md
# If you did NOT upload it yet: unzip the pack and copy its folders into docs/specs/ instead.

cp /path/to/prompts/CLAUDE.md ./CLAUDE.md            # agent instructions (repo root)
ln -s CLAUDE.md AGENTS.md                             # other agents read AGENTS.md
cp /path/to/prompts/01-prompt-library.md docs/prompts/
printf '# IOT_V2\n\nIoT platform (Go + React). Specs: docs/specs/README.md\n' > README.md
git add -A && git commit -m "chore: add specs, agent instructions and prompt library"
git push origin main
```
Instruction-file names differ per tool (`CLAUDE.md`, `AGENTS.md`, `.github/copilot-instructions.md`, `.cursor/rules`): keep one canonical copy and link/copy it; check your tool's docs for the exact location.

---

## 3. The working loop (use it for every task)

1. **New session per task.** Long sessions drift. Start fresh and paste the prompt (library §1–§3).
2. **Make the agent plan first:** its understanding (≤ 10 lines), assumptions/questions, numbered steps (each ≤ ~300 lines of diff). **You approve or correct the plan.**
3. **Implement one step at a time**, tests first for anything stateful (state machines, parsers, ordering, authz, tenancy).
4. **Verify:** `task lint test` after every step; run the acceptance command from the stage doc yourself.
5. **Review adversarially:** open a second session and run a review prompt (spec compliance → security → concurrency/perf).
6. **Commit small** (conventional commits). If code diverges from the spec, update the spec in the same commit or write an ADR.
7. **Hand off:** at the end of a session ask for a handoff summary (library U4) and paste it into the next session.

**Task sizing:** one service workstream (the tables in each stage doc) per session. If a step needs more than ~½ day of review, split it.

---

## 4. What to build first: the walking skeleton (≈ 2 weeks)

Don't build stage by stage in isolation; first prove the thinnest end-to-end path with a single binary (`cmd/iotp-lite`, in-memory bus, Postgres):

| Days | Deliverable | Spec |
|---|---|---|
| 1–2 | **Stage 0** bootstrap: monorepo, Taskfile, compose, shared libs, CI | `stages/stage-00` |
| 3–4 | Migrations + `pkg/db` + tenant/user/device/credentials tables with RLS | `00-overview/03-data-model.md` |
| 5–7 | `identity` (login, JWT) + `core` (devices CRUD, default profile) + REST via OpenAPI | `services/02`, `03`, `01` |
| 8–9 | `pkg/bus` (mem) + `tr-mqtt` (token auth, telemetry, ack-after-bus) | `services/05`, `08`, `04` |
| 10–11 | Telemetry store (Postgres `ts_kv` + latest) + direct-save worker + REST query | `services/11` |
| 12–14 | React: login → device list → device page with latest values + line chart | `frontend/01` |

**Done when** this script passes (put it in `tools/smoke.sh`):
```bash
# 1 login → TOKEN   2 create device → DEVICE_ID + DEVICE_TOKEN
mosquitto_pub -h localhost -p 1883 -u "$DEVICE_TOKEN" -t v1/devices/me/telemetry -q 1 -m '{"temperature":22.5}'
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/telemetry/DEVICE/$DEVICE_ID/values/timeseries?keys=temperature" | jq .
# 3 a second tenant's token gets 404/403 for the same device
```

---

## 5. After the skeleton: follow the stages

1. Harden **Stages 1–3** properly (refresh tokens, 2FA later, devreg caches, Kafka/NATS drivers, Timescale, devstate, retention).
2. **Stage 4 rule engine** (the MVP gate), then **Stage 5 alarms/notifications**.
3. **Stage 6 realtime + dashboards** (start the frontend thin slice earlier, in parallel with 3–4).
4. Then 7 (RPC/OTA/gateway) → 8 (integrations/protocols) → 9 (edge) → 10 (SCADA) → 11–12 → 13–15 (continuous) → 16 (extensibility).
Parallel tracks that rarely block each other: **frontend**, **protocols/connectors**, **edge/SCADA** (after 3–5), **DevOps/observability**.
The estimates in the stage docs assume human-only effort; AI shortens coding but not review/testing, so measure your own velocity after the skeleton and re-plan.

---

## 6. Human review checklist (never delegate blindly)

| Area | What to check |
|---|---|
| Tenancy | `tenant_id` in every query, RLS enabled, cross-tenant test exists and fails when you remove a check |
| AuthN/Z | argon2id, JWT validation (alg, exp, audience), refresh rotation, permission checks on every route |
| Device auth | constant-time compare, negative cache, rate limits, no token logging, ACL only `v1/devices/me/*` |
| SQL | parameterised only (sqlc), migrations expand-only, indexes match query plans |
| Concurrency | every goroutine has an owner + stop; bounded channels; `-race` clean; no sleeps hiding races |
| Messaging | idempotent consumers, ack-after-durable, per-key ordering preserved |
| Scripts/plugins | sandbox limits, SSRF guard, secrets by reference |
| Dependencies | only those in doc 05 §2 or an ADR; `govulncheck` clean |

---

## 7. Common AI pitfalls and fixes

| Symptom | Likely cause | Fix |
|---|---|---|
| Invents library APIs (e.g. MQTT hooks) | Version drift | Prompt U1: read the pinned library source/docs, write a tiny spike test, then code |
| Compiles but leaks across tenants | Missing scope checks | Write the cross-tenant test first; enforce `WithTenant` in the repo layer |
| Over-engineering | Open-ended prompt | State scope in/out; "no new abstractions unless the spec calls for them" |
| Giant diffs | Vague task | Step plan with ≤ 300-line steps; commit per step |
| Flaky tests with `sleep` | Time not modelled | Fake clocks, channels/WaitGroups, `rapid`, `-race` |
| Spec drift | Quick fixes | Spec-change rule in `CLAUDE.md`; update docs in the same commit |
| Context overflow / forgets rules | Long session | New session + handoff summary; rules live in `CLAUDE.md` |
| Green tests, wrong behaviour | Weak assertions | Property tests; run the stage acceptance script yourself |

---

## 8. Habits that keep the project healthy

- CI green from day one (lint, `-race` tests, govulncheck, gitleaks); never merge red.
- One ADR per significant decision (`docs/adr/`), referenced from PRs.
- Keep prompts in the repo (`docs/prompts/`) and improve them when an agent misbehaves — they are part of your engineering system.
- Seed data and a device simulator (`tools/device-sim`) early: demos and load tests depend on them.
- Re-read the stage's **Risks** and **Definition of Done** before closing it.
