# Stage 5 — Alarms & Notifications

> **Estimate:** 24 eng-weeks · **Depends on:** Stage 4 · **Unlocks:** 6 (alarm widgets), 10 (SCADA alarms), 12
> **Specs:** `services/12-alarm.md`, `13-notification.md`

## 1. Goal
Detect conditions, manage the alarm lifecycle, and tell the right people reliably — with escalation, throttling and an in-app inbox.

## 2. Scope
**In (P0):** alarm lifecycle (create/update/ack/clear/assign/comment), unique-active-alarm constraint, propagation index, search/count APIs, **profile-level alarm rules** (SIMPLE, DURATION, REPEATING, schedules, dynamic thresholds), evaluator state snapshots, rule nodes (create/clear/assign alarm), notification service (targets, templates, rules, requests), channels **Inbox + Email + Slack**, escalation chains, system notifications (entity limits, API usage, edge offline placeholder), UI (alarm table/details, rule builder with test, notification admin, bell/inbox).
**P1:** entity/customer-scoped rules (2.0), MISSING_FOR + SCRIPT conditions, SMS/Teams/push/webhook channels, user preferences + DND, digest mode.
**Out:** ISA-18.2 states (stage 10).

## 3. Deliverables
1. `cmd/alarm` (lifecycle + evaluator) and `cmd/notify`.
2. Alarm rule model + compiler + subscription index + timers (duration/missing).
3. Rule-builder UI with **test on history** (replay last 24 h to preview alarms).
4. Template editor with preview/locales; target resolver honouring permissions.
5. Realtime alarm streams (hooks for stage 6) and counters.
6. Runbooks: stuck alarms, replay evaluator state, provider outage.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Alarm lifecycle | Schema, SQL upsert/ack/clear, comments, assign, events, TTL | 4 |
| Propagation + search | `entity_alarm`, recompute job, cursor APIs, counts | 3 |
| Evaluator | Compiler, state, schedules (tz/DST), dynamic values, snapshots | 6 |
| Notification core | Models, triggers, resolver, templates, queues, retries/DLQ | 5 |
| Channels | Inbox, SMTP, Slack (+ SSRF-safe webhook) | 2 |
| Escalation/throttle | Durable timers, cancel on ack/clear, throttle/dedupe | 2 |
| Frontend | Alarm pages, rule builder + tester, notification admin, inbox | 4 |

## 5. Acceptance criteria
- **Single active alarm:** 1 000 parallel triggers for one `(originator,type)` produce exactly one row; severity upgrades are applied once.
- **Duration/repeating** conditions correct with late/out-of-order data (fake-clock + replay datasets); restart mid-condition does not lose or duplicate alarms (snapshot + warm-up).
- **Propagation:** asset dashboards show child-device alarms; relation changes reconcile within 60 s.
- **Notification:** ack within the escalation delay **cancels** step 2 (verified by timeline test); provider outage triggers retries/backoff and eventual DLQ with admin notification.
- **Throughput:** 100 k evaluations/s per node; p99 alarm-create-to-inbox < 2 s end to end.
- **Security:** notification targets never expose users outside the originator's permission scope.

## 6. Demo (exit)
Build a rule "temp > 90 for 1 min → CRITICAL, notify Maintenance via email+Slack, escalate to Manager after 10 min unless acknowledged"; run a simulated device through it; acknowledge from the inbox email deep link; show audit trail and comments.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Alarm floods | Coalescing ≤ 1 update/s per alarm, throttle/digest, flood metrics |
| Evaluator state drift after rebalance | Snapshots + warm-up re-evaluation of latest values |
| Email deliverability | DKIM/SPF via relay, bounce handling, per-tenant sender domains (P1) |

## 8. Definition of Done
- [ ] Lifecycle + evaluator property tests; time-travel tests for conditions
- [ ] Escalation timeline tests; provider chaos tests
- [ ] Alarm/notification dashboards and alerts live
- [ ] Docs: rule cookbook (10 patterns) + notification setup guide
