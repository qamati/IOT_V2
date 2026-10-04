# Stage 8 — Integrations & Additional Protocols

> **Estimate:** 40 eng-weeks · **Depends on:** Stages 2–4, 7 · **Unlocks:** 9 (edge reuses connectors), 10
> **Specs:** `services/18-integration-converter.md`, `19-field-gateway.md`, `06-transports-http-coap-lwm2m-snmp.md`

## 1. Goal
Reach devices and systems that don't speak our native protocols: third-party networks (LoRaWAN, cloud IoT hubs, brokers), industrial protocols (Modbus, OPC UA), and the remaining device protocols (CoAP, LwM2M, SNMP).

## 2. Scope
**In (P1):** integration runtime (embedded + executor pool + **remote integration**), canonical `UplinkEvent`/identity resolver, converter engine (declarative → `expr`) + **tester UI**, connectors: HTTP/webhook, MQTT client, Kafka, ChirpStack, TTN/TTI, OPC UA client; `field-gateway` with Modbus + OPC UA + MQTT connectors, local buffer, remote config, stats; `tr-coap` (observe, blockwise, DTLS); `tr-snmp`; Sparkplug B (MQTT profile option + host app basics).
**P2:** `tr-lwm2m` (native or Leshan sidecar), BLE/CAN/BACnet/KNX/S7 connectors, Sigfox/AWS/Azure/GCP connectors, custom gRPC connector SDK, JS/WASM converters.

## 3. Deliverables
1. `cmd/integration`, shared `pkg/connectors` (SPI + Modbus/OPC UA/MQTT), `cmd/field-gateway`.
2. Converter editor with payload tester, saved test cases, debug capture.
3. Remote-integration enrollment + gRPC stream; per-integration status dashboard.
4. `tr-coap`, `tr-snmp`; `tr-lwm2m` decision record (native vs sidecar) + spike results.
5. Interop lab: Modbus/OPC UA simulators + at least two real PLCs; ChirpStack and TTN test networks.
6. Connector authoring guide.

## 4. Work breakdown
| Workstream | Tasks | EW |
|---|---|---|
| Integration core | Runtime, leases/checkpoints, identity resolver, pipeline, downlinks | 6 |
| Converters | Declarative engine, `expr` helpers, tester, debug capture | 5 |
| Connectors (cloud-side) | HTTP, MQTT, Kafka, ChirpStack, TTN/TTI | 6 |
| Field gateway | SPI, buffer, northbound (MQTT), config/remote mgmt, packaging | 6 |
| Modbus + OPC UA | Full-featured connectors + simulators + soak | 6 |
| CoAP + SNMP | Transports, interop, DTLS, pollers | 5 |
| Sparkplug B | Decode/encode, birth cache, commands | 2 |
| LwM2M (spike → impl) | Decision + core registration/observe (if native) | 4 |

## 5. Acceptance criteria
- **LoRaWAN:** 10 k simulated sensors via ChirpStack/TTN pipelines at 2 k events/s; downlink queueing verified; auto-created devices obey quotas.
- **Converters:** vendor payload corpus (≥ 30 decoders) passes golden tests; tester reproduces production output exactly.
- **Modbus:** 200 devices × 20 registers polled at 1 s with < 1 % missed polls; block-read coalescing reduces requests ≥ 50 %; write-with-verify works.
- **OPC UA:** 5 k monitored items with subscriptions; session recovery after server restart < 30 s; certificates trust flow documented/tested.
- **Offline:** field gateway survives 24 h disconnect at 1 k msg/s with ordered drain and no loss of alarms/events.
- **CoAP/SNMP:** interop with libcoap/aiocoap and `snmpsim`; CoAP observe delivers shared-attribute updates < 1 s.

## 6. Demo (exit)
Ingest from a ChirpStack network via an integration + converter authored in the tester; read a Modbus simulator and an OPC UA server through the field gateway; write a setpoint via RPC with verification; unplug the gateway's network and show store-and-forward catch-up.

## 7. Risks
| Risk | Mitigation |
|---|---|
| Protocol long tail | Priority list from real customers; custom gRPC connector SDK early |
| Vendor quirks (Modbus/OPC UA) | Interop lab with real hardware; recorded traces for regression |
| LwM2M effort | Sidecar (Leshan) fallback, strict time-box on native spike |
| Remote integration security | mTLS enrollment, scoped permissions, per-integration rate limits |

## 8. Definition of Done
- [ ] Connector SPI stable + documented; 6+ connectors shipped
- [ ] Interop reports archived; simulators in CI
- [ ] Field gateway packages (deb/rpm/docker/msi) published
