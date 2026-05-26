# 图谱同步 PR 开发方案

## 1. 背景

本方案面向向 WeKnora 上游提交 PR 的实现视角，目标是在不大幅改动现有 GraphRAG / Neo4j 抽取链路的前提下，新增一条“数据库为事实源、Neo4j 为投影”的图谱同步能力。

当前已有链路会把 `types.GraphData` 直接写入 Neo4j：

```text
文档 chunk
  -> LLM 抽取 GraphNode / GraphRelation
  -> RetrieveGraphRepository.AddGraph
  -> Neo4j
```

这条链路适合轻量 GraphRAG，但有几个问题：

1. Neo4j 实际承担了主数据存储角色。
2. 实体元数据、来源文本、置信度、审核状态、同步状态难以完整表达。
3. 失败重试、软删除、按来源回滚、全量重建都缺少数据库状态支撑。
4. 如果直接改原有 Neo4j 写入模型，容易影响上游既有行为，也容易产生冲突。

因此本 PR 应新增一套并行能力：

```text
HTTP / 导入逻辑
  -> 业务数据库 graph_entities / graph_relations
  -> sync_status = pending
  -> DB 到 Neo4j 投影任务
  -> Neo4j 最小图结构
```

核心原则与 `GRAPH_SYNC_DESIGN.md` 一致：

> 数据库是事实源，Neo4j 是图谱投影。所有增删改操作只写数据库，不直接写 Neo4j。

## 2. 目标

1. 新增图谱实体表和关系表，保存完整元数据。
2. 新增批量实体/关系导入接口，导入后只写数据库。
3. 为实体和关系维护同步状态。
4. 新增数据库到 Neo4j 的幂等投影逻辑。
5. 保持原有 `GraphNode` / `GraphRelation` / `AddGraph` 抽取链路可用。
6. 尽量通过新增文件实现，只对上游热点文件做少量注册改动。

## 3. 非目标

首个 PR 不做以下事情：

1. 不替换现有 `types.GraphNode`、`types.GraphRelation`、`types.GraphData`。
2. 不改写现有 `RetrieveGraphRepository.AddGraph` 的行为。
3. 不把来源、审核、置信度等全部写入 Neo4j。
4. 不让 Neo4j 成为唯一数据源。
5. 不改现有聊天 GraphRAG 检索语义。
6. 不新增前端图谱编辑 UI。

## 4. 当前图谱模型

当前轻量图谱结构如下：

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

type GraphData struct {
    Text     string           `json:"text,omitempty"`
    Node     []*GraphNode     `json:"node,omitempty"`
    Relation []*GraphRelation `json:"relation,omitempty"`
}
```

现有 Neo4j 写入模型大致是：

```text
节点唯一键：name + kg
节点标签：ENTITY<knowledge_base_id>、ENTITY<knowledge_id>
节点属性：name、kg、attributes、chunks
边类型：GraphRelation.Type
```

这个模型应该继续保留，避免影响上游原有 GraphRAG 行为。新增图谱同步能力应使用新的数据库模型和新的 Neo4j 投影 label。

## 5. 推荐架构

```text
GraphSyncHandler
  -> GraphSyncService
  -> GraphEntityRepository / GraphRelationRepository
  -> graph_entities / graph_relations
  -> GraphProjectionService
  -> Neo4jGraphProjectionRepository
  -> Neo4j (:GraphEntity)-[:RELATION]->(:GraphEntity)
```

写入侧只负责落库：

```text
导入实体 / 关系
  -> 校验
  -> batch upsert 数据库
  -> sync_status = pending
```

投影侧负责同步 Neo4j：

```text
读取 pending / failed 行
  -> 先同步实体
  -> 再同步关系
  -> 成功标记 synced
  -> 失败标记 failed + sync_error
```

## 6. 新增文件规划

优先新增文件，降低与上游冲突概率。

建议新增：

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

不可避免的小改动：

```text
internal/container/container.go
internal/router/router.go
```

可选改动，取决于是否在首个 PR 接入任务队列：

```text
internal/router/task.go
internal/router/sync_task.go
```

## 7. 数据库模型

### 7.1 实体表

表名：

```text
graph_entities
```

建议字段：

| 字段 | 说明 |
|---|---|
| `id` | WeKnora 内部主键 |
| `tenant_id` | 租户 ID |
| `knowledge_base_id` | 知识库 ID |
| `source_entity_id` | 外部/上游实体 ID |
| `entity_type` | 实体类型 |
| `entity_name` | 实体展示名 |
| `entity_data` | 实体完整属性 JSON |
| `source_doc_uuid` | 来源文档 ID |
| `source_site` | 来源站点 |
| `source_text` | 支撑该实体的原文证据 |
| `confidence_score` | 置信度，范围 0 到 1 |
| `confidence_reason` | 置信度说明 |
| `review_status` | 审核状态 |
| `sync_status` | Neo4j 同步状态 |
| `neo4j_node_id` | Neo4j 节点 ID 或投影 ID |
| `is_deleted` | 软删除标记 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `synced_at` | 最近同步时间 |
| `sync_error` | 最近同步错误 |

唯一键：

```text
tenant_id + knowledge_base_id + source_entity_id
```

### 7.2 关系表

表名：

```text
graph_relations
```

建议字段：

| 字段 | 说明 |
|---|---|
| `id` | WeKnora 内部主键 |
| `tenant_id` | 租户 ID |
| `knowledge_base_id` | 知识库 ID |
| `source_relation_id` | 外部/上游关系 ID |
| `from_entity_id` | 起点实体的 `source_entity_id` |
| `to_entity_id` | 终点实体的 `source_entity_id` |
| `relation_type` | 关系类型 |
| `relation_props` | 关系属性 JSON |
| `source_doc_uuid` | 来源文档 ID |
| `source_site` | 来源站点 |
| `source_text` | 支撑该关系的原文证据 |
| `confidence_score` | 置信度，范围 0 到 1 |
| `confidence_reason` | 置信度说明 |
| `review_status` | 审核状态 |
| `sync_status` | Neo4j 同步状态 |
| `neo4j_relation_id` | Neo4j 关系 ID 或投影 ID |
| `is_deleted` | 软删除标记 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `synced_at` | 最近同步时间 |
| `sync_error` | 最近同步错误 |

唯一键：

```text
tenant_id + knowledge_base_id + source_relation_id
```

关系端点约束：

```text
from_entity_id / to_entity_id 必须引用同一 tenant_id + knowledge_base_id 下的 source_entity_id
```

## 8. 同步状态

实体和关系使用同一套状态：

```text
pending
synced
failed
deleted
skipped
```

状态流转：

```text
新增或更新数据库行
  -> sync_status = pending

投影 Neo4j 成功
  -> sync_status = synced

投影 Neo4j 失败
  -> sync_status = failed
  -> sync_error = 错误信息

软删除数据库行
  -> is_deleted = true
  -> sync_status = pending

Neo4j 删除成功
  -> sync_status = deleted
```

## 9. HTTP 接口设计

### 9.1 批量 upsert 实体

```text
POST /api/v1/knowledge-bases/:id/graph/entities:batch-upsert
```

请求示例：

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

响应示例：

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

### 9.2 批量 upsert 关系

```text
POST /api/v1/knowledge-bases/:id/graph/relations:batch-upsert
```

请求示例：

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

响应结构与实体接口一致。

### 9.3 兼容旧的手动导入接口

如果保留当前实验接口：

```text
POST /api/v1/knowledge-bases/:id/graph/import
```

它应该作为兼容 wrapper 存在：

```text
legacy nodes / relations
  -> 转换为 GraphSyncService 的实体 / 关系 DTO
  -> 写入 graph_entities / graph_relations
  -> 不直接写 Neo4j
```

推荐转换规则：

| 旧字段 | 新字段 |
|---|---|
| `GraphNode.Name` | `source_entity_id`、`entity_name` |
| `GraphNode.Attributes` | `entity_data.attributes` |
| `GraphRelation.Node1` | `from_entity_id` |
| `GraphRelation.Node2` | `to_entity_id` |
| `GraphRelation.Type` | `relation_type` |

当旧请求没有显式 `source_relation_id` 时，可用确定性 hash：

```text
hash(knowledge_base_id + node1 + relation_type + node2)
```

## 10. 权限设计

接口是知识库级写操作，建议沿用敏感 KB mutation 的权限矩阵：

```go
g.OwnedKBOrAdmin()
g.KBAccessWrite("id")
```

含义：

1. KB 创建者或 Admin+ 才能写入图谱数据。
2. 共享 KB 场景下仍通过 `KBAccessWrite` 解析有效租户上下文。
3. 不在 handler 内重复实现租户隔离和权限判断。

## 11. 类型与关系校验

新增：

```text
internal/application/service/graph_schema.go
```

放置实体类型、关系类型、关系方向约束。

实体类型白名单：

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

关系类型白名单：

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

校验规则：

1. `source_entity_id`、`entity_type`、`entity_name` 必填。
2. `source_relation_id`、`from_entity_id`、`to_entity_id`、`relation_type` 必填。
3. 实体类型必须在白名单中。
4. 关系类型必须在白名单中。
5. 关系的起点和终点实体必须存在于同一知识库。
6. 关系方向必须符合 `GRAPH_SYNC_DESIGN.md` 中的方向矩阵。
7. `confidence_score` 如存在，必须在 0 到 1 之间。
8. 拼接 Neo4j Cypher 前必须先校验 `relation_type`，禁止外部输入直接进入 Cypher。

## 12. Migration 方案

建议新增 versioned 和 sqlite 两套 migration：

```text
migrations/versioned/0000xx_graph_sync.up.sql
migrations/versioned/0000xx_graph_sync.down.sql
migrations/sqlite/0000xx_graph_sync.up.sql
migrations/sqlite/0000xx_graph_sync.down.sql
```

Postgres / ParadeDB 使用 `JSONB`，SQLite 用 `TEXT` 存 JSON。

推荐索引：

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

## 13. Repository 设计

实体 repository：

```go
type GraphEntityRepository interface {
    BatchUpsert(ctx context.Context, rows []*types.GraphEntity) error
    ListForProjection(ctx context.Context, limit int) ([]*types.GraphEntity, error)
    MarkSynced(ctx context.Context, id string, neo4jNodeID string, syncedAt time.Time) error
    MarkFailed(ctx context.Context, id string, errMsg string) error
    MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error
    FindBySourceIDs(ctx context.Context, tenantID uint64, kbID string, sourceIDs []string) ([]*types.GraphEntity, error)
}
```

关系 repository：

```go
type GraphRelationRepository interface {
    BatchUpsert(ctx context.Context, rows []*types.GraphRelationRecord) error
    ListForProjection(ctx context.Context, limit int) ([]*types.GraphRelationRecord, error)
    MarkSynced(ctx context.Context, id string, neo4jRelationID string, syncedAt time.Time) error
    MarkFailed(ctx context.Context, id string, errMsg string) error
    MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error
}
```

`BatchUpsert` 使用 GORM `OnConflict` 实现幂等写入。只要来源 ID 不变，重复导入不会创建重复实体或关系。

## 14. Service 设计

### 14.1 GraphSyncService

负责导入和数据库写入：

1. 读取 KB 并确认租户上下文。
2. 校验实体类型、关系类型和关系方向。
3. 将请求 DTO 转换为数据库 row。
4. 批量 upsert。
5. 对新增/更新/软删除行设置 `sync_status = pending`。
6. 可选：导入后触发投影任务。

### 14.2 GraphProjectionService

负责数据库到 Neo4j 投影：

1. 读取 `pending` / `failed` 实体。
2. 对未删除实体执行 Neo4j `MERGE`。
3. 对已删除实体执行 Neo4j 删除。
4. 更新实体同步状态。
5. 读取 `pending` / `failed` 关系。
6. 确认关系两端节点已经存在。
7. 对未删除关系执行 Neo4j `MERGE`。
8. 对已删除关系执行 Neo4j 删除。
9. 更新关系同步状态。

同步顺序必须是：

```text
先实体，后关系
```

## 15. Neo4j 投影设计

新增投影不复用现有 `ENTITY<kb_id>:ENTITY<knowledge_id>` label，建议使用统一 label：

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

节点 MERGE：

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

关系：

```cypher
(:GraphEntity {id})-[:TARGETS {
  id,
  source_relation_id,
  properties
}]->(:GraphEntity {id})
```

关系 MERGE：

```cypher
MATCH (from:GraphEntity {id: $from_id})
MATCH (to:GraphEntity {id: $to_id})
MERGE (from)-[r:TARGETS {id: $id}]->(to)
SET
  r.source_relation_id = $source_relation_id,
  r.properties = $properties
```

注意：Cypher 不能参数化关系类型，因此 `TARGETS` 这类 relation type 必须先经过白名单校验，再由代码拼接。

## 16. 任务执行方案

MVP 可以先做手动触发：

```text
导入接口
  -> 写 DB pending

手动触发投影接口
  -> GraphProjectionService.RunOnce(limit)
```

后续再接入异步任务：

```text
导入接口
  -> 写 DB pending
  -> enqueue graph projection task
  -> Asynq / SyncTaskExecutor 执行投影
```

如果首个 PR 要控制范围，建议先实现 service 和手动触发，Asynq 自动投影放到第二个 PR。

## 17. 兼容策略

1. 不删除旧 `GraphNode` / `GraphRelation`。
2. 不改变旧 `RetrieveGraphRepository.AddGraph`。
3. 新增 DB-backed 图谱同步路径。
4. 新增 Neo4j projection label，和旧 GraphRAG 图谱隔离。
5. 旧 `/graph/import` 如果保留，只作为 compatibility wrapper。
6. 后续如需让聊天 GraphRAG 使用新投影，应单独开 PR。

## 18. PR 拆分建议

### PR 1：数据库图谱导入

内容：

1. 新增类型、接口和 migration。
2. 新增实体/关系 repository。
3. 新增 `GraphSyncService`。
4. 新增批量导入 handler 和路由。
5. 旧 `/graph/import` 改为落库，不直接写 Neo4j。
6. 补充 repository/service 测试。

### PR 2：Neo4j 投影

内容：

1. 新增 Neo4j projection repository。
2. 新增 `GraphProjectionService`。
3. 新增手动投影接口或任务。
4. 实现同步状态流转。
5. 补充投影测试。

### PR 3：运维与产品化能力

内容：

1. 同步失败列表。
2. 手动重试。
3. 按 `source_doc_uuid` 查询实体/关系。
4. 按 `source_doc_uuid` 批量撤回。
5. Neo4j 全量重建。
6. 前端入口。

## 19. 测试计划

### 19.1 单元测试

1. 实体缺少必填字段应失败。
2. 未知实体类型应失败。
3. 未知关系类型应失败。
4. 关系端点不存在应失败。
5. 关系方向不符合约束应失败。
6. `confidence_score` 超出 0 到 1 应失败。
7. 重复导入相同 `source_entity_id` 应更新而不是插入重复数据。
8. 更新已有行应把 `sync_status` 重置为 `pending`。
9. 软删除应设置 `is_deleted = true` 和 `sync_status = pending`。

### 19.2 Repository 测试

1. SQLite 下 batch upsert 正常。
2. `ListForProjection` 只返回 `pending` / `failed`。
3. `ListForProjection` 按 `updated_at` 升序返回。
4. `MarkSynced` 正确写入 `synced_at` 和 `neo4j_*_id`。
5. `MarkFailed` 正确写入 `sync_error`。
6. `MarkDeleted` 正确写入 `deleted`。

### 19.3 Projection 测试

1. 实体投影生成正确的 `MERGE (:GraphEntity {id})`。
2. 删除实体时 Neo4j 删除节点并标记 `deleted`。
3. 关系投影前会先校验 relation type。
4. 关系投影失败时数据库标记 `failed`。
5. 重复执行投影结果保持一致。

### 19.4 手动验证

1. 启动无 Neo4j 的本地环境，导入实体/关系，确认只写 DB。
2. 检查导入行 `sync_status = pending`。
3. 启动 Neo4j 后触发投影。
4. 检查 Neo4j 中只有最小节点和边。
5. 更新实体后再次投影，确认幂等。
6. 软删除实体/关系后投影，确认 Neo4j 删除对应结构。

## 20. 待确认问题

1. `source_entity_id` 是全局唯一，还是仅在单个知识库内唯一？当前方案按单知识库唯一设计。
2. 是否只有 `review_status = approved` 的数据才能投影到 Neo4j？建议后续开启，MVP 可先不过滤。
3. 关系导入时，如果端点实体不存在，是拒绝还是自动创建占位实体？建议拒绝。
4. 首个 PR 是否接入 Asynq 自动投影？建议先手动触发，降低 review 范围。
5. Neo4j 是否需要 `:Drug`、`:Company` 这类动态 label？建议首版只用 `:GraphEntity`，需要性能优化时再加。

## 21. 给 upstream reviewer 的说明

这套方案刻意不重写现有 GraphRAG 图谱模型，而是旁路新增 DB-backed 图谱同步能力。这样做的好处是：

1. 现有文档抽取、聊天检索和 Neo4j 写入行为不受影响。
2. 新能力有完整数据库事实源，可追溯、可审核、可重试、可重建。
3. Neo4j 只保存最小投影，降低 schema 复杂度。
4. 大多数实现放在新增文件中，现有热点文件只做少量注册。
5. 后续可以逐步把 GraphRAG 查询迁移到新投影，而不是一次性大改。
