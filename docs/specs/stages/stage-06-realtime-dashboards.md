# Stage 6 — Realtime & Dashboards

> **Estimate:** 56 eng-weeks (largest; frontend-heavy; starts early with a thin slice during stages 3–4) · **Depends on:** Stage 3 (data), 5 (alarm widgets)
> **Specs:** `services/16-realtime.md`, `17-dashboard-resources.md`, `frontend/01-architecture-and-pages.md`, `frontend/02-widget-runtime-sdk.md`

## 1. Goal
Users build and view live dashboards with maps, charts, tables and controls — fast, secure and shareable (customers, public links, embeds).

## 2. Scope
**Thin slice (month 3–5, MVP gate):** WebSocket service v0 (ENTITY_DATA latest + ts), dashboard CRUD v0, 5 widgets (value card, line chart, gauge, entities table, switch), aliases (single/device type), timewindow v0.
**In (P0/P1):** full WebSocket protocol (ENTITY_DATA, ALARM_DATA, counts, notifications), dynamic queries, coalescing/limits; dashboard model with **states, aliases, filters, timewindow, actions**; widget runtime (ESM, sandbox), SDK + CLI; ~**40 widgets by gate, 60 by end**; maps (markers, polygons, routes, image map); resources/images; customer assignment, **public dashboards**, embed tokens, export/import, revisions, edit locks; home dashboards; print mode (for reporter, stage 12); i18n and branding tokens.
**Out:** SCADA HMI studio (stage 10), reports (stage 12), marketplace (stage 16).

## 3. Deliverables
1. `cmd/realtime` + `packages/sdk` RealtimeClient + subscription manager + worker pipeline.
2. `cmd/dashboard` + schema migrators + resource store + sanitisers.
3. Dashboard editor/viewer (grid, states, alias/filter editors, widget library UI, action editor).
4. `packages/widgets` (first-party), `@iotp/widget-sdk`, `iotp-widget` CLI, sandbox host.
5. Maps package (MapLibre) with offline tile support.
6. Public/embedded dashboard runtime with minimal bundle.
7. Perf + visual regression suites.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Realtime service | Protocol, auth/reauth, registry, NATS interests, coalescing, limits, scope re-eval | 8 |
| SDK/client pipeline | Dedup, history+realtime merge, worker (LTTB/units), rAF batching | 6 |
| Dashboard backend | CRUD, revisions, migrators, sharing, public/embed, import/export | 6 |
| Resources/images | Upload, sanitisers, hash serving, references | 3 |
| Dashboard UI | Grid editor, states/aliases/filters/timewindow, actions, home/fullscreen | 10 |
| Widget runtime | Contract, sandbox bridge, signing, permissions | 5 |
| Widget library | ~60 widgets across groups | 12 |
| Maps | Layers, clustering, geofences editor, routes, image map | 4 |
| Perf/quality | Visual + perf + sandbox tests, a11y | 2 |

## 5. Acceptance criteria
- **Realtime:** 50 k concurrent WebSockets / node; p99 commit→browser < 1 s at 10 k updates/s/node; permission revoke stops updates ≤ 5 s.
- **Dashboard load:** 20 widgets, TTI < 1.5 s after data on a 4× CPU-throttled laptop; 60 fps interactions; memory stable over 8 h (no leaks) with 1 Hz updates.
- **Sandbox:** malicious widget cannot access cookies/tokens/network/parent DOM (escape suite).
- **Security:** public dashboard exposes only allow-listed entities/keys; rate-limited; revocable; SVG/JS-module sanitisers pass XSS corpora.
- **Compat:** dashboards survive schema migrations (golden corpus v1→vN); export/import round-trips with resources.
- **UX:** user builds a 5-widget dashboard bound to a device profile alias in < 10 minutes.

## 6. Demo (exit)
Create dashboard with a map of devices (clustered), per-device drill-down state, alarm table with ack, a control switch (RPC stub until stage 7), time-window switching between realtime/history, share a public read-only link, embed via iframe, export/import into another tenant.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Widget scope creep | Library tiers (must/should/could), ship value early, community widgets via SDK |
| Realtime fan-out at scale | Per-entity interest subscriptions, coalescing, load tests from week 4 |
| Sandbox complexity | Start with first-party same-realm; add iframe bridge before opening custom widgets |
| Chart performance | uPlot, workers, downsampling budgets enforced in CI |

## 8. Definition of Done
- [ ] Load + soak + leak tests pass; budgets enforced in CI
- [ ] Widget SDK docs + 3 sample external widgets
- [ ] Accessibility audit (axe + manual) for core widgets
- [ ] Public/embed security review
