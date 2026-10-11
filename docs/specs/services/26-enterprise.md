# Service 26 — Enterprise Features (`enterprise` modules)

> **Stage:** 12 · **Lives in:** modules inside `identity`, `core`, `dashboard`, plus workers `vcs-sync`, `whitelabel`, `solutions` · **Priority:** P1 (groups/roles, white-label, VCS, secrets) · P2 (SSO, marketplace, multi-region)

## 1. Purpose
The features that turn the platform into a sellable, governable product: fine-grained access, branding, GitOps/promotion, packaged solutions, SSO, secrets, regions and a plugin marketplace.

## 2. ThingsBoard reference (PE)
Entity groups + roles (group-level permissions, customer hierarchy); white-labeling (logo, colours, favicon, login page, domain, mail templates, translations, custom menu); integrations/converters (doc 18); scheduler/reports (doc 20); **version control** (Git sync, auto-commit, restore, supported entity types growing through 4.2); solution templates; SAML/LDAP/OAuth2 SSO; secrets management (4.2); mobile center/app builder; **IoT Hub** marketplace for widgets/solutions.

## 3. Entity groups, roles and fine-grained RBAC
**Model**
```sql
CREATE TABLE entity_group (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, owner_type text NOT NULL, owner_id uuid NOT NULL,
  type text NOT NULL, name text NOT NULL, additional_info jsonb, is_all boolean NOT NULL DEFAULT false, UNIQUE (owner_id, type, name));
CREATE TABLE entity_group_member (group_id uuid NOT NULL, entity_id uuid NOT NULL, PRIMARY KEY (group_id, entity_id));
CREATE INDEX egm_entity ON entity_group_member (entity_id);
CREATE TABLE role (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, owner_id uuid, name text NOT NULL, type text NOT NULL, -- GENERIC | GROUP
  permissions jsonb NOT NULL, version int NOT NULL DEFAULT 1);
CREATE TABLE role_binding (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, principal_type text NOT NULL, principal_id uuid NOT NULL, -- USER | USER_GROUP
  role_id uuid NOT NULL, scope_type text NOT NULL, scope_id uuid, -- TENANT | CUSTOMER_SUBTREE | ENTITY_GROUP | ENTITY
  conditions jsonb);
```
- **Generic role:** `{resource → [operations]}` applying to all entities in the scope. **Group role:** operations on *entities in a specific group* (e.g., `Operators` may `READ_TELEMETRY` + `RPC_CALL` on group `Pump stations`).
- **Customer hierarchy:** customers can own sub-customers; ownership changes propagate group membership validation.
- **Resolution:** `Filter(principal, op, resource)` = union of scopes from all bindings (principal + its user groups) where the role grants `op`; compiled to SQL predicates (`tenant_id=…` AND (`customer_id IN subtree` OR `id IN (SELECT entity_id FROM entity_group_member WHERE group_id = ANY($groups))`)). Decision cache keyed by `(principal, scope-version)`; **deny-by-default**; optional explicit **deny** rules (P2) evaluated first.
- **ABAC conditions (P2):** `conditions` (CEL) over entity attributes/tags (`entity.tags.contains('prod') && ctx.time.hour in 8..18`).
- **Audit:** every role/binding/group change; **access review** report (who can do what on which group).
- **UI:** role editor matrix, group manager, "effective permissions" explorer for a user.

## 4. White-labeling
| Asset | Detail |
|---|---|
| Design tokens | Colours (primary/accent/surface/semantic), typography, radii, density — CSS variables; light/dark pairs; contrast validator (WCAG AA) |
| Brand assets | Logo (light/dark), favicon, login background, app name, footer links; stored as resources (sanitised SVG) |
| Domains | Custom domain per tenant/customer (CNAME) with **automatic TLS (ACME/Let's Encrypt)** at the edge proxy; domain→tenant map cached; CORS/CSP/OAuth redirect allow-lists derived |
| Login page | Custom text/legal links, SSO buttons per domain, sign-up policy |
| Email/SMS/notification templates | Branded base layouts, per-tenant overrides |
| Translations | Per-tenant overrides of i18n keys (ICU), custom locales |
| Menu/home | Hide/show/reorder nav items, custom home dashboard, custom links |
| PWA | Per-tenant manifest/icons/name |
Hierarchy: **system default → tenant → customer** override resolution with cache; preview mode before publish; versioned (`branding_revision`).

## 5. Version control and GitOps (`vcs-sync`)
- **Repository settings:** URL, branch, auth (deploy key / PAT / GitHub App — stored as secrets), author identity, default commit message templates, entity types to sync, `readOnly` flag.
- **Canonical export:** each entity → JSON file `entities/{type}/{name-or-externalId}.json` with **stable key ordering**, volatile fields removed (ids → `externalId`, timestamps, versions), secrets replaced by `${secret:name}`, references by `externalId` so the repo is **portable across environments**; related files (dashboard resources, widget bundles) alongside.
- **Operations:** manual **commit/push**, **load/restore** (dry-run diff → apply with dependency ordering), **auto-commit** (debounced per entity, attributed to the acting user), history/diff per entity, branch switch, **environment promotion** (dev → staging → prod via branches/PRs and the CLI `iotpctl vcs apply --env prod`), conflict detection by `entity_version`.
- **Continuous deployment:** webhook or poller triggers `apply` on merge; plan/apply output with approval gates; rollback = revert commit and apply.
- Implementation with `go-git`; workspace cache per tenant on object-store-backed volume; size/time limits; locks per tenant.

## 6. Solution templates (`solutions`)
- **Package:** manifest + entities (profiles, rule chains, alarm rules, calculated fields, dashboards, widget bundles, notification rules, SCADA UDTs/symbols, converters/integrations with placeholders) + demo-data generators (device simulators with scenarios) + README + parameters (`{{siteName}}`, thresholds).
- **Install:** wizard resolves parameters → creates entities in dependency order → records `solution_instance(id, tenant, package, version, created_entities[])`; **uninstall** removes only tracked entities; **upgrade** = 3-way diff on tracked entities (user edits preserved or flagged).
- **Catalogue:** built-in (smart building, energy monitoring, cold chain, fleet tracking, water network, industrial OEE) + marketplace.

## 7. SSO and directory sync
- **SAML 2.0** (`crewjam/saml`): per-domain IdP metadata, signed requests/assertions, attribute → user/role mapping, JIT provisioning, SLO.
- **OIDC** generic (see doc 02) with group claims → role bindings.
- **LDAP/AD** (`go-ldap`): bind auth, group sync (scheduled), nested groups, TLS.
- **SCIM 2.0 server (P2):** users/groups provisioning from Okta/Entra/etc.
- Enforce SSO-only per domain; break-glass local admin; session lifetime tied to IdP policy.

## 8. Secrets management
`secret(id, tenant_id, name, kind, value_enc, key_id, version, created_by, rotated_ts)`; kinds: `PASSWORD`, `TOKEN`, `SSH_KEY`, `TLS_CERT`, `OAUTH_CLIENT`, `GENERIC`. Referenced as `${secret:name}` in integrations, rule nodes, VCS, SMTP/SMS settings. **Write-only API** (value never returned); envelope encryption (AES-256-GCM data keys wrapped by KMS/Vault Transit; per-tenant KEK); rotation with versioning (`${secret:name@2}` pinning optional); access audit (who/when/which component resolved it); **resolved only inside executors** (never in UI/logs/exports); sync to edges sealed with the edge key.

## 9. Multi-region and data residency
- **Tenant home region** (`tenant.region`): data plane (PG, TS, bus, object store) is region-local; **global directory** (users→tenant→region) is a small replicated service for login routing; DNS/edge routes `tenant.example.com` to its region.
- Residency tags on exports/backups; cross-region replication only for explicit DR pairs; region-aware marketplace/artefact distribution; per-region upgrade rings.
- Edge/device endpoints are **region-specific** (device config carries regional hostnames; provisioning can redirect).

## 10. Marketplace and plugin catalogue (P2)
Registry of **widgets, rule nodes (WASM), integrations/converters, calculated-field templates, solution templates**: package format (`.iotppkg` zip + manifest + SBOM + signature), publisher verification, semver + compatibility range (`platform >=1.4 <2`), review/scan pipeline (static checks, sandbox tests), install with **capability prompts** (network, secrets, data scopes), usage telemetry (opt-in), revocation list, private catalogues per tenant/reseller.

## 11. Mobile / PWA
Installable PWA (offline shell, push via WebPush/FCM, deep links to alarms/dashboards, biometric unlock via WebAuthn, QR-based device claiming/provisioning (BLE/SoftAP helpers via Web Bluetooth where supported)); native wrappers later (Capacitor) reusing the same React bundle.

## 12. API (excerpt)
`CRUD /entityGroups` (+ members), `CRUD /roles`, `CRUD /roleBindings`, `GET /users/{id}/effective-permissions`, `GET/PUT /branding` (+ `/preview`, `/publish`), `CRUD /domains` (+ `/verify`), `CRUD /vcs/settings`, `POST /vcs/{commit|push|pull|restore|apply}`, `GET /vcs/diff`, `CRUD /solutions` (+ `/install`, `/uninstall`, `/upgrade`), `CRUD /secrets`, `CRUD /sso/{saml|ldap|oidc}`, `GET /marketplace/packages`, `POST /marketplace/install`.

## 13. Security notes
Role editing requires `ROLE_ADMIN` and step-up auth; prevent privilege escalation (cannot grant permissions you lack unless sysadmin); domain verification via DNS TXT before activating custom domains; VCS tokens as secrets; sanitised branding assets; marketplace code runs only in sandbox; all admin actions audited.

## 14. Testing
RBAC matrix tests (property-based: grants never exceed union of bindings; deny wins); SQL predicate equivalence tests (in-memory authorizer vs SQL filter); branding contrast/ sanitisation tests; VCS round-trip (export → import → export identical); solution install/uninstall idempotency; SSO flows against mock IdPs (SAML/LDAP containers).

## 15. Task checklist
- [ ] Entity groups, roles, bindings, scope compiler + decision cache; UI editors
- [ ] Customer hierarchy + ownership validation
- [ ] Secrets store + `${secret:}` resolution in executors
- [ ] White-label tokens/assets/domains(ACME)/templates/translations
- [ ] Canonical export format + `vcs-sync` (commit/restore/auto-commit/apply) + CLI
- [ ] Solution package format + installer + 3 launch solutions
- [ ] SAML + LDAP (+ SCIM later)
- [ ] Multi-region directory + routing + residency tags
- [ ] Marketplace registry + signing + sandbox policies
- [ ] PWA features (push, deep links, WebAuthn)
