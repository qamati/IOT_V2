# Service 03 — Core Entity Service (`core`)

> **Stage:** 1 (core), 12 (groups/roles) · **Binary:** `cmd/core` · **State:** Postgres (+ read-model cache) · **Priority:** P0

## 1. Purpose
System of record for the **entity domain**: tenants, tenant profiles, customers, users' org placement, devices/assets and their profiles, entity views, relations, tags, queues config, and the **entity query engine (EDQL)**. Emits lifecycle events for every other service.

## 2. ThingsBoard reference
Entities with ownership (tenant/customer), `additionalInfo`, free-form relations (`from`, `to`, `type`, `typeGroup`), entity views (restricted virtual views), search APIs, the **Entity Data Query** API (entity filters + key filters + latest values + pagination) and counts. Since 4.0 an **EDQS** in-memory read model (fed by `edqs.events`, snapshotted in a compacted `edqs.state` topic, queried through `edqs.requests`) offloads dashboards from SQL.
**Lesson:** design the query contract once; allow a SQL implementation first and a read-model implementation later behind the same API.

## 3. Capabilities
| Area | Spec | Pri |
|---|---|---|
| CRUD | Tenant, TenantProfile, Customer (hierarchy), Device/Asset (+profiles), EntityView, Dashboard references, Queue config | P0 |
| Ownership | `owner = tenant | customer`; `changeOwner` with validation of dependent entities (dashboards, relations) | P0 |
| Optimistic concurrency | `version` + `If-Match`; 409 on conflict | P0 |
| Validation | Names unique per tenant+type, profile schemas, JSON-schema for `additionalInfo` (optional per profile) | P0 |
| Quotas | Enforce `tenant_profile.entities.*` on create (§5) | P0 |
| Lifecycle events | `EntityEvent{CREATED,UPDATED,DELETED,ASSIGNED,UNASSIGNED}` via transactional outbox | P0 |
| Relations | CRUD, directional queries, depth-limited traversal, cycle-safe | P0 |
| EDQL | Filters, key filters, sort, paging, counts; permission-scoped | P0 |
| Search | Text search (`tsvector`/`pg_trgm`), tags, label | P0 |
| Bulk | CSV import/export with dry-run and error report | P1 |
| Entity views | Key + time-window restricted views over another entity | P1 |
| Graph/twin model | Typed schemas on entities + graph queries | P2 |

## 4. Transactional outbox (guarantees DB + bus consistency)
```sql
CREATE TABLE outbox (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id uuid NOT NULL, topic text NOT NULL, key bytea NOT NULL, payload bytea NOT NULL,
  created_time bigint NOT NULL, published boolean NOT NULL DEFAULT false
);
CREATE INDEX outbox_unpublished ON outbox (id) WHERE published = false;
```
Entity mutations insert the row **in the same transaction**; a relay (`FOR UPDATE SKIP LOCKED`, batch 500, or logical-replication/Debezium in cluster mode) publishes to `entity.events` and marks rows; consumers are idempotent (event id = outbox id). Published rows are pruned after 24 h.

## 5. Quotas
Create path: `SELECT pg_advisory_xact_lock(hashtext(tenant_id||entity_type))` → `count(*)` check (cached counter) → insert. For hot creation paths use Redis `INCR` counters reconciled periodically from `count(*)`. Over-quota → `429`/`409 QUOTA_EXCEEDED` + notification event `ENTITIES_LIMIT`.

## 6. Relations
```go
type RelationsQuery struct {
    Root       EntityRef
    Direction  Direction            // FROM | TO
    MaxLevel   int                  // default 10, hard cap 20
    FetchLastLevelOnly bool
    Filters    []RelationFilter     // relationType, entityTypes
    Group      string               // COMMON | DASHBOARD | RULE_CHAIN | EDGE
}
```
Traversal uses a **recursive CTE** with `CYCLE` detection and depth cap:
```sql
WITH RECURSIVE walk(from_id,to_id,type,depth,path) AS (
  SELECT from_id,to_id,relation_type,1,ARRAY[from_id,to_id] FROM relation
   WHERE from_id=$1 AND relation_type_group=$2
  UNION ALL
  SELECT r.from_id,r.to_id,r.relation_type,w.depth+1,w.path||r.to_id FROM relation r
   JOIN walk w ON r.from_id=w.to_id
   WHERE w.depth<$3 AND r.relation_type_group=$2 AND NOT r.to_id=ANY(w.path)
) SELECT * FROM walk;
```
For hot hierarchies (building → floor → room → device) maintain an optional **materialised path** (`ltree`) or closure table updated on relation change (P2) so ancestor/descendant lookups are index scans.

## 7. EDQL (entity data query language)
**Request shape** (JSON, see `04-protocols-and-api.md §5`): `entityFilter` + `entityFields[]` + `latestValues[]` (+ `keyFilters[]`) + `pageLink{page,pageSize,textSearch,sortOrder{key,direction}}`.

| Filter type | Meaning |
|---|---|
| `singleEntity`, `entityList`, `entityName` (prefix) | Direct selection |
| `entityType` | All of a type (scoped) |
| `deviceType` / `assetType` / `entityViewType` / `edgeType` | By profile/type name list |
| `relationsQuery` | Start from a root, follow relations, filter by entity types |
| `deviceSearchQuery`, `assetSearchQuery`, … | Relations + type constraint |
| `stateEntity` | Resolved from dashboard state (client-side substitution) |
| `apiUsageState` | Usage entities |

**Key filters:** `{key{type: ATTRIBUTE|TIME_SERIES|ENTITY_FIELD|CONSTANT, key}, valueType, predicate}` with predicates `STRING (EQUAL, NOT_EQUAL, STARTS_WITH, ENDS_WITH, CONTAINS, NOT_CONTAINS, IN, NOT_IN)`, `NUMERIC (EQUAL…GREATER_OR_EQUAL…)`, `BOOLEAN`, and `COMPLEX (AND/OR)`; values may be **dynamic** (take from current user/customer/tenant attribute).

**Compilation (SQL implementation):**
1. `candidate_ids` CTE from the filter, always `WHERE tenant_id = $t` plus the **authorizer scope** (customer subtree / group ids).
2. For each requested field/latest key add a `LEFT JOIN LATERAL` to `attribute_kv` / `ts_kv_latest` (by `key_id`), or entity columns.
3. Key filters become `WHERE` clauses on the joined typed columns (choose `long_v/dbl_v/str_v/bool_v` by `valueType`).
4. Sort by field or latest value; keyset pagination when sorting by indexed columns, `OFFSET` otherwise (guarded by max offset).
5. Count query shares the same CTE.
**Guards:** `EXPLAIN` budget tests in CI; query timeout 5 s; max 50 keys, 10 key filters; per-tenant concurrency limit.

**Read-model upgrade path (P2):** an `edql-index` consumer builds in-memory indexes (per tenant: entities by id/type/profile/name trigram, latest values map) from `entity.events` + `rt.updates`; compacted `edql.state` topic for fast restarts; same `Query` API; use when p95 SQL > 200 ms or QPS > threshold. Read-your-writes via version watermark (`X-Min-Version` header).

## 8. Deletion semantics
`DELETE device` → transaction deletes entity + credentials + relations (both directions) + attribute rows; emits `ENTITY_DELETED` → other services purge their data async (telemetry purge job, alarms, RPC, OTA assignments, edge assignments, dashboards references flagged). Soft-block when entity is referenced by an active rule chain/dashboard alias unless `force=true`.

## 9. Caching
`entity:{tenant}:{type}:{id}` (versioned) in Redis + in-process LRU; invalidation by `entity.events` (all instances subscribe); credentials cache lives in `devreg`. Cache keys include `version` so stale writes can't resurrect old data.

## 10. API groups
`/tenants`, `/tenantProfiles`, `/customers`, `/devices`, `/deviceProfiles`, `/assets`, `/assetProfiles`, `/entityViews`, `/relations`, `/entitiesQuery/{find,count}`, `/entityQueries/keys`, `/tags`, `/queues`, `/import/{type}`, `/export/{type}` (see doc 04 §4).

## 11. Observability
`iotp_core_entity_ops_total{type,op,result}`, `iotp_core_edql_seconds{filter}`, `iotp_core_outbox_lag_seconds`, `iotp_core_quota_denied_total{type}`, `iotp_core_cache_hit_ratio`. Slow query log with normalised SQL.

## 12. Testing
Cross-tenant isolation suite (REST/gRPC/repo); EDQL golden SQL + result tests against a fixture of 100 k entities; relation traversal property tests (cycles, depth); outbox crash tests (kill relay between publish and mark → duplicates harmless); migration up/down tests.

## 13. Task checklist
- [ ] Entity repositories (sqlc) + services + validation + optimistic locking
- [ ] Outbox + relay + `entity.events` contract
- [ ] Quotas + counters + events
- [ ] Relations CRUD + traversal + guards
- [ ] EDQL parser/compiler/executor + count + keys discovery
- [ ] Search (tsvector/trgm/tags)
- [ ] Ownership change + deletion semantics
- [ ] Cache + invalidation
- [ ] CSV bulk import/export
- [ ] Entity views
- [ ] (P2) read-model index; typed twin schemas + graph queries
