# Frontend 01 — Web Architecture & Page Map

> **Stage:** 1 (shell) → 6 (dashboards) → 10 (SCADA) · **Apps:** `web/apps/console` (cloud), `web/apps/edge-ui` (edge subset) · **Stack:** React 19 + TypeScript (strict) + Vite + pnpm/Turborepo

## 1. Goals
One design system and one SDK for: tenant console, system admin, customer portal, public dashboards, edge UI and the report print mode. White-label ready, permission-aware, realtime-first, fast on mid-range hardware, accessible (WCAG 2.2 AA).

## 2. Workspace layout
```
web/
├── apps/
│   ├── console/        # main SPA (admin + tenant + customer)
│   ├── edge-ui/        # trimmed build for edge runtime (embedded in Go binary via go:embed)
│   └── docs/           # developer docs / widget catalogue
└── packages/
    ├── ui/             # design system (shadcn/ui + tokens), table, form, dialogs
    ├── api-client/     # generated from OpenAPI (openapi-typescript + openapi-fetch)
    ├── sdk/            # realtime client, subscription manager, auth, permissions
    ├── widgets/        # first-party widgets (ESM) + widget-sdk
    ├── charts/         # uPlot/ECharts wrappers, downsampling worker
    ├── rule-editor/    # React Flow editor (frontend/03)
    ├── scada/          # symbol runtime + HMI editor (frontend/04)
    └── i18n/           # locale bundles + ICU helpers
```
Rules: apps import packages; packages never import apps; `ui` has no data-fetching; every package has Storybook stories and unit tests; lint rule bans cross-package deep imports.

## 3. State and data
| Concern | Tool | Notes |
|---|---|---|
| Server state (REST) | TanStack Query | Query-key factories per entity; stale-while-revalidate; optimistic updates with `version`/ETag; mutation → targeted invalidation |
| Realtime | `RealtimeClient` (packages/sdk) | single WebSocket per tab; shared via `BroadcastChannel`/`SharedWorker` across tabs (optional) |
| UI state | Zustand (+ immer) | per-feature stores; no global mega-store |
| Forms | react-hook-form + zod; RJSF for schema-driven forms | Schema from server for profiles, nodes, widgets |
| URL state | React Router search params | filters/sort/pagination are shareable links |

### RealtimeClient contract
```ts
export interface RealtimeClient {
  connect(): Promise<void>;
  subscribe<T>(cmd: SubscriptionCmd, onUpdate: (u: Update<T>) => void): Unsubscribe; // dedupes identical cmds
  status$: Observable<'connecting'|'open'|'reconnecting'|'closed'>;
}
```
Behaviour: dedupe identical subscriptions across widgets (ref-counted); batch `cmds[]` within a microtask; assign `cmdId`; reconnect with jittered exponential backoff (500 ms → 30 s); **resubscribe** all active subs after reconnect (client keeps last `ts` to request only newer history); `REAUTH` on token refresh; **coalesce** inbound updates per animation frame; heavy transforms (downsampling, aggregation) in a **Web Worker**; backpressure: if frame budget exceeded, drop intermediate points and keep latest.

## 4. Authentication and permissions in the UI
- Access token in memory only; refresh through `httpOnly` cookie endpoint (or rotating refresh in secure storage for the PWA); single-flight refresh on 401; logout broadcast across tabs.
- `usePermissions()` + `<Can op="WRITE" resource="DEVICE" entity={device}>` mirror server rules (server remains authoritative; UI uses `GET /auth/user/permissions` snapshot + scope version).
- Route guards by authority/permission/feature flag (`tenant_profile.features`); impersonation banner; step-up auth modal for sensitive actions.

## 5. Navigation and page map
| Area | Routes | Key components | Needs |
|---|---|---|---|
| Auth | `/login`, `/activate`, `/forgot`, `/2fa`, `/oauth2/callback` | forms, MFA flows | public |
| Home | `/home` | overview cards, alarm summary, usage, quick actions | any |
| Devices | `/devices`, `/devices/:id` | entity table, detail tabs: *Details, Attributes, Latest telemetry, Alarms, Events, Relations, OTA, RPC history, Audit, Credentials, Connectivity* | DEVICE:READ |
| Profiles | `/deviceProfiles`, `/assetProfiles`, `/tenantProfiles` | schema forms (transport, provisioning, alarm rules, data policy) | profile perms |
| Assets/views/customers/users | `/assets`, `/entityViews`, `/customers`, `/users` | tables + hierarchy tree + relation graph | per entity |
| Dashboards | `/dashboards`, `/dashboards/:id` (+ `?state=`), `/widgets` | dashboard grid, widget library, aliases/filters editors | DASHBOARD |
| Rule chains | `/ruleChains`, `/ruleChains/:id` | graph editor, debug, revisions | RULE_CHAIN |
| Alarms | `/alarms`, `/alarmRules` | alarm table (cursor), details panel, comments, assign; rule builder with test | ALARM |
| Notifications | `/notifications/{inbox,rules,templates,targets,requests}` | inbox, template editor w/ preview | NOTIFICATION |
| OTA | `/ota/packages`, `/ota/campaigns` | uploader, wave planner, live progress | OTA |
| Edge | `/edges`, `/edges/:id` | status, backlog, assignments, remote commands | EDGE |
| Integrations | `/integrations`, `/converters` | wizard, converter editor with tester | INTEGRATION |
| Analytics | `/calculatedFields`, `/models` | CF editor + test, KPI library | CF |
| SCADA | `/scada/tags`, `/scada/udts`, `/scada/hmi/:id`, `/scada/alarms`, `/scada/control-log`, `/scada/studio` | tag browser, HMI runtime, alarm summary, symbol/HMI editor | SCADA_* |
| Automation | `/scheduler`, `/reports` | calendar preview, report templates | JOB/REPORT |
| Governance | `/audit`, `/events`, `/usage` | chain-verified audit view, usage charts | AUDIT/USAGE |
| Settings | `/settings/{general,mail,sms,security,oauth2,sso,branding,domains,secrets,queues,resources,vcs,roles,groups,api-keys,solutions,marketplace}` | admin forms | ADMIN |
| System admin | `/admin/{tenants,tenantProfiles,system,regions}` | tenant lifecycle, health | SYS_ADMIN |
| Public | `/p/:publicId` | public dashboard viewer (read-only, minimal bundle) | public principal |
| Print | `/print/dashboards/:id` | print mode for reporter | report token |

## 6. Layout, theming, i18n, a11y
- **Shell:** collapsible side nav (config-driven by role + feature flags + white-label menu), top bar (tenant/customer switch, search palette `⌘K`, notifications bell with unread counter via WS, user menu), breadcrumb from route handles.
- **Tokens:** CSS variables from `branding` API (light/dark/auto, density), Tailwind mapped to tokens; no hard-coded colours.
- **i18n:** i18next + ICU; lazy-loaded locale bundles; tenant overrides merged at runtime; RTL support (logical CSS properties).
- **A11y:** keyboard-first tables and dialogs, focus management, ARIA live regions for alarms/toasts, colour-blind-safe severity palette with icons/shapes, reduced-motion respect; axe checks in CI.

## 7. Tables, lists and performance
Server-side pagination/sort/filter via EDQL or REST `pageLink`; TanStack Table + `react-virtual` for > 200 rows; saved column sets/filters per user; CSV export via async job. **Budgets:** initial JS ≤ 250 KB gz (shell), route chunks ≤ 150 KB, LCP < 2.5 s on 4× CPU throttle, dashboard with 20 widgets < 1.5 s TTI after data, 60 fps interactions; heavy libs (Monaco, MapLibre, ECharts, React Flow) lazy-loaded; prefetch on hover/idle; bundle analyser in CI with regression gate.

## 8. Error handling and observability
Error boundaries per route and per widget; problem+json error mapping to toasts/inline field errors; offline banner; retry with jitter for idempotent queries; Web RUM (Core Web Vitals) + OTel web traces (`traceparent` forwarded to API); client error reporting with source maps (self-hosted); feature flags via `GET /features`.

## 9. PWA and edge UI
`vite-plugin-pwa` (app shell cache, versioned runtime caching, update prompt), Web Push (alarms), deep links, WebAuthn. **edge-ui** = same packages with a route allow-list (devices, dashboards, alarms, SCADA, local settings), `go:embed` into the edge binary, talks to local API, offline-capable.

## 10. Testing strategy
Unit (Vitest + Testing Library), contract tests against generated API client + MSW, visual regression (Storybook + Playwright snapshots), E2E (Playwright flows: login → create device → see telemetry → build dashboard), a11y (axe), perf (Lighthouse CI budgets), WS resilience tests (kill/restore socket, token expiry mid-stream).

## 11. Task checklist
- [ ] Workspace, design system, tokens, Storybook
- [ ] API client generation + auth flows + permission hooks
- [ ] App shell, nav, tables, schema forms
- [ ] Entity pages (devices/assets/customers/users/profiles)
- [ ] `RealtimeClient` + worker pipeline
- [ ] Notifications inbox, alarms pages
- [ ] Dashboards (stage 6), rule editor, SCADA (stage 10) integration points
- [ ] i18n + branding runtime; a11y + perf CI gates
- [ ] PWA + edge-ui build
