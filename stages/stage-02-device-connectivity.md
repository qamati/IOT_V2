# Stage 2 — Device Connectivity & Registry

> **Estimate:** 24 eng-weeks · **Depends on:** Stage 1 · **Unlocks:** stage 3, 4, 7
> **Specs:** `services/04-device-registry.md`, `05-transport-mqtt.md`, `06-transports-http-coap-lwm2m-snmp.md` (HTTP part), `08-message-bus.md`, `00-overview/04-protocols-and-api.md`

## 1. Goal
Devices authenticate and push data into the platform over MQTT and HTTP using ThingsBoard-compatible topics/paths; messages land durably on the bus with correct tenant/profile context.

## 2. Scope
**In (P0):** device credentials (token, MQTT basic, X.509), `devreg` hot-path auth with caches, provisioning (all three strategies) + claiming (basic), `tr-mqtt` (v3.1.1 + v5, telemetry/attributes/RPC topics skeleton, TLS/mTLS, rate limits, session registry), `tr-http`, message bus (in-memory, NATS JetStream, Kafka), transport↔core downlink routing skeleton, device profile transport configuration.
**Out:** persistence of telemetry (stage 3), rule execution (stage 4), CoAP/LwM2M/SNMP (stage 8).

## 3. Deliverables
1. `pkg/bus` implementations with a shared conformance test-suite (ordering, at-least-once, consumer-group rebalance, DLQ).
2. `devreg` gRPC `DeviceAuthService` + provisioning REST/MQTT/HTTP endpoints; Redis + LRU caches with invalidation events.
3. `tr-mqtt` built on `mochi-mqtt` hooks: authenticate, ACL (device may only use `v1/devices/me/*`), decode, produce `Envelope`, PUBACK after bus ack.
4. `tr-http` implementing doc 04 §2 (except OTA/CoAP specifics).
5. `pkg/transport` common library: session manager, rate limiter, payload adaptors (JSON now, protobuf later), downlink consumer for `notify.transport.{nodeId}`.
6. Device simulator (`tools/device-sim`) supporting token/basic/X.509/gateway scenarios.
7. UI: device details (credentials, connectivity snippets), profile transport editor.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Message bus | Interfaces, mem, NATS, Kafka (franz-go), topic provisioning, conformance suite, DLQ | 4 |
| devreg | Credentials CRUD, lookup_id scheme, caches, negative cache, brute-force limiter, provisioning, claiming | 5 |
| tr-mqtt | Hooks, TLS, session registry, QoS handling, decoding, rate limits, graceful shutdown, metrics | 8 |
| tr-http | Endpoints, long-poll, auth, limits | 2 |
| Transport lib | Sessions, downlink routing, adaptors, limits | 2 |
| Profiles | Transport config schema + validation | 1 |
| Frontend | Device pages, connectivity helper, profile forms | 2 |

## 5. Acceptance criteria
- TB-compatible simulators (token, basic, X.509, gateway connect) pass the contract suite without modification.
- **Ack contract:** PUBACK is only sent after the bus acknowledges; killing the bus during a burst never produces an acknowledged-but-lost message (fault-injection test).
- 50 k idle MQTT connections per `tr-mqtt` pod (2 vCPU/4 GB) with < 1.5 GB RSS; 10 k msg/s sustained at p99 publish→bus < 50 ms.
- Credential cache hit ratio > 99 % in steady state; invalid-token flood from one IP is rate-limited without affecting valid devices.
- Provisioning: all three strategies work over MQTT and HTTP; X.509 chain provisioning verified with a test CA.
- Rolling restart of transports causes reconnect storm handled within configured jitter; no message duplication beyond at-least-once semantics.

## 6. Demo (exit)
Create a device → copy mosquitto command from UI → publish telemetry → show it on the bus (CLI tail) with tenant/profile metadata → kill a transport pod while publishing and show clients reconnect and zero acknowledged loss.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Embedded broker limitations (feature gaps vs. TB) | Wrapper hooks; keep an escape hatch to external broker + bridge |
| Reconnect storms | Server-side connect throttling, client jitter guidance |
| Credential cache poisoning/staleness | Event-based invalidation + short TTL |

## 8. Definition of Done
- [ ] Contract transcripts for every v1 topic/path committed
- [ ] Load test report attached (connections, throughput, memory)
- [ ] Security review: TLS settings, ACLs, auth rate limits
