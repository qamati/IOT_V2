# Service 06 — Other Transports: HTTP, CoAP, LwM2M, SNMP

> **Stages:** 2 (HTTP), 8 (CoAP, LwM2M, SNMP) · **Binaries:** `tr-http`, `tr-coap`, `tr-lwm2m`, `tr-snmp` · **Priority:** HTTP P0 · CoAP P1 · LwM2M P1/P2 · SNMP P2

## 1. Shared foundation: `pkg/transport`
Every transport is a *thin adaptor* over the same library, which is what lets new protocols cost weeks, not months.
```go
type Core interface {                                  // implemented by pkg/transport using devreg + bus
    Authenticate(ctx context.Context, req AuthRequest) (*Session, error)
    PostTelemetry(ctx context.Context, s *Session, kv []KV, ts int64) error   // ack-after-durable
    PostAttributes(ctx context.Context, s *Session, kv []KV) error
    RequestAttributes(ctx context.Context, s *Session, client, shared []string) (map[string]any, error)
    SubscribeAttributes(ctx context.Context, s *Session) (<-chan AttrUpdate, func())
    SubscribeRPC(ctx context.Context, s *Session) (<-chan RpcRequest, func())
    ReplyRPC(ctx context.Context, s *Session, id string, body []byte) error
    ClientRPC(ctx context.Context, s *Session, method string, params []byte, timeout time.Duration) ([]byte, error)
    Claim(ctx context.Context, s *Session, secret string, dur time.Duration) error
    Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResponse, error)
    OtaChunk(ctx context.Context, s *Session, kind, title, version string, chunk, size int) ([]byte, error)
    SessionEvent(s *Session, ev SessionEventKind)       // connect/disconnect/activity (coalesced)
}
```
Provides: session manager + Redis registry, rate limiters, payload adaptors (JSON/protobuf), downlink consumer (`notify.transport.{nodeId}`), metrics/tracing, graceful shutdown. A transport only implements wire framing, auth extraction, and mapping to `Core`.

## 2. HTTP transport (`tr-http`)
- Endpoints as in `04-protocols-and-api.md §2` (`/api/v1/{token}/…`).
- **Long-poll subscriptions:** `GET /attributes?timeout=T` and `GET /rpc?timeout=T` register a waiter in the session manager keyed by device; the first matching downlink completes the request; empty after timeout → `408`. Cap concurrent waiters per device (1 per kind) and per node; use `http.Server` with `ReadHeaderTimeout`, `IdleTimeout`.
- **Ack contract:** `200` only after bus ack; `429` on limits; `413` oversize; `400` malformed JSON (with machine-readable error).
- HTTP/2 enabled; `Content-Encoding: zstd/gzip` accepted on uploads; optional `If-None-Match` on attribute GETs.
- **OTA:** `GET /firmware` with `Range`; `ETag` = checksum; resumable.

## 3. CoAP transport (`tr-coap`)
- Library: `plgd-dev/go-coap` (UDP, TCP, DTLS via `pion/dtls`).
- Resources: `coap(s)://host/api/v1/{token}/{telemetry|attributes|rpc|claim|firmware|provision}`; JSON/CBOR/protobuf content formats selected by `Content-Format`.
- **Observe:** `attributes` and `rpc` support observe registrations; notifications on shared-attribute update / RPC arrival; client RPC via POST with token matching; **separate responses** for slow operations.
- Reliability: CON for telemetry (ack after bus ack), NON allowed per profile; block-wise transfer (RFC 7959) for OTA and large payloads; deduplication by `(MID, peer)` window.
- Security: DTLS PSK (identity = token) and X.509; DTLS connection-ID (RFC 9146) to survive NAT rebinding on cellular.
- Sleepy devices: session registry holds observe state with TTL; downlink while asleep → RPC stays queued (persistent RPC).

## 4. LwM2M transport (`tr-lwm2m`)
**Scope:** LwM2M 1.0/1.1 server + bootstrap server.
| Component | Responsibility |
|---|---|
| Registration store | Endpoint name → session, lifetime, binding (U/UQ/T/S), object links |
| Bootstrap server | Deliver security/server objects to devices (factory bootstrap, client-initiated) |
| Observe manager | Establish observe on configured resources; map notifications → telemetry/attributes; honour `pmin/pmax/gt/lt/st` attributes |
| Model registry | OMA/IPSO object definitions (XML/JSON) per tenant; resolves `/3/0/9` → name/type/units |
| Codecs | TLV, JSON (1.0), SenML-JSON, SenML-CBOR, plain text, opaque |
| Security | NoSec (dev only), PSK, RPK, X.509 (DTLS 1.2, CID) |
| RPC | Read, Write, Execute, Create, Delete, Discover, Write-Attributes mapped to server RPC methods |
| FOTA/SOTA | Objects 5 (firmware), 9 (software mgmt); push (write package URI) and pull; state/result → OTA status |
| Queue mode | Buffer downlinks for sleeping clients until next registration update |
**Profile mapping:** `telemetry[]`, `attributes[]`, `observe[]`, `attributeLwm2m{}` lists referencing resource paths; "keyName" map (`/3/0/9 → batteryLevel`). **Effort/risk:** the largest transport (≈ 10–14 eng-weeks). Options: (a) native Go on go-coap; (b) run **Eclipse Leshan** as a sidecar speaking a small gRPC/bus bridge initially, replace later.

## 5. SNMP transport (`tr-snmp`)
- Platform acts as **manager/poller** using `gosnmp`: profile defines `communicationConfigs[]` (spec: `TELEMETRY_QUERYING`, `CLIENT_ATTRIBUTES_QUERYING`, `SHARED_ATTRIBUTES_SETTING`, `TO_DEVICE_RPC_REQUEST`, optional `TRAP`), polling frequency, and `mappings[] {oid, key, dataType}`.
- Scheduler: hashed timing wheel per device; worker pool bounded by `maxConcurrentPolls`; jitter; per-target concurrency 1; exponential backoff on timeouts; `GETBULK` for tables; v1/v2c/v3 (auth MD5/SHA/SHA-2, priv DES/AES) with `engineID` discovery.
- RPC: SNMP GET/SET with OID templates; trap receiver (UDP 162) maps trap OIDs → events (P2).
- Device config in credentials: host, port, protocol version, community or USM user/keys (encrypted).

## 6. Session & downlink semantics by transport
| Transport | Session identity | Downlink delivery | Delivered ack |
|---|---|---|---|
| HTTP | none (request scoped) | long-poll waiter | HTTP 200 on poll completion |
| CoAP | token + observe registration | observe notification / separate response | CoAP ACK |
| LwM2M | registration (endpoint) | server-initiated operation / queue mode | operation response |
| SNMP | none (poll) | SET during poll window | SNMP response |

## 7. Configuration pattern (all)
`TR{X}_NODE_ID`, listen addresses, TLS/DTLS material, devreg address, bus config, limits; transport-specific: `TRHTTP_LONGPOLL_MAX`, `TRCOAP_BLOCKSIZE`, `TRLWM2M_BOOTSTRAP_ENABLED`, `TRSNMP_MAX_CONCURRENT_POLLS`.

## 8. Observability
Common: `iotp_tr_{name}_requests_total{op,result}`, `…_ack_seconds`, `…_sessions`, `…_ratelimited_total`. Specific: HTTP waiters gauge, CoAP retransmissions, LwM2M registrations/expired, SNMP poll duration/timeouts.

## 9. Testing
Contract transcripts (HTTP/CoAP) from TB-compatible clients; `libcoap`/`aiocoap` interop; Leshan client interop for LwM2M; `snmpsim` for SNMP; fuzz CoAP options and LwM2M TLV; DTLS handshake load tests.

## 10. Task checklist
- [ ] `pkg/transport` Core + session mgmt + limiter + downlink consumer
- [ ] `tr-http` endpoints, long-poll, OTA Range
- [ ] `tr-coap` resources, observe, blockwise, DTLS
- [ ] `tr-lwm2m`: registration, bootstrap, observe, codecs, model registry, RPC, FOTA, queue mode
- [ ] `tr-snmp`: poller, v1/v2c/v3, mappings, RPC, traps
- [ ] Interop + fuzz + load suites
