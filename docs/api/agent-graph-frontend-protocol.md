# Agent 流式知识图谱 — 前端展示协议

本文约定 WeKnora 与前端之间的**展示契约**：实时 SSE + 查询 API。  
后端已实现；前端按本文对接即可，无需关心 MCP / BioDSA 内部细节。

---

## 0. 一句话

| 场景 | 用什么 |
|------|--------|
| 回答进行中，实时画图 | Agent Chat SSE：`response_type = "agent_graph"` |
| 刷新 / 断线重连 / 回答结束后再打开 | `GET .../graph` 增量拉取 |
| 最终对齐 | 收到 `RunComplete`（或回答结束）后 **再 GET 一次全量**（`after_seq=0`） |

本地状态建议维护：

```ts
type GraphViewState = {
  nodes: Map<string, GraphNodeView>;   // key = entity_name
  edges: Map<string, GraphEdgeView>;   // key = `${source}\0${target}\0${relation}`
  run: GraphRunView | null;
  lastSeq: number;                     // 仅用于 GET 游标（见 §3）
};
```

---

## 1. 鉴权

与现有会话接口相同：`Authorization: Bearer <token>` 或 API Key。  
需要能读该 session 的 Viewer 权限。

---

## 2. 实时通道：SSE `agent_graph`

### 2.1 出现位置

挂在现有 Agent 对话 SSE 流上（`POST /api/v1/agent-chat/:session_id` 及 continue-stream 回放），与 `thinking` / `tool_call` 等并列。

### 2.2 帧结构

外层与其它 SSE 事件一致：

```ts
interface StreamResponse {
  id: string;
  response_type: "agent_graph";
  content: "";           // 恒为空，不看 content
  done: false;           // 图谱帧不做 done 语义；结束看 graph_event=RunComplete 或整轮 complete
  data: AgentGraphSSEData;
  session_id?: string;
  assistant_message_id?: string;
}
```

```ts
interface AgentGraphSSEData {
  /** 当前 tool_call 内的单调序号，从 1 开始；每次工具调用各自从 1 重计 */
  seq: number;
  /** 见 §4 */
  graph_event: GraphEventType;
  tool_call_id: string;
  /** 该事件的业务字段；RunComplete 为摘要，见 §2.4 */
  payload: Record<string, unknown>;
}
```

示例：

```json
{
  "response_type": "agent_graph",
  "content": "",
  "done": false,
  "data": {
    "seq": 12,
    "graph_event": "EntityConfirmed",
    "tool_call_id": "chatcmpl-tool-b5f79860a48f5ba8",
    "payload": {
      "event_id": "...",
      "timestamp": 1786608000.123,
      "event_type": "EntityConfirmed",
      "entity_name": "SNCA",
      "entity_type": "gene",
      "observations": ["Alpha-synuclein ..."]
    }
  }
}
```

### 2.3 前端如何应用（增量 patch）

| `graph_event` | 动作 |
|---------------|------|
| `EntitySearching` | upsert 节点：`status=searching`，可闪烁/虚线 |
| `EntityConfirmed` | upsert 节点：`status=confirmed`，写入 `observations` |
| `RelationFound` | upsert 边 |
| `PhaseChange` | 更新 `run.phase` / 副标题（`broad_search` \| `deep_dive`） |
| `Progress` | 更新进度条：`step` / `entities_found` / `relations_found` |
| `LiteratureSearching` | 可选：仅进度文案，不改图 |
| `RunComplete` | 标记本 tool_call 图构建结束；**不要依赖 SSE 里的完整实体列表**（见下） |

节点 upsert key：`entity_name`（同一 message 内唯一）。  
边 upsert key：`source_entity + target_entity + relation_type`。

同一轮回答里可能有多次 DeepEvidence 调用（不同 `tool_call_id`）。图是 **message 级合并视图**：后写覆盖同名节点/边即可。

### 2.4 `RunComplete` 在 SSE 里是摘要

为避免大包，SSE 的 `RunComplete.payload` **只有**：

```ts
{
  event_id?: string;
  timestamp?: number;
  event_type: "RunComplete";
  total_steps?: number;
  duration_seconds?: number;
  final_response_preview?: string;
  entity_count?: number;      // 不是 entities[]
  relation_count?: number;    // 不是 relations[]
}
```

完整节点/边以查询 API 为准。推荐：

```ts
onRunComplete() {
  await refreshGraph({ after_seq: 0 }); // 全量对齐
}
```

### 2.5 注意：`data.seq` ≠ GET 的 `after_seq`

| 字段 | 含义 |
|------|------|
| SSE `data.seq` | **单次 tool_call** 内序号 |
| GET `after_seq` / 响应 `last_seq` | **整条 assistant message** 上的 `msg_seq` |

因此：**不要**把 SSE 的 `seq` 传给 GET 的 `after_seq`。  
实时阶段只 patch 本地图；游标只在调用 GET 时使用响应里的 `last_seq`。

---

## 3. 查询通道：GET snapshot

### 3.1 请求

```
GET /api/v1/sessions/{session_id}/messages/{message_id}/graph
    ?after_seq={number}
    &include=run,nodes,edges
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `after_seq` | `0` | 只返回 `msg_seq`（节点/边用 `last_msg_seq`）**大于**该值的增量 |
| `include` | `run,nodes,edges` | 逗号分隔；`events` 需显式加（量大，默认不要） |

路径占位必须是 session=`id`、message=`message_id`（与 suggestions 路由一致）。

### 3.2 响应

```ts
interface GraphApiResponse {
  success: true;
  data: {
    run: GraphRunView | null;
    nodes: GraphNodeView[];
    edges: GraphEdgeView[];
    events: GraphEventRow[];   // 仅 include 含 events 时有意义
    last_seq: number;          // 下次 after_seq 用这个
  };
}
```

```ts
type GraphRunStatus = "running" | "completed" | "failed";

interface GraphRunView {
  id: string;
  session_id: string;
  message_id: string;
  stream_key: string;
  tool_call_id: string;
  status: GraphRunStatus;
  phase: string;
  step: number;
  entity_count: number;
  relation_count: number;
  last_seq: number;       // stream 内 seq
  last_msg_seq: number;   // message 级游标
  duration_seconds: number | null;
  started_at: string;
  completed_at: string | null;
}

interface GraphNodeView {
  entity_name: string;
  /** 小写规范词表，见 §4.1；空字符串按 unknown 上色 */
  entity_type: string;
  status: "searching" | "confirmed";
  source_kb: string;
  observations: string[] | null; // 可能为 [] 或 null，按空数组处理
  first_msg_seq: number;
  last_msg_seq: number;
}

interface GraphEdgeView {
  source_entity: string;
  target_entity: string;
  relation_type: string;
  first_msg_seq: number;
  last_msg_seq: number;
}
```

运行中与运行后**同一接口、同一结构**，只看 `run.status`。

### 3.3 推荐调用时机

1. **进入已有消息**（刷新、从历史点开）：`after_seq=0` 拉全量。  
2. **SSE 存活时**：可只靠 SSE patch；弱网可每 N 秒 `after_seq=last_seq` 补洞。  
3. **`RunComplete` 或整轮 `complete`**：`after_seq=0` 再拉一次做最终对齐。  
4. **轮询补洞**：

```ts
let cursor = 0;
async function poll() {
  const snap = await getGraph({ after_seq: cursor, include: "run,nodes,edges" });
  applySnapshotDelta(snap);
  cursor = snap.last_seq;
  if (snap.run?.status === "completed" || snap.run?.status === "failed") stop();
}
```

---

## 4. `graph_event` / payload 字段

公共字段（在 `payload` 内）：`event_id`、`timestamp`、`event_type`。

| graph_event | payload 主要字段 | 图上含义 |
|-------------|------------------|----------|
| `EntitySearching` | `entity_name`, `entity_type`, `source_kb`, `search_term` | 节点出现，searching |
| `LiteratureSearching` | `query`, `source_kb` | 文案/进度 |
| `EntityConfirmed` | `entity_name`, `entity_type`, `observations[]` | 节点确认 |
| `RelationFound` | `source_entity`, `target_entity`, `relation_type` | 加边 |
| `PhaseChange` | `phase`, `search_target`, `knowledge_bases[]` | 阶段切换 |
| `Progress` | `step`, `total_steps_estimate`, `entities_found`, `relations_found`, `current_phase` | 进度 |
| `RunComplete` | SSE 见 §2.4；DB/events 另有摘要字段 | 收尾 |

---

## 4.1 实体类型（上色用，必读）

后端落库 / GET `nodes[].entity_type`、SSE `EntityConfirmed.payload.entity_type` 使用**小写规范词表**（与 BioDSA `ENTITY_TYPE_NORMALIZE` 对齐）。

### 规范值（frontend 按此配色）

| `entity_type` | 含义 | 建议色 token（可自定 hex） |
|---------------|------|---------------------------|
| `gene` | 基因 / 蛋白 | `entity.gene` |
| `variant` | 变异 | `entity.variant` |
| `drug` | 药物 | `entity.drug` |
| `compound` | 化合物（含 CHEMICAL） | `entity.compound` |
| `disease` | 疾病 / 表型 | `entity.disease` |
| `pathway` | 通路 / gene set | `entity.pathway` |
| `target` | 靶点 | `entity.target` |
| `literature` | 文献（如 PMID:…） | `entity.literature` |
| `finding` | 研究结论 / 发现 | `entity.finding` |
| `cell_line` | 细胞系 | `entity.cell_line` |
| `tissue` | 组织 | `entity.tissue` |
| `""` / 其它 | 未知 | `entity.unknown` |

```ts
type EntityType =
  | "gene" | "variant" | "drug" | "compound" | "disease"
  | "pathway" | "target" | "literature" | "finding"
  | "cell_line" | "tissue" | "unknown";

const ENTITY_COLOR: Record<EntityType, string> = {
  gene: "#2F6FED",
  variant: "#7C5CFC",
  drug: "#0F9F6E",
  compound: "#0D9488",
  disease: "#E11D48",
  pathway: "#D97706",
  target: "#2563EB",
  literature: "#64748B",
  finding: "#DB2777",
  cell_line: "#0891B2",
  tissue: "#CA8A04",
  unknown: "#94A3B8",
};

function colorForEntityType(t: string | null | undefined): string {
  const key = (t || "").toLowerCase() as EntityType;
  return ENTITY_COLOR[key] ?? ENTITY_COLOR.unknown;
}
```

### 别名归一（后端已做；前端若收到原始大写可同样处理）

| 原始 | → 规范 |
|------|--------|
| `GENE` / `PROTEIN` | `gene` |
| `DRUG` | `drug` |
| `CHEMICAL` | `compound` |
| `DISEASE` / `PHENOTYPE` | `disease` |
| `PATHWAY` / `GENE_SET` | `pathway` |
| `PAPER` | `literature` |
| `FINDING` | `finding` |
| `CELL_LINE` / `TISSUE` / `VARIANT` / `TARGET` | 对应小写 |

**上色规则**：以节点最终的 `entity_type` 为准（`EntityConfirmed` 或 GET `nodes`）。`EntitySearching` 阶段类型一般已是小写词表，但未确认前可用浅色/虚线，确认后再套实色。

---

## 4.2 关系类型（边标签）

`edges[].relation_type` / SSE `RelationFound.payload.relation_type` 为 **大写蛇形**字符串，开放集合（模型可发明），前端应：

1. 已知类型用固定中文/样式；  
2. 未知类型原样展示（或 `prettify(s)`：下划转空格）。

### 常见值（线上已出现 + BioDSA 常用）

| `relation_type` | 建议展示 |
|-----------------|----------|
| `ASSOCIATED_WITH` | 相关 |
| `TREATS` | 治疗 |
| `INHIBITS` | 抑制 |
| `ACTIVATES` | 激活 |
| `TARGETS` | 靶向 |
| `BINDS` | 结合 |
| `SUPPORTS` | 支持 |
| `MEMBER_OF_PATHWAY` | 属于通路 |

边颜色可不按类型区分；若需要，用低饱和灰边 + 文字标签即可。

---

## 5. UI 建议（非强制）

1. **一块画布**：节点 + 边；searching 用虚边框/脉冲，confirmed 实线。  
2. **进度条**：绑 `Progress` / `run.step` + `entity_count`/`relation_count`。  
3. **阶段标签**：`PhaseChange.phase` →「广搜 / 深挖」。  
4. **不把图谱塞进 tool_result 卡片**；与回答区并列或右侧抽屉，按 `assistant_message_id` 绑定。  
5. **多 tool_call**：可用 `tool_call_id` 在时间轴上分段，但图数据 message 级合并。

---

## 6. 最小接入伪代码

```ts
// 实时
sse.on("agent_graph", (ev) => {
  const { graph_event, payload, tool_call_id } = ev.data;
  switch (graph_event) {
    case "EntitySearching":
      upsertNode(payload.entity_name, { type: payload.entity_type, status: "searching", source_kb: payload.source_kb });
      break;
    case "EntityConfirmed":
      upsertNode(payload.entity_name, { type: payload.entity_type, status: "confirmed", observations: payload.observations ?? [] });
      break;
    case "RelationFound":
      upsertEdge(payload.source_entity, payload.target_entity, payload.relation_type);
      break;
    case "PhaseChange":
    case "Progress":
      patchRun(payload);
      break;
    case "RunComplete":
      void reloadFullGraph(sessionId, messageId); // GET after_seq=0
      break;
  }
});

// 刷新 / 历史
async function reloadFullGraph(sessionId: string, messageId: string) {
  const res = await fetch(`/api/v1/sessions/${sessionId}/messages/${messageId}/graph?after_seq=0`);
  const { data } = await res.json();
  replaceGraph(data.nodes, data.edges, data.run);
  lastSeq = data.last_seq;
}
```

---

## 7. 自检清单

- [ ] 能收到 `response_type=agent_graph`  
- [ ] Searching → Confirmed 节点状态会更新  
- [ ] RelationFound 出边  
- [ ] 刷新后 GET 能还原节点/边  
- [ ] `RunComplete` 后全量 GET，与最终库表一致  
- [ ] 未把 SSE `seq` 误当作 GET `after_seq`
