# Stage 14 — Security & Compliance

> **Estimate:** 24 eng-weeks (continuous track from stage 0; formal gate before GA) · **Depends on:** all stages
> **Specs:** `services/02-identity.md`, `04-device-registry.md`, `05-transport-mqtt.md`, `22-edge-hub.md`, `24-scada.md §11`, `26-enterprise.md`

## 1. Goal
Be demonstrably secure by design and by evidence: threat-modelled, tested, hardened, auditable — and aligned with the standards industrial and enterprise buyers ask for (IEC 62443, SOC 2, ISO 27001, GDPR).

## 2. Threat model (STRIDE per trust boundary)
| Boundary | Key threats | Primary controls |
|---|---|---|
| Device ↔ transport | Credential theft/stuffing, spoofing, flooding, payload attacks, downgrade | Strong tokens (≥ 128 bit), mTLS/X.509, constant-time compare, failed-auth bans, rate limits, protocol fuzzing, TLS 1.2+, no plaintext in prod |
| Browser ↔ API/WS | XSS, CSRF, token theft, IDOR, mass assignment, enumeration | Strict CSP, sanitisers, short-lived JWT + rotating refresh, scope-filtered queries, generated clients, authz matrix tests |
| Tenant ↔ tenant | Data leakage, noisy neighbour | `tenant_id` in every query + RLS, scoped caches/topics, fairness + quotas, L2–L4 isolation levels, cross-tenant test suite |
| Rule scripts / plugins | RCE, SSRF, resource exhaustion, secret exfiltration | `expr` default, `goja` interrupts, WASM sandbox with capabilities, SSRF guard, secrets by reference, quotas |
| Edge ↔ cloud | Rogue edge, replay, MITM, stolen disk | Enrollment tokens, internal CA + mTLS, per-edge authz, signed updates, TPM/sealed keys, disk encryption option |
| Integrations/webhooks | Spoofed callbacks, replay, SSRF | HMAC signatures, replay windows, mTLS for remote integrations, outbound allow-lists |
| SCADA control | Unauthorised/unsafe writes | Deny-by-default, SBO, interlocks, four-eyes, step-up auth, audit chain, network segmentation |
| Supply chain | Malicious dependency, compromised build | Pinned deps, SBOM, signed images/binaries, provenance (SLSA), reproducible builds, dependency review |
| Admin plane | Privilege escalation, insider misuse | Least privilege, break-glass with alerts, just-in-time admin, immutable audit |

## 3. Scope of work
**Design/process:** threat models per service (living docs), security requirements in each stage DoD, secure SDLC aligned to **IEC 62443-4-1** (practices SM, SR, SD, SI, SVV, DM, SUM, SG), security champions, code-owner reviews for auth/crypto.
**Engineering controls:** crypto agility + KMS/Vault (envelope encryption, key rotation), secrets hygiene (no secrets in env dumps/logs), input validation at boundaries, safe deserialisation, SSRF library, CSRF/CORS policy, security headers, DoS controls (limits everywhere, slow-request timeouts), WAF/ingress rules, network policies (default-deny), pod security (non-root, read-only FS, seccomp), image hardening (distroless), CIS baselines.
**Testing/verification:** SAST (gosec, semgrep), SCA (govulncheck, npm audit/osv), secret scanning, IaC scanning (checkov/trivy), DAST (ZAP) on staging, fuzzing (MQTT/CoAP/HTTP/protobuf/SVG sanitiser/expr), authz fuzz (cross-tenant), **independent penetration tests** (pre-GA and annually), bug bounty (post-GA), red-team exercise on edge/SCADA.
**Compliance mapping:** control matrices for **SOC 2 (CC series)**, **ISO 27001 Annex A**, **IEC 62443-3-3 SRs** (SL-2 target for SCADA/edge, roadmap to SL-3), **GDPR** (DSR export/erase, residency tags, retention, DPA templates, sub-processor list), **NIS2**-readiness notes, accessibility conformance report (WCAG 2.2 AA).
**Operations:** vulnerability management SLAs (critical 7 d, high 30 d), incident response plan + tabletop exercises, security logging to SIEM, anomaly detection on auth/audit streams, certificate lifecycle automation, backup encryption and restore tests.

## 4. Deliverables
1. Threat model documents + risk register; security architecture overview.
2. Hardened base images, Helm security defaults, network policies, policy-as-code (OPA/Kyverno).
3. CI security gates (SAST/SCA/secrets/IaC/image scan + fail thresholds), SBOM + signature + provenance on every release.
4. Fuzzing harnesses in nightly CI; cross-tenant authz fuzz suite.
5. Pen-test reports with tracked remediation; vulnerability disclosure policy (`security.txt`).
6. Compliance evidence generator (access reviews, audit verification, retention proofs, change logs).
7. IEC 62443-3-3 SR mapping for SCADA/edge + gap analysis.

## 5. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Threat modelling & SDLC | Models, requirements, champions, review gates | 3 |
| Platform hardening | Images, network/pod policies, TLS config, headers, DoS controls | 4 |
| Crypto/secrets | KMS integration, rotation, envelope encryption audit | 3 |
| Pipeline security | SAST/SCA/DAST/IaC/SBOM/signing/provenance | 4 |
| Fuzz & authz testing | Harnesses, corpora, cross-tenant fuzz | 3 |
| Pen test & remediation | Scoping, execution support, fixes | 3 |
| Compliance programme | SOC 2/ISO/GDPR/62443 mapping, evidence automation, policies | 4 |

## 6. Acceptance criteria
- Zero open critical/high findings from independent pen test at GA; medium findings have dated plans.
- Cross-tenant fuzzing (REST/gRPC/WS/bus) finds no data leakage across 24 h runs.
- All images/binaries signed; deployments reject unsigned artifacts; SBOM published per release.
- Secrets scanner finds none in repo/history; no secret appears in logs in log-scrubbing tests.
- DSR (export/erase) completes within SLA in test; audit chain verification passes after erasure.
- 62443-3-3 SR mapping shows SL-2 coverage for SCADA/edge with documented compensating controls.

## 7. Demo (exit)
Walk an auditor through: threat model → controls → live evidence (signed release + SBOM, authz test run, audit verification, access review report, DR restore log), then replay a red-team scenario (stolen edge disk, rogue webhook) and show detection and containment.

## 8. Risks
| Risk | Mitigation |
|---|---|
| Security as late-stage gate | Per-stage security DoD + continuous scanning from stage 0 |
| Compliance paper-chase | Evidence automation from product data |
| Plugin ecosystem attack surface | Capability model, review pipeline, sandbox, revocation |

## 9. Definition of Done
- [ ] Pen test passed; remediation verified
- [ ] SOC 2 Type I readiness assessment complete (Type II window started)
- [ ] Incident response tabletop executed; contacts and runbooks current
