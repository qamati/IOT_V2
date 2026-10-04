# Stage 12 — Enterprise Features

> **Estimate:** 40 eng-weeks · **Depends on:** Stages 1, 6 (+ 9 for edge-aware items) · **Unlocks:** commercial readiness
> **Specs:** `services/26-enterprise.md`, `20-scheduler-reporter.md`, `21-audit-events-usage.md`, `02-identity.md` (RBAC v2, SSO)

## 1. Goal
Make the platform governable and sellable: fine-grained RBAC, branding, GitOps, scheduling/reporting, complete audit/metering, secrets, SSO and packaged solutions.

## 2. Scope
**In (P1):** entity groups + roles + bindings + scope compiler (RBAC v2), customer hierarchy, secrets store + `${secret:}` resolution, white-labeling (tokens, assets, custom domains with ACME, templates, translations), version control (`vcs-sync`: canonical export, commit/restore/auto-commit/apply, environment promotion CLI), scheduler (cron/tz/misfire/fan-out) + reporter (PDF/PNG/CSV/XLSX/Parquet, delivery, retention), full audit (hash chain, anchors, SIEM export) + usage metering/ledger/billing hooks, solution templates (3 launch solutions), SSO (SAML, LDAP; OIDC group mapping).
**P2:** ABAC conditions (CEL), SCIM, marketplace/registry, multi-region directory + residency, PWA push/WebAuthn extras.

## 3. Deliverables
1. RBAC v2 in `identity`/`core` with SQL-scope compilation + effective-permissions explorer UI.
2. `secrets` module, `whitelabel` module, `vcs-sync` worker, `scheduler`, `reporter`, `audit`/usage modules.
3. Solution package format + installer + solutions: Smart Building, Energy Monitoring, Cold Chain.
4. SSO connectors with mock-IdP test fixtures; admin UIs for all settings.
5. Compliance evidence exports (access review, audit verification, retention proofs).

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| RBAC v2 | Groups/roles/bindings, scope compiler, decision cache, ownership validation, UI | 8 |
| Secrets | Store, envelope encryption, resolution in executors, write-only API, audit | 3 |
| White-label | Tokens, assets, domains+ACME, templates, translations, preview/publish | 5 |
| VCS/GitOps | Canonical export, go-git sync, restore/apply, promotion CLI, UI | 7 |
| Scheduler/reporter | Schedules, River workers, print mode, renderer pool, delivery, exports | 8 |
| Audit/usage | Hash chain, anchors, SIEM export, counters, state machine, ledger, billing hook | 5 |
| Solutions | Package format, installer/upgrade/uninstall, 3 solutions | 3 |
| SSO | SAML, LDAP, OIDC mappings, break-glass | 1 (+ continues) |

## 5. Acceptance criteria
- **RBAC:** property tests prove effective permissions = union of bindings (deny wins if enabled); in-memory authorizer and SQL filter agree on a 100 k-entity fixture; privilege-escalation attempts blocked.
- **VCS:** export → import → export is byte-identical; applying a repo to a clean tenant reproduces dashboards/chains/profiles; promotion dev→prod dry-run shows exact diff.
- **Reports:** scheduled dashboard PDF delivered at the right local time across DST; renderer sandbox verified (no internal network access, resource limits); 50 concurrent renders stable.
- **Audit:** tamper detection (modify a row → `verify` fails); GDPR subject erasure preserves chain validity; SIEM export lossless.
- **Metering:** counters accurate within ±1 % under load/restarts; monthly rollover and WARNING/DISABLED transitions correct; billing export idempotent.
- **White-label:** custom domain live with valid TLS in < 5 min after DNS verification; branding contrast validator blocks inaccessible palettes.
- **SSO:** SAML/LDAP flows pass interop tests; SSO-only enforcement with working break-glass.

## 6. Demo (exit)
Reseller creates a branded tenant on a custom domain with SSO, installs the Energy Monitoring solution, defines a group-scoped "Operators" role, schedules a weekly PDF report, promotes configuration from a staging tenant via Git, and shows audit verification plus usage vs plan.

## 7. Risks
| Risk | Mitigation |
|---|---|
| RBAC performance at scale | SQL-scope compilation, caching by scope-version, index on group membership |
| Headless-browser fragility | Dedicated print mode, readiness signal, pooled sandboxes, retries |
| Git sync edge cases | Canonical format tests, dependency-ordered apply, dry-run first |
| Secret leakage | Write-only API, executor-side resolution only, redaction tests |

## 8. Definition of Done
- [ ] Compliance evidence pack generated from the product
- [ ] Reseller/white-label guide + solution authoring guide published
- [ ] Security review for RBAC, secrets, SSO, VCS tokens
