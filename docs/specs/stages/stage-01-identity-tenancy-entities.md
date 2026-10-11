# Stage 1 — Identity, Tenancy & Entity Core

> **Estimate:** 24 eng-weeks · **Depends on:** Stage 0 · **Unlocks:** stages 2–7
> **Specs:** `services/01-apigw.md`, `02-identity.md`, `03-core.md`, `00-overview/03-data-model.md`, `frontend/01-architecture-and-pages.md`

## 1. Goal
Multi-tenant foundation: system admin creates tenants; tenant admins create customers, users, profiles, devices, assets and relations; everything is authenticated, authorised, audited and queryable.

## 2. Scope
**In (P0):** tenants + tenant profiles (quotas stored, enforcement later), customers (hierarchy-ready), users, JWT access/refresh, password policy/lockout/activation/reset, devices/assets (+profiles, no transport yet), relations, entity views (basic), EDQL v1 (filters, key filters, pagination, sort), API gateway, audit hooks, frontend shell.
**P1 (spillover allowed):** 2FA, OAuth2/OIDC, API keys.
**Out:** device protocols, telemetry, rule engine.

## 3. Deliverables
1. `identity`, `core`, `apigw` services; OpenAPI v1 for Auth/Tenant/Customer/User/Device/Asset/Profile/Relation/Query.
2. Postgres migrations for tenancy, identity, entities, relations (doc 03 §3) with RLS policies.
3. Permission model v1: authority-based (SYS_ADMIN/TENANT_ADMIN/CUSTOMER_USER) with a `Resource × Operation` matrix and owner-scope checks (designed so stage 12 RBAC plugs in).
4. React console shell: login, layout, navigation by role, tenant/customer/user/device/asset tables + forms, profile editor (JSON schema forms).
5. Audit events emitted for all mutations (consumed later by `audit`).
6. Seed: system admin, demo tenant, sample customers/devices.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Identity | Argon2id, JWT (EdDSA/JWKS, rotation), refresh with reuse detection, lockout, activation/reset mail, token_version revoke | 5 |
| Core entities | CRUD + validation + optimistic locking + cascade rules for tenant, customer, user, device, asset, profiles | 6 |
| Relations + EDQL | Relation CRUD, recursive traversal with depth/cycle guard, EDQL parser/compiler → SQL (squirrel/goqu), counts, permission scoping | 4 |
| API gateway | Router, middleware chain, rate limiting v1, OpenAPI serving, error mapping | 3 |
| DB & RLS | Migrations, RLS policies, tenancy helpers, indexes, search_text | 2 |
| Frontend shell | Auth flows, route guards, typed client, tables, forms, i18n scaffold, theming tokens | 4 |

## 5. Acceptance criteria
- Automated test: tenant A user cannot read/write any tenant B row via REST, gRPC or direct repo call (RLS + app checks).
- Customer user sees only entities of own customer subtree; tenant admin sees all tenant entities; sysadmin sees tenants only.
- JWT refresh rotation: reuse of an old refresh token revokes the family; `token_version` bump invalidates all access tokens within TTL.
- 5 consecutive failed logins lock the account for the configured period; audit entries exist.
- List endpoint (10 k devices, text search + sort) p95 < 100 ms; EDQL with 3 key filters p95 < 200 ms.
- OpenAPI spec validates, typed TS client generated, contract tests pass.

## 6. Demo (exit)
Sysadmin creates tenant → tenant admin activates via emailed link (Mailpit) → creates customer + customer user → creates device profile, 3 devices, asset with `Contains` relation → customer user logs in and sees only their devices.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Permission model too rigid for stage 12 | Abstract `Authorizer` interface now; ship matrix implementation behind it |
| EDQL becomes a SQL monster | Keep a closed filter set, golden-SQL tests, `EXPLAIN` budget tests |
| Entity cache invalidation bugs | Version-stamped cache keys + invalidation events on bus |

## 8. Definition of Done
- [ ] Cross-tenant isolation tests in CI (REST + gRPC + repo)
- [ ] Audit event for every mutating endpoint (checked by a reflection test)
- [ ] Migrations reversible and idempotent
- [ ] Security review of auth flows signed off
