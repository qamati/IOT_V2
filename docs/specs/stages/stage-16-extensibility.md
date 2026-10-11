# Stage 16 — Extensibility & "Your Own Features"

> **Estimate:** 28 eng-weeks (platform SDK work) · **Depends on:** Stages 4, 6, 8, 12 · **Outcome:** you and third parties add features **without forking core**
> **Related:** `services/09-rule-engine.md §8` (WASM), `10-rule-nodes-catalog.md §9`, `frontend/02-widget-runtime-sdk.md §10`, `services/18-integration-converter.md`, `services/26-enterprise.md §10`

## 1. Goal
A deliberate **extension architecture**: stable extension points, a capability-based sandbox, SDKs/CLI, packaging/signing, and a documented playbook for adding first-class features (new entity types, services, UI) to the core itself.

## 2. Extension points
| Extension point | Mechanism | Language | Sandbox / trust |
|---|---|---|---|
| **Rule nodes** | WASM module implementing `Node` ABI (`init`, `on_msg`, `close`) + descriptor | Rust, TinyGo, AssemblyScript, C | wazero; fuel/time/memory limits; host functions by capability (`tell`, `attrs.read`, `http.fetch` allow-list, `kv.get/put`) |
| **Script functions** | Registered `expr` function packs (pure) | Go (built-in) / WASM (pure functions) | no I/O |
| **Widgets** | ESM bundle + manifest (frontend/02) | TS/JS | iframe sandbox + bridge |
| **Integrations/connectors** | gRPC `Connector` service (cloud) / `pkg/connectors` plugin (edge/gateway) | any (gRPC), Go | out-of-process; mTLS; per-plugin quotas |
| **Converters/decoders** | WASM/JS decoder with `decode(bytes, meta) → events` | JS/WASM | no network |
| **Device transports** | gRPC "transport adapter" speaking `pkg/transport.Core` over the bus | any | tenant-scoped service identity |
| **Notification channels** | gRPC `Channel` service | any | secrets by reference |
| **Auth providers** | OIDC/SAML (standard) or gRPC `AuthProvider` | any | strict claims mapping |
| **Storage drivers** | `tsstore.Store` Go plugin (build-time) or gRPC adapter | Go | operator-installed only |
| **Solution templates** | Package of entities + generators | JSON/YAML | install-time validation |
| **Platform API/webhooks** | Stable REST/gRPC + event subscriptions (`entity.events`, `alarm.events`…) via webhooks/NATS | any | scoped API keys/OAuth clients |

## 3. Capability model
Every plugin declares **capabilities** in its manifest; install shows them for approval; runtime enforces them.
```json
{ "id": "acme.rule.pid", "kind": "rule-node", "version": "1.2.0", "sdk": ">=1.0 <2",
  "capabilities": [ "msg.read", "msg.write", "attributes.read:server", "kv.own", "http.fetch:https://api.acme.com" ],
  "limits": { "fuelPerCall": 5000000, "memoryMiB": 16, "timeoutMs": 50, "callsPerSec": 2000 },
  "signature": { "alg": "ed25519", "keyId": "acme-2026", "sig": "…" } }
```
Capabilities are namespaced (`resource.action[:scope]`); no ambient authority; denied calls return errors (and are logged/audited); per-tenant install scope; global kill-switch + revocation list.

## 4. WASM node ABI (v1)
- Exports: `iotp_init(cfg_ptr, cfg_len) -> i32`, `iotp_on_msg(msg_ptr, msg_len) -> i32`, `iotp_close()`.
- Imports (host functions): `tell_next(rel_ptr, rel_len, msg_ptr, msg_len)`, `tell_failure(err_ptr, err_len)`, `attrs_get(...)`, `log(level, ptr, len)`, `kv_get/kv_put` (per-plugin namespace), `http_fetch(req_ptr, req_len) -> resp` (allow-listed, async via callback), `now_ms()`.
- Message encoding: protobuf/JSON in linear memory; zero-copy where possible; **one `tell_*` per message** enforced by host.
- SDKs generate bindings (Rust/TinyGo/AssemblyScript) and a **local test harness** (`iotpctl plugin test` feeding sample messages).

## 5. SDKs and tooling
| Tool | Purpose |
|---|---|
| `iotpctl plugin new <kind>` | Scaffolds node/widget/connector/converter projects with CI |
| `iotpctl plugin dev` | Hot-loads a plugin into a dev tenant (WASM reload, widget HMR, connector gRPC dial-in) |
| `iotpctl plugin test` | Runs golden tests with mock host (capabilities enforced) |
| `iotpctl plugin pack/sign/publish` | Builds `.iotppkg` (manifest + artifact + SBOM + signature) → registry |
| SDK packages | `github.com/yourorg/iotp/pkg/sdk` (Go), `@iotp/widget-sdk` (TS), Rust crate `iotp-node-sdk` |
| Docs | Extension guide, ABI spec, capability reference, example plugins (PID controller node, custom decoder, custom widget, Slack-like channel) |

## 6. Stable platform API
Versioned public **Platform API** (REST + gRPC) with OAuth2 client credentials/API keys, **event subscriptions** (webhooks with HMAC + retry, or NATS/Kafka for in-cluster extensions), rate limits per client, and a **compatibility policy** (semver, 2-release deprecation). Extensions never touch the database directly.

## 7. Playbook: adding your own feature to the core
Use this checklist so new features integrate like built-ins:
1. **Spec** — add `docs/specs/<feature>.md` (problem, API, data model, permissions, events, UI, limits, metrics). Record ADRs.
2. **Data** — migration (expand-only), sqlc queries, RLS policy, indexes, `schema_version` for JSON configs.
3. **Domain/service** — new package under `internal/<svc>`; or new module in an existing service if it shares data/scale; define ports; idempotent handlers.
4. **API** — OpenAPI paths (+ examples, errors), pagination/cursors, `If-Match`, idempotency; regenerate clients.
5. **Authz** — add `Resource` + `Operation`s to the permission catalogue; role templates; scope filters; tests in the authz matrix.
6. **Tenancy & limits** — entity/quotas in tenant profile; rate limits; usage counters; isolation level behaviour.
7. **Events** — `entity.events` emission via outbox; add to audit allow-list; rule-engine message types if user-facing; notification triggers if relevant.
8. **Realtime** — subscriptions/EDQL filters if lists/counters need live updates.
9. **UI** — pages/components using `ui` kit; permission gating; i18n keys; a11y; docs links; dashboard widget if data-centric.
10. **Edge** — decide edge relevance (assignment dependency closure, downlink/uplink events, local runtime support).
11. **Observability** — metrics (low-cardinality), traces, logs, dashboards, alerts, runbook.
12. **Security** — threat-model delta, input validation, secrets handling, rate limits.
13. **Testing** — unit/property, integration, contract, E2E, perf scenario if hot path.
14. **Packaging/ops** — Helm values, config docs, feature flag (`tenant_profile.features`), migration/rollback notes.
15. **Docs** — user guide, API reference, changelog entry.

## 8. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| WASM node runtime | wazero host, ABI, capabilities, limits, sandbox tests, SDK bindings | 8 |
| Packaging & registry | `.iotppkg`, signing/verification, registry service, revocation, capability prompts | 6 |
| Connector/channel/auth SDKs | gRPC contracts, plugin supervisor, quotas, examples | 5 |
| Widget extensions | CLI polish, dev-in-dashboard, marketplace metadata | 3 |
| Platform API & webhooks | Public API hardening, OAuth clients, event subscriptions, docs | 3 |
| Developer experience | Docs site, examples, templates, playground, feature-playbook automation | 3 |

## 9. Acceptance criteria
- A third party builds, signs and installs a **WASM rule node** and a **custom widget** without touching core; capability prompts shown; denied capabilities fail safely.
- Malicious plugin tests (memory bomb, infinite loop, forbidden host call, data exfiltration attempt) are contained: limits trigger, tenant impact < 5 %, audit shows violations, kill-switch revokes within 60 s.
- Plugin crash/timeout never blocks the shard (per-call timeout + circuit breaker) and never loses acknowledged messages.
- Platform API clients survive a minor upgrade without changes (compat tests).
- A new engineer follows the **playbook** to add a small entity type end-to-end (DB → API → authz → UI → edge flag) in ≤ 2 weeks, passing the checklist gates in CI (a lint verifies items like authz resource and audit registration exist).

## 10. Demo (exit)
Ship an `acme.rule.pid` WASM node and an `acme.widget.setpoint` widget through the registry to a tenant; show capability approval, live debug traces, a forced fuel-limit violation, revocation, and then add a "Maintenance Window" entity to the core using the playbook scaffolder.

## 11. Risks
| Risk | Mitigation |
|---|---|
| Sandbox escape or abuse | Defence in depth (WASM + limits + capability audit + process isolation for connectors), security review, fuzzing |
| ABI churn | Versioned ABI with long-term compatibility, conformance tests |
| Ecosystem cold start | Ship first-party plugins using the same SDK (dogfooding), example gallery, partner programme |
| Support burden | Clear support tiers: core vs verified vs community plugins |

## 12. Definition of Done
- [ ] ABI v1 frozen with conformance suite
- [ ] Extension docs + 4 example plugins published
- [ ] Playbook lint in CI; two internal features built with it
- [ ] Revocation/kill-switch drill performed
