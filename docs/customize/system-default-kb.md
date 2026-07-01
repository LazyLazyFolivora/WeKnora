# 系统默认知识库

## 需求

将某个知识库标记为"系统默认"，所有租户的所有用户的问答请求都会自动将该知识库纳入检索范围。用户无感知，不需要手动选择。

## 方案

不修改数据库表结构（便于 fork 与上游同步），使用已有的 `system_settings` 表存储默认知识库 ID 列表。

```
请求 body: {"knowledge_base_ids": ["用户选的KB"]}
     ↓ Gin 中间件拦截
修改后:   {"knowledge_base_ids": ["用户选的KB", "系统默认KB"]}
     ↓ 原 handler → 原 service → 无感知
```

## 数据存储

`system_settings` 表手动插入一条记录：

| key | value | value_type |
|---|---|---|
| `system_default_kb_ids` | `["kb-uuid-1", "kb-uuid-2"]` | `string_list` |

```sql
INSERT INTO system_settings (key, value, value_type, category)
VALUES ('system_default_kb_ids', '["<KB-ID>"]', 'string_list', 'knowledge_base');
```

## 文件清单

### 新增文件

| 文件 | 说明 |
|---|---|
| `internal/middleware/default_kb.go` | Gin 中间件，拦截 KnowledgeQA/AgentQA 请求体，注入系统默认 KB ID |

### 修改文件

| 文件 | 改动 | 说明 |
|---|---|---|
| `internal/router/router.go` | +3 行 | ① `RouterParams` 加 `DB *gorm.DB` ② `r.Use(middleware.SystemDefaultKB(params.DB))` 全局挂中间件 |

### 不碰的文件

- 所有 handler（`session/qa.go` 等）
- 所有 service（`session_knowledge_qa.go`、`session_qa_helpers.go` 等）
- 所有 repository
- 数据库表结构（零 migration）
- `resolveKnowledgeBases` 函数
- `container.go`

## 中间件原理

中间件以全局 `r.Use()` 方式注册在认证之后，但通过内部 `shouldIntercept` 判断，只对以下路由生效：

- `POST /api/v1/knowledge-chat/`
- `POST /api/v1/agent-chat/`

其他请求直接 `c.Next()` 透传，零开销。

## 管理方式

无管理 API。管理员直接在 `system_settings` 表手动维护 `system_default_kb_ids` 记录即可，中间件每次请求实时读取，修改即时生效无需重启。
