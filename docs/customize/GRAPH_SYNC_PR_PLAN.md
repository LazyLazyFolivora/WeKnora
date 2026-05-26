# Graph Sync PR Implementation Plan

## Background

This PR extends WeKnora's existing knowledge-graph path without changing the upstream graph extraction model more than necessary.

The existing graph path writes extracted `types.GraphData` directly to Neo4j through `RetrieveGraphRepository.AddGraph`. That is useful for lightweight GraphRAG, but it makes Neo4j behave like the primary data store and leaves little room for metadata, review status, source traceability, retry state, and rebuildable projections.

`docs/GRAPH_SYNC_DESIGN.md` defines the target boundary:

- The relational database is the source of truth.
- Neo4j is a derived graph projection.
- All create/update/delete operations write the database first.
- Neo4j can be rebuilt from database rows.

## Goals

1. Add a database-backed graph entity and relation store.
2. Accept curated/manual graph data through HTTP APIs and persist it to the database.
3. Track sync lifecycle for database-to-Neo4j projection.
4. Add an idempotent projection path from database rows to Neo4j.
5. Keep the existing upstream graph extraction path working.
6. Minimize changes to upstream hot files by adding new files and using small registration-only edits where unavoidable.

## Non-Goals

1. Replace the existing `types.GraphNode`, `types.GraphRelation`, or `RetrieveGraphRepository.AddGraph` extraction path in the first PR.
2. Store every source, audit, and confidence field in Neo4j.
3. Make Neo4j the authoritative graph source.
4. Add frontend graph editing UI in the first PR.
5. Change existing chat GraphRAG retrieval semantics unless explicitly done in a later PR.

## Current State

Existing lightweight graph types:

```go
type GraphNode struct {
    Name       string   `json:"name,omitempty"`
    Chunks     []string `json:"chunks,omitempty"`
    Attributes []string `json:"attributes,omitempty"`
}

type GraphRelation struct {
    Node1 string `json:"node1,omitempty"`
    Node2 string `json:"node2,omitempty"`
    Type  string `json:"type,omitempty"`
}
```

Existing Neo4j projection shape:

- Node identity: `{name, kg}`.
- Node labels: namespace-derived labels such as `ENTITY<kb_id>` and `ENTITY<knowledge_id>`.
- Node properties: `name`, `kg`, `attributes`, `chunks`.
- Relationship type: `GraphRelation.Type`.

This PR should not mutate that existing shape in-place. The new DB-backed graph projection should use a separate model and separate Neo4j label to avoid breaking existing behavior.

## Proposed Architecture

```text
HTTP API
  -> GraphSyncHandler
  -> GraphSyncService
  -> GraphEntityRepository / GraphRelationRepository
  -> graph_entities / graph_relations tables
  -> GraphProjectionService
  -> Neo4j projection repository
  -> Neo4j (:GraphEntity)-[:RELATION]->(:GraphEntity)
```

The import/write side never writes Neo4j directly. It only writes database rows with `sync_status = pending`.

The projection side reads pending or failed rows, writes Neo4j idempotently, and updates database sync state.

## Package Layout

Prefer additive files:

```text
internal/types/graph_sync.go
internal/types/interfaces/graph_sync.go
internal/application/repository/graph_sync.go
internal/application/service/graph_sync.go
internal/application/service/graph_schema.go
internal/application/service/graph_projection.go
internal/handler/graph_sync.go
internal/router/graph_sync_routes.go
internal/application/repository/retriever/neo4j/graph_projection.go
```

Minimal existing-file edits:

```text
internal/container/container.go     # register repositories/services/handlers
internal/router/router.go           # add handler param and RegisterGraphSyncRoutes call
internal/router/task.go             # optional, only when projection task uses Asynq
internal/router/sync_task.go        # optional, only when projection task supports lite mode
```

## Data Model

### GraphEntity

```go
type GraphEntity struct {
    ID             string
    TenantID       uint64
    KnowledgeBaseID string

    SourceEntityID string
    EntityType     string
    EntityName     string
    EntityData     JSONMap

    SourceDocUUID   string
    SourceSite      string
    SourceText      string
    ConfidenceScore *float64
    ConfidenceReason string
    ReviewStatus    string

    SyncStatus string
    Neo4jNodeID string
    IsDeleted bool
    SyncedAt *time.Time
    SyncError string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Unique key:

```text
tenant_id + knowledge_base_id + source_entity_id
```

### GraphRelationRecord

```go
type GraphRelationRecord struct {
    ID              string
    TenantID        uint64
    KnowledgeBaseID string

    SourceRelationID string
    FromEntityID     string
    ToEntityID       string
    RelationType     string
    RelationProps    JSONMap

    SourceDocUUID    string
    SourceSite       string
    SourceText       string
    ConfidenceScore  *float64
    ConfidenceReason string
    ReviewStatus     string

    SyncStatus string
    Neo4jRelationID string
    IsDeleted bool
    SyncedAt *time.Time
    SyncError string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Unique key:

```text
tenant_id + knowledge_base_id + source_relation_id
```

`FromEntityID` and `ToEntityID` refer to `source_entity_id` within the same tenant and knowledge base.

## Sync Status

Use the same status set for entities and relations:

```text
pending
synced
failed
deleted
skipped
```

Transitions:

```text
upsert row        -> pending
projection ok     -> synced
projection failed -> failed + sync_error
soft delete       -> pending + is_deleted=true
neo4j delete ok   -> deleted
```

## HTTP APIs

### Batch Upsert Entities

```text
POST /api/v1/knowledge-bases/:id/graph/entities:batch-upsert
```

Request:

```json
{
  "entities": [
    {
      "source_entity_id": "1",
      "entity_type": "Drug",
      "entity_name": "SYSA1801",
      "entity_data": {
        "name": "SYSA1801",
        "modality": "ADC"
      },
      "source_doc_uuid": "doc-001",
      "source_site": "example",
      "source_text": "SYSA1801 是一种靶向 CLDN18.2 的 ADC 药物。",
      "confidence_score": 0.92,
      "confidence_reason": "原文明确提及",
      "review_status": "approved",
      "is_deleted": false
    }
  ]
}
```

Response:

```json
{
  "success": true,
  "data": {
    "upserted": 1,
    "deleted": 0,
    "skipped": 0
  }
}
```

### Batch Upsert Relations

```text
POST /api/v1/knowledge-bases/:id/graph/relations:batch-upsert
```

Request:

```json
{
  "relations": [
    {
      "source_relation_id": "1001",
      "from_entity_id": "1",
      "to_entity_id": "2",
      "relation_type": "TARGETS",
      "relation_props": {
        "mechanism": "抑制剂"
      },
      "source_doc_uuid": "doc-001",
      "source_site": "example",
      "source_text": "SYSA1801 是一种靶向 CLDN18.2 的 ADC 药物。",
      "confidence_score": 0.91,
      "confidence_reason": "原文明确提及",
      "review_status": "approved",
      "is_deleted": false
    }
  ]
}
```

Response shape is the same as entities.

### Backward-Compatible Manual Import Endpoint

If keeping the existing experimental endpoint:

```text
POST /api/v1/knowledge-bases/:id/graph/import
```

The handler should convert legacy `nodes` and `relations` into graph sync entity/relation rows and persist them to the database. It must not call Neo4j directly.

Recommended conversion:

- `GraphNode.Name` -> `source_entity_id` and `entity_name` when no explicit ID exists.
- `GraphNode.Attributes` -> `entity_data.attributes`.
- `GraphRelation.Node1` / `Node2` -> `from_entity_id` / `to_entity_id`.
- `GraphRelation.Type` -> `relation_type`.
- `source_relation_id` can be deterministic: hash of `kb_id + node1 + type + node2`.

This keeps the endpoint useful for quick import tests while routing all writes through the new DB-backed path.

## Validation

Add `internal/application/service/graph_schema.go` with:

```go
var allowedGraphEntityTypes = map[string]struct{}{...}
var allowedGraphRelationTypes = map[string]struct{}{...}
var allowedGraphRelationDirections = map[string][]GraphRelationDirection{...}
```

Entity types from design:

```text
Company
Drug
Target
Indication
ClinicalTrial
DealEvent
DealItem
ApprovalEvent
Policy
TCMFormula
Compound
Pathway
DevelopmentProject
TrialIndication
```

Relation types from design:

```text
DEVELOPS
PARTICIPATES_IN
INVESTED_IN
SUBSIDIARY_OF
SPONSORS
ISSUES
APPROVES
PARTICIPATES_IN_PROJECT
TARGETS
TREATS
IN_TRIAL
HAS_APPROVAL
HAS_ITEM
INVOLVES_DRUG
EVALUATES
FOR_INDICATION
IN_PATHWAY
ASSOCIATED_WITH
HOMOLOG_OF
SUBTYPE_OF
AFFECTS
DEVELOPED_BY
CONTAINS
```

Validation rules:

1. `source_entity_id`, `entity_type`, and `entity_name` are required.
2. `source_relation_id`, `from_entity_id`, `to_entity_id`, and `relation_type` are required.
3. Entity type must be allowed.
4. Relation type must be allowed.
5. Relation endpoints must exist in the same tenant and knowledge base.
6. Relation direction should match the allowed direction matrix.
7. `relation_type` must be sanitized before any Neo4j Cypher interpolation.
8. `confidence_score`, when present, must be between 0 and 1.

## Database Migrations

Add versioned and sqlite migrations:

```text
migrations/versioned/0000xx_graph_sync.up.sql
migrations/versioned/0000xx_graph_sync.down.sql
migrations/sqlite/0000xx_graph_sync.up.sql
migrations/sqlite/0000xx_graph_sync.down.sql
```

Postgres fields should use `JSONB`; SQLite should store JSON as `TEXT` and let Go marshal/unmarshal.

Suggested indexes:

```sql
CREATE UNIQUE INDEX idx_graph_entities_unique_source
  ON graph_entities (tenant_id, knowledge_base_id, source_entity_id);

CREATE INDEX idx_graph_entities_sync
  ON graph_entities (sync_status, updated_at);

CREATE INDEX idx_graph_entities_doc
  ON graph_entities (tenant_id, knowledge_base_id, source_doc_uuid);

CREATE UNIQUE INDEX idx_graph_relations_unique_source
  ON graph_relations (tenant_id, knowledge_base_id, source_relation_id);

CREATE INDEX idx_graph_relations_sync
  ON graph_relations (sync_status, updated_at);

CREATE INDEX idx_graph_relations_doc
  ON graph_relations (tenant_id, knowledge_base_id, source_doc_uuid);
```

## Repository Design

`GraphEntityRepository`:

```go
BatchUpsert(ctx context.Context, rows []*types.GraphEntity) error
ListForProjection(ctx context.Context, limit int) ([]*types.GraphEntity, error)
MarkSynced(ctx context.Context, id string, neo4jNodeID string, syncedAt time.Time) error
MarkFailed(ctx context.Context, id string, errMsg string) error
MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error
FindBySourceIDs(ctx context.Context, tenantID uint64, kbID string, sourceIDs []string) ([]*types.GraphEntity, error)
```

`GraphRelationRepository`:

```go
BatchUpsert(ctx context.Context, rows []*types.GraphRelationRecord) error
ListForProjection(ctx context.Context, limit int) ([]*types.GraphRelationRecord, error)
MarkSynced(ctx context.Context, id string, neo4jRelationID string, syncedAt time.Time) error
MarkFailed(ctx context.Context, id string, errMsg string) error
MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error
```

Use GORM `OnConflict` for idempotent upsert.

## Service Design

`GraphSyncService` owns import/write-side behavior:

1. Resolve KB and tenant context.
2. Validate entity/relation schema.
3. Convert request DTOs into DB rows.
4. Batch upsert rows.
5. Mark changed rows `pending`.
6. Optionally enqueue projection task.

`GraphProjectionService` owns DB-to-Neo4j projection:

1. Read pending/failed entities first.
2. Merge or delete Neo4j nodes.
3. Update entity sync status.
4. Read pending/failed relations second.
5. Merge or delete Neo4j relations.
6. Update relation sync status.

Projection should be idempotent and safe to retry.

## Neo4j Projection Shape

Use a separate label from the existing extraction graph:

```cypher
(:GraphEntity {
  id,
  tenant_id,
  knowledge_base_id,
  source_entity_id,
  type,
  name,
  properties
})
```

Relationship:

```cypher
(:GraphEntity {id})-[:TARGETS {
  id,
  source_relation_id,
  properties
}]->(:GraphEntity {id})
```

Node merge:

```cypher
MERGE (n:GraphEntity {id: $id})
SET
  n.tenant_id = $tenant_id,
  n.knowledge_base_id = $knowledge_base_id,
  n.source_entity_id = $source_entity_id,
  n.type = $type,
  n.name = $name,
  n.properties = $properties
```

Relationship merge requires code-level relation type validation before string interpolation:

```cypher
MATCH (from:GraphEntity {id: $from_id})
MATCH (to:GraphEntity {id: $to_id})
MERGE (from)-[r:TARGETS {id: $id}]->(to)
SET
  r.source_relation_id = $source_relation_id,
  r.properties = $properties
```

## Task Execution

MVP option:

- Import APIs only mark rows as `pending`.
- Add an admin/internal endpoint to trigger projection manually.

Full option:

- Register an Asynq task type, for example `types.TypeGraphProjection`.
- Import service enqueues projection after DB commit.
- Lite mode registers the same task with `SyncTaskExecutor`.

Recommended first PR behavior:

1. Implement projection service and manual trigger endpoint.
2. Add Asynq wiring in a follow-up PR if upstream maintainers prefer smaller review chunks.

## API Permissions

Use the same guard shape as other KB mutations:

```go
g.OwnedKBOrAdmin()
g.KBAccessWrite("id")
```

Reasoning:

- Graph imports mutate KB-scoped derived knowledge.
- Shared KB write permission must still resolve the effective tenant context.
- KB creator or Admin+ should be required for this first PR, matching sensitive KB mutation routes.

## Compatibility Strategy

1. Do not remove or alter `types.GraphNode`, `types.GraphRelation`, or `types.GraphData`.
2. Do not change `RetrieveGraphRepository.AddGraph` behavior in the initial PR.
3. Add new DB-backed graph sync code beside the existing extraction graph.
4. Keep existing GraphRAG behavior untouched unless a later PR explicitly migrates it.
5. If `/graph/import` already exists on the branch, make it a compatibility wrapper over `GraphSyncService`.

## PR Breakdown

Recommended split:

### PR 1: DB-backed graph sync import

- Add graph sync types and interfaces.
- Add DB migrations.
- Add repositories.
- Add service validation and batch upsert.
- Add handlers/routes.
- Make legacy manual import endpoint persist to DB, not Neo4j.
- Add repository/service tests.

### PR 2: Neo4j projection

- Add Neo4j projection repository methods.
- Add projection service.
- Add manual projection trigger endpoint or task.
- Add tests around relation type validation and sync status transitions.

### PR 3: Operational polish

- Add retry listing/manual retry APIs.
- Add source document rollback APIs.
- Add full Neo4j rebuild task.
- Add UI integration if needed.

## Test Plan

Unit tests:

1. Entity validation rejects missing required fields.
2. Entity validation rejects unknown entity type.
3. Relation validation rejects unknown relation type.
4. Relation validation rejects missing endpoints.
5. Relation direction matrix is enforced.
6. Batch upsert is idempotent.
7. Updating an existing row resets `sync_status` to `pending`.
8. Soft delete sets `is_deleted = true` and `sync_status = pending`.

Repository tests:

1. GORM upsert works for Postgres-compatible SQL generation where practical.
2. SQLite repository tests cover Lite mode.
3. `ListForProjection` returns `pending` and `failed` rows ordered by `updated_at`.

Projection tests:

1. Entity projection calls MERGE with expected ID and properties.
2. Deleted entity projection deletes node and marks row `deleted`.
3. Relation projection validates relation type before Cypher construction.
4. Failed projection writes `sync_status = failed` and `sync_error`.

Manual verification:

1. Start app with DB only and import entities/relations.
2. Confirm rows are stored as `pending`.
3. Start app with Neo4j enabled and run projection.
4. Confirm Neo4j has minimal nodes/edges.
5. Update a row and confirm projection is idempotent.
6. Soft delete a row and confirm Neo4j projection removes it.

## Open Questions

1. Should `source_entity_id` be globally unique from the producer, or only unique within a KB?
   Current plan assumes uniqueness within `(tenant_id, knowledge_base_id)`.
2. Should only `review_status = approved` rows project to Neo4j?
   Recommended default: yes, but MVP can project all non-deleted rows and add review gating later.
3. Should relation endpoint validation require both endpoints already exist in DB before relation upsert?
   Recommended: yes, to keep projection deterministic.
4. Should projection be automatic on import or manually triggered first?
   Recommended for first PR: manual trigger or enqueue behind a small service option to reduce review scope.
5. Should dynamic type labels like `:Drug` be added in Neo4j?
   Recommended: keep only `:GraphEntity` first; add type labels later if query performance requires them.

## Review Notes for Upstream

This plan deliberately adds a new DB-backed graph sync path beside the existing extraction graph instead of rewriting upstream GraphRAG internals. The goal is to make the feature reviewable and low-risk:

- Existing extraction and chat behavior remains unchanged.
- New APIs are KB-scoped and permissioned like other KB mutations.
- Neo4j writes move behind a projection layer.
- The database can always rebuild Neo4j.
- Most implementation lives in new files, with only small registration edits in existing files.
