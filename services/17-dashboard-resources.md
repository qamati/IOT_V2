# Service 17 — Dashboards, Widgets & Resources (`dashboard`)

> **Stage:** 6 · **Binary:** `cmd/dashboard` · **State:** Postgres + object store · **Priority:** P0 (CRUD, images), P1 (public, export/import, widget bundles), P2 (marketplace)

## 1. Purpose
Server side of the visualisation layer: dashboard definitions, widget types/bundles, the **image & resource library** (images, fonts, JS modules, SCADA symbols, LwM2M models), sharing (customers, public links, embeds), import/export, and versioned schemas.

## 2. ThingsBoard reference
Dashboard JSON `configuration` (widgets map, **states** with layouts, **entity aliases**, **filters**, timewindow, settings); widget types (timeseries, latest, RPC/control, alarm, static) shipped in **bundles** (system/tenant); image library; JS modules (3.9); SCADA symbols as resources (3.8); export/import JSON; assign to customers; **public dashboards**; iframe embedding; mobile-app dashboards; PE: scheduled reports (Reporting 2.0), solution templates, version control.

## 3. Dashboard schema (ours)
```json
{
  "schemaVersion": 3,
  "settings": { "theme": "auto", "gridBase": 24, "margin": 8, "showTitle": true, "stateControllerId": "default" },
  "timewindow": { "realtime": { "interval": 1000, "timewindowMs": 3600000 }, "aggregation": { "type": "AVG", "limit": 500 } },
  "entityAliases": { "a1": { "alias": "Boilers", "filter": { "type": "deviceType", "deviceTypes": ["boiler"] }, "resolveMultiple": true } },
  "filters": { "f1": { "name": "Hot", "keyFilters": [ ] } },
  "widgets": { "w1": { "typeFqn": "iotp.charts.timeseries_line", "config": {
      "datasources": [ { "aliasId": "a1", "dataKeys": [ { "type": "TIME_SERIES", "name": "temperature", "label": "Temp", "unit": "degC", "color": "#e11d48" } ] } ],
      "settings": { }, "actions": { "onClick": [ { "type": "openState", "state": "details" } ] }, "timewindow": null } } },
  "states": { "default": { "root": true, "layouts": { "main": { "widgets": { "w1": { "x": 0, "y": 0, "w": 12, "h": 6 } }, "grid": { } } } },
              "details": { "layouts": { "main": { "widgets": { } } } } }
}
```
**Schema evolution:** every dashboard has `schemaVersion`; server runs **migrators** on read/write (`v1→v2→v3`), keeps the original in `dashboard_revision`; UI never edits unknown future versions.

## 4. Widget model
```json
{ "fqn": "iotp.charts.timeseries_line", "name": "Timeseries line chart", "type": "timeseries",
  "version": "1.4.0", "entry": "resource://widgets/timeseries_line@1.4.0/index.js",
  "manifest": { "defaultSize": { "w": 8, "h": 5 }, "dataKeys": { "timeseries": {"min":1,"max":20}, "attribute": {"min":0,"max":0} },
                "settingsSchema": { }, "settingsUi": { }, "dataKeySettingsSchema": { }, "hasBasicMode": true,
                "permissions": ["data.read"], "trust": "first-party" },
  "bundle": "charts", "scope": "SYSTEM" }
```
- **Widgets are ESM modules** (not HTML/JS strings): signed, versioned, loaded from the resource store; **trust levels:** `first-party` (same origin, reviewed), `tenant` (sandboxed iframe by default), `marketplace` (sandboxed + capability prompts). See `frontend/02-widget-runtime-sdk.md`.
- **Bundles:** named collections with ordering/tags; system bundles read-only; tenants can clone/customise; import/export as `.iotpwidget` zip (manifest + bundle + assets).

## 5. Resources and images
- Types: `IMAGE` (png/jpg/webp/svg), `FONT`, `JS_MODULE`, `WIDGET_BUNDLE_ASSET`, `SCADA_SYMBOL`, `LWM2M_MODEL`, `PKCS12`, `DASHBOARD_THUMBNAIL`.
- **Upload:** presigned PUT to object store; server verifies MIME by magic bytes, size caps (image 10 MB, module 2 MB, symbol 1 MB), computes SHA-256, stores `resource(id, tenant_id, type, name, sha256, size, storage_key, etag, public boolean, system boolean)`.
- **SVG sanitisation:** strict allow-list (no `<script>`, `foreignObject` handlers, external `href`/`xlink:href`, `javascript:` URLs, CSS `@import`/`url()` to external) using a hardened sanitiser; same for SCADA symbols (tags/behaviours metadata preserved in a dedicated namespace).
- **Serving:** immutable URLs by hash (`/res/{sha256}`), long cache headers, CDN-friendly; private resources require auth or signed URLs; `Content-Security-Policy: sandbox` for SVG when opened directly.
- **References:** dashboards/widgets reference resources by id; export bundles them; deletion blocked when referenced (or force with orphan report).

## 6. Sharing, public links, embedding
| Feature | Design |
|---|---|
| Assign to customers | `dashboard_customer(dashboard_id, customer_id)`; customer users see only assigned dashboards |
| Public dashboard | Random `public_id`; resolves to **public principal** with read-only allow-list of entities/keys; separate rate limits; can be revoked/rotated; indexable = no |
| Embed (iframe) | `frame-ancestors` allow-list per tenant/domain; signed embed tokens (JWT, TTL, scope = dashboard + state + filters) for per-user context without login |
| Home dashboard | per user/customer/tenant (precedence), fullscreen/hide toolbar options |
| Mobile | `mobile_dashboard` flags + simplified layout hints |

## 7. Import / export
Format `.iotpdash` (zip): `dashboard.json` + `resources/*` + `widgets/*` (non-system) + `manifest.json` (schema, dependencies, checksums). Import wizard: dry-run report (missing aliases/entities mapped by name/profile, missing widgets, resources to create), conflict policy (skip/overwrite/rename). Optional **ThingsBoard dashboard importer (stretch):** map TB widget fqns to iotp equivalents, alias filters, timewindow; unsupported widgets become placeholders with warnings.

## 8. Versioning and history
`dashboard_revision(dashboard_id, rev, author, comment, config, created)` auto-saved on explicit save (not every drag); restore/diff in UI; optional Git sync (doc 26); soft concurrency control (`version` + last-writer warning; **edit lock** with TTL to avoid collisions).

## 9. API
`CRUD /dashboards`, `/dashboards/{id}/revisions`, `POST /dashboards/{id}/assign|unassign`, `POST /dashboards/{id}/public`, `POST /dashboards/{id}/embed-token`, `GET /dashboards/{id}/export`, `POST /dashboards/import`, `CRUD /widgetsBundles`, `/widgetTypes`, `POST /widgetTypes/import`, `CRUD /resources`, `POST /resources/upload-url`, `GET /res/{sha256}`, `GET /images/{id}/thumbnail`.

## 10. Observability & security
Metrics: dashboard loads/saves, resource uploads/size, sanitiser rejections, import failures. Security: sanitise all uploads; scan JS modules for forbidden APIs (static analysis: `eval`, `new Function`, `document.cookie`, network calls outside SDK) when trust < first-party; CSP strict; widget signature verification; audit publish/share actions; rate-limit public endpoints.

## 11. Testing
Schema migration tests (golden dashboards per version); SVG sanitiser fuzz/corpus (XSS payload sets); import/export round-trip; authz matrix (customer/public); screenshot tests of thumbnails.

## 12. Task checklist
- [ ] Schema + CRUD + revisions + migrators
- [ ] Widget type/bundle model + system bundle seeding
- [ ] Resource store, presigned upload, hash URLs, sanitisers
- [ ] Customer assignment + public principal + embed tokens
- [ ] Export/import + dry-run + conflict policies
- [ ] Edit locks; thumbnails (reporter)
- [ ] (P2) marketplace metadata, signature verification, TB importer
