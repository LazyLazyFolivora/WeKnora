# 图谱数据同步与 Neo4j 投影开发说明

## 1. 目标

当前项目作为数据生产方，负责：

- 文档解析
- 实体抽取
- 关系抽取
- 数据清洗
- 实体消歧
- 质量控制
- 来源追溯

上游图谱项目作为数据消费方，负责：

- 接收清洗后的实体和关系
- 写入业务数据库
- 从业务数据库同步到 Neo4j
- 提供图查询、图展示、图分析能力

核心原则：

> 数据库是事实源，Neo4j 是图谱投影。所有增删改操作只操作数据库，不直接操作 Neo4j。

---

## 2. 推荐架构

```text
当前项目
  ↓
同步实体 / 关系数据
  ↓
上游业务数据库
  ↓
DB → Neo4j 同步任务
  ↓
Neo4j 图数据库
```

### 2.1 设计原则

1. 业务数据库保存完整数据：实体、关系、来源信息、置信度、审核状态、同步状态、删除状态。
2. Neo4j 只保存图谱查询所需的最小数据：节点、边及其核心属性。
3. 禁止业务直接写 Neo4j：新增、修改、删除都先写数据库。
4. Neo4j 可随时从数据库重建，不作为权威数据源。

---

## 3. 数据分层

上游建议设计两份数据结构。

### 3.1 数据库同步数据

数据库接收完整数据，用于追溯、审核、重建、同步控制。

包括：

```text
图谱结构字段
+
来源字段
+
质量字段
+
审核字段
+
同步状态字段
```

### 3.2 Neo4j 导入数据

Neo4j 只接收干净的图谱结构。

包括：

```text
节点：id, type, name, properties
边：id, from, to, type, properties
```

---

## 4. 实体数据设计

### 4.1 数据库实体表建议字段

表名示例：

```text
graph_entities
```

| 字段 | 类型 | 必须 | 说明 |
|---|---|---|---|
| `id` | string / bigint | 是 | 上游系统实体主键 |
| `source_entity_id` | string / bigint | 是 | 当前项目中的实体 ID |
| `entity_type` | string | 是 | 实体类型 |
| `entity_name` | string | 是 | 实体名称 |
| `entity_data` | json | 是 | 实体详细属性 |
| `source_doc_uuid` | string | 建议 | 来源文档 UUID |
| `source_site` | string | 可选 | 来源站点 |
| `source_text` | text | 建议 | 支持该实体的原文证据 |
| `confidence_score` | float | 建议 | 置信度，0 到 1 |
| `confidence_reason` | text | 可选 | 置信度说明 |
| `review_status` | string | 建议 | 审核状态 |
| `sync_status` | string | 是 | Neo4j 同步状态 |
| `neo4j_node_id` | string | 可选 | Neo4j 节点 ID |
| `is_deleted` | boolean | 是 | 软删除标记 |
| `created_at` | datetime | 是 | 创建时间 |
| `updated_at` | datetime | 是 | 更新时间 |
| `synced_at` | datetime | 可选 | 最近一次同步时间 |
| `sync_error` | text | 可选 | 同步失败原因 |

### 4.2 实体最小数据库数据

```json
{
  "source_entity_id": "1",
  "entity_type": "Drug",
  "entity_name": "SYSA1801",
  "entity_data": {
    "name": "SYSA1801",
    "modality": "ADC"
  },
  "source_doc_uuid": "doc-001",
  "source_text": "SYSA1801 是一种靶向 CLDN18.2 的 ADC 药物。",
  "confidence_score": 0.92,
  "review_status": "approved",
  "sync_status": "pending",
  "is_deleted": false
}
```

### 4.3 Neo4j 节点数据

Neo4j 节点只需要最小图谱字段。

```json
{
  "id": "entity:1",
  "type": "Drug",
  "name": "SYSA1801",
  "properties": {
    "name": "SYSA1801",
    "modality": "ADC"
  }
}
```

### 4.4 Neo4j 节点字段

| 字段 | 必须 | 说明 |
|---|---|---|
| `id` | 是 | 节点唯一 ID |
| `type` | 是 | 节点类型 |
| `name` | 是 | 展示名称 |
| `properties` | 是 | 节点属性 JSON |

建议 Neo4j 节点中保留一个回查字段：

```json
{
  "source_entity_id": "1"
}
```

这样可以从 Neo4j 查询结果回查业务数据库。

---

## 5. 关系数据设计

### 5.1 数据库关系表建议字段

表名示例：

```text
graph_relations
```

| 字段 | 类型 | 必须 | 说明 |
|---|---|---|---|
| `id` | string / bigint | 是 | 上游系统关系主键 |
| `source_relation_id` | string / bigint | 是 | 当前项目中的关系 ID |
| `from_entity_id` | string / bigint | 是 | 起点实体 ID |
| `to_entity_id` | string / bigint | 是 | 终点实体 ID |
| `relation_type` | string | 是 | 关系类型 |
| `relation_props` | json | 是 | 关系属性 |
| `source_doc_uuid` | string | 建议 | 来源文档 UUID |
| `source_site` | string | 可选 | 来源站点 |
| `source_text` | text | 建议 | 支持该关系的原文证据 |
| `confidence_score` | float | 建议 | 置信度，0 到 1 |
| `confidence_reason` | text | 可选 | 置信度说明 |
| `review_status` | string | 建议 | 审核状态 |
| `sync_status` | string | 是 | Neo4j 同步状态 |
| `neo4j_relation_id` | string | 可选 | Neo4j 关系 ID |
| `is_deleted` | boolean | 是 | 软删除标记 |
| `created_at` | datetime | 是 | 创建时间 |
| `updated_at` | datetime | 是 | 更新时间 |
| `synced_at` | datetime | 可选 | 最近一次同步时间 |
| `sync_error` | text | 可选 | 同步失败原因 |

### 5.2 关系最小数据库数据

```json
{
  "source_relation_id": "1001",
  "from_entity_id": "1",
  "to_entity_id": "2",
  "relation_type": "TARGETS",
  "relation_props": {
    "mechanism": "抑制剂"
  },
  "source_doc_uuid": "doc-001",
  "source_text": "SYSA1801 是一种靶向 CLDN18.2 的 ADC 药物。",
  "confidence_score": 0.91,
  "review_status": "approved",
  "sync_status": "pending",
  "is_deleted": false
}
```

### 5.3 Neo4j 关系数据

Neo4j 边只需要最小图谱字段。

```json
{
  "id": "relation:1001",
  "from": "entity:1",
  "to": "entity:2",
  "type": "TARGETS",
  "properties": {
    "mechanism": "抑制剂"
  }
}
```

### 5.4 Neo4j 边字段

| 字段 | 必须 | 说明 |
|---|---|---|
| `id` | 是 | 边唯一 ID |
| `from` | 是 | 起点节点 ID |
| `to` | 是 | 终点节点 ID |
| `type` | 是 | 边类型 |
| `properties` | 是 | 边属性 JSON |

建议 Neo4j 关系中保留一个回查字段：

```json
{
  "source_relation_id": "1001"
}
```

---

## 6. 是否需要同步元数据

### 6.1 不建议写入 Neo4j 的字段

以下字段建议只保存在业务数据库，不写入 Neo4j：

```text
source_text
confidence_score
confidence_reason
source_doc_uuid
source_site
review_status
sync_status
sync_error
created_at
updated_at
```

原因：

1. Neo4j 保持图结构干净。
2. 减少图查询复杂度。
3. 避免改动 fork 项目的核心图模型。
4. 降低未来同步上游代码时的冲突。
5. 元数据可以通过数据库回查。

### 6.2 建议保存在数据库的元数据

这些字段虽然不是构图必需，但建议数据库保留：

| 字段 | 建议程度 | 原因 |
|---|---|---|
| `source_doc_uuid` | 强烈建议 | 后续可以按文档回滚、重爬、更新 |
| `source_text` | 建议 | 人工审核、证据展示、问题排查 |
| `confidence_score` | 建议 | 过滤低质量数据、自动审核 |
| `confidence_reason` | 可选 | 辅助解释置信度 |
| `review_status` | 建议 | 控制哪些数据进入 Neo4j |
| `sync_status` | 必须 | 控制 DB 到 Neo4j 的投影状态 |

---

## 7. 同步状态设计

实体和关系都建议使用同一套同步状态。

```text
pending
synced
failed
deleted
skipped
```

| 状态 | 说明 |
|---|---|
| `pending` | 待同步到 Neo4j |
| `synced` | 已同步 |
| `failed` | 同步失败 |
| `deleted` | 已从 Neo4j 删除 |
| `skipped` | 可选，跳过同步 |

### 7.1 新增状态流转

```text
insert database
  ↓
sync_status = pending
  ↓
sync to Neo4j
  ↓
sync_status = synced
```

### 7.2 更新状态流转

```text
update database
  ↓
sync_status = pending
  ↓
merge to Neo4j
  ↓
sync_status = synced
```

### 7.3 删除状态流转

```text
set is_deleted = true
  ↓
sync_status = pending
  ↓
delete from Neo4j
  ↓
sync_status = deleted
```

### 7.4 失败状态流转

```text
sync failed
  ↓
sync_status = failed
  ↓
write sync_error
  ↓
retry later
```

---

## 8. 同步任务逻辑

### 8.1 同步实体

同步任务读取：

```sql
SELECT *
FROM graph_entities
WHERE sync_status IN ('pending', 'failed')
ORDER BY updated_at ASC
LIMIT 100;
```

处理逻辑：

```text
如果 is_deleted = false：
    MERGE Neo4j 节点
    更新 sync_status = synced

如果 is_deleted = true：
    DELETE Neo4j 节点
    更新 sync_status = deleted
```

### 8.2 同步关系

同步任务读取：

```sql
SELECT *
FROM graph_relations
WHERE sync_status IN ('pending', 'failed')
ORDER BY updated_at ASC
LIMIT 100;
```

处理逻辑：

```text
如果 is_deleted = false：
    确认 from_entity_id 和 to_entity_id 对应节点存在
    MERGE Neo4j 关系
    更新 sync_status = synced

如果 is_deleted = true：
    DELETE Neo4j 关系
    更新 sync_status = deleted
```

### 8.3 同步顺序

推荐顺序：

```text
1. 先同步实体
2. 再同步关系
```

原因：关系依赖实体节点存在。

---

## 9. 幂等要求

DB 到 Neo4j 的同步必须是幂等的。

同一条数据重复同步多次，结果应该一致。

### 9.1 实体幂等键

推荐：

```text
entity.id
```

或：

```text
source_entity_id
```

Neo4j 中可以使用：

```text
GraphEntity { id: "entity:1" }
```

### 9.2 关系幂等键

推荐：

```text
relation.id
```

或：

```text
source_relation_id
```

Neo4j 中可以在关系属性中保存：

```text
source_relation_id
```

---

## 10. 推荐 Neo4j 写入方式

### 10.1 节点 MERGE

示例逻辑：

```cypher
MERGE (n:GraphEntity {id: $id})
SET
  n.type = $type,
  n.name = $name,
  n.properties = $properties,
  n.source_entity_id = $source_entity_id
```

如果需要按类型打 label，可以额外加类型 label，但核心仍建议保留统一 label：

```text
GraphEntity
```

例如：

```text
(:GraphEntity:Drug)
(:GraphEntity:Company)
(:GraphEntity:Target)
```

### 10.2 关系 MERGE

由于 Cypher 不支持直接参数化关系类型，需要在代码层校验 `relation_type` 后拼接。

逻辑：

```cypher
MATCH (from:GraphEntity {id: $from_id})
MATCH (to:GraphEntity {id: $to_id})
MERGE (from)-[r:TARGETS {id: $id}]->(to)
SET
  r.properties = $properties,
  r.source_relation_id = $source_relation_id
```

注意：

```text
relation_type 必须先经过白名单校验，不能直接使用外部输入拼接 Cypher。
```

---

## 11. 当前支持的实体类型

上游建议预定义以下实体类型：

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

实体属性统一放在：

```text
entity_data
```

Neo4j 中对应：

```text
properties
```

---

## 12. 当前支持的关系类型

上游建议预定义以下关系类型：

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

---

## 13. 关系方向约束

| 关系类型 | 起点 | 终点 |
|---|---|---|
| `DEVELOPS` | `Company` | `Drug` |
| `DEVELOPS` | `DevelopmentProject` | `Drug` |
| `PARTICIPATES_IN` | `Company` | `DealEvent` |
| `INVESTED_IN` | `Company` | `Company` |
| `SUBSIDIARY_OF` | `Company` | `Company` |
| `SPONSORS` | `Company` | `ClinicalTrial` |
| `ISSUES` | `Company` | `Policy` |
| `APPROVES` | `Company` | `ApprovalEvent` |
| `PARTICIPATES_IN_PROJECT` | `Company` | `DevelopmentProject` |
| `TARGETS` | `Drug` | `Target` |
| `TARGETS` | `Compound` | `Target` |
| `TREATS` | `Drug` / `TCMFormula` | `Indication` |
| `IN_TRIAL` | `Drug` | `ClinicalTrial` |
| `HAS_APPROVAL` | `Drug` | `ApprovalEvent` |
| `HAS_ITEM` | `DealEvent` | `DealItem` |
| `INVOLVES_DRUG` | `DealItem` | `Drug` |
| `EVALUATES` | `ClinicalTrial` | `TrialIndication` |
| `FOR_INDICATION` | `TrialIndication` | `Indication` |
| `IN_PATHWAY` | `Target` | `Pathway` |
| `ASSOCIATED_WITH` | `Target` | `Indication` |
| `HOMOLOG_OF` | `Target` | `Target` |
| `SUBTYPE_OF` | `Indication` | `Indication` |
| `AFFECTS` | `Policy` | `Company` / `Drug` / `Indication` / `Target` |
| `DEVELOPED_BY` | `TCMFormula` | `Company` |
| `CONTAINS` | `TCMFormula` | `Compound` |

---

## 14. 推荐同步接口格式

### 14.1 批量同步实体

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

### 14.2 批量同步关系

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

---

## 15. 上游开发建议

### 15.1 必须实现

1. 实体表。
2. 关系表。
3. 实体批量 upsert 接口。
4. 关系批量 upsert 接口。
5. DB 到 Neo4j 的同步任务。
6. 同步状态字段。
7. 同步失败重试机制。
8. 软删除机制。

### 15.2 建议实现

1. 按 `source_doc_uuid` 查询实体和关系。
2. 按 `source_doc_uuid` 批量撤回数据。
3. 同步失败列表。
4. 手动重试同步。
5. Neo4j 全量重建任务。
6. 关系类型白名单校验。
7. 实体类型白名单校验。

### 15.3 不建议实现

1. 业务代码直接写 Neo4j。
2. Neo4j 作为唯一数据源。
3. 把 `source_text` 等元数据全部塞进 Neo4j。
4. 在 Neo4j 中做人审状态流转。
5. 依赖 Neo4j 中的数据反向修复数据库。

---

## 16. 最小可交付版本

如果先做 MVP，上游只需要实现以下内容。

### 16.1 数据库实体

```json
{
  "source_entity_id": "1",
  "entity_type": "Drug",
  "entity_name": "SYSA1801",
  "entity_data": {},
  "sync_status": "pending",
  "is_deleted": false
}
```

### 16.2 数据库关系

```json
{
  "source_relation_id": "1001",
  "from_entity_id": "1",
  "to_entity_id": "2",
  "relation_type": "TARGETS",
  "relation_props": {},
  "sync_status": "pending",
  "is_deleted": false
}
```

### 16.3 Neo4j 节点

```json
{
  "id": "entity:1",
  "type": "Drug",
  "name": "SYSA1801",
  "properties": {}
}
```

### 16.4 Neo4j 关系

```json
{
  "id": "relation:1001",
  "from": "entity:1",
  "to": "entity:2",
  "type": "TARGETS",
  "properties": {}
}
```

---

## 17. 最终结论

上游项目建议采用：

```text
业务数据库 = 权威事实源
Neo4j = 查询投影
```

当前项目同步给上游时，应该同步完整实体和关系数据；上游写入数据库后，再由同步任务投影到 Neo4j。

Neo4j 中只保留最小图谱结构，不保存完整来源和审核元数据。

推荐边界：

```text
当前项目负责：抽取、清洗、审核、来源追溯、置信度
上游数据库负责：保存完整图谱数据和同步状态
Neo4j 负责：图查询、图展示、图分析
```

这样可以最大程度降低 fork 项目的代码冲突，同时保留数据追溯和 Neo4j 重建能力。
