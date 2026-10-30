# 固定版本索引与 KDD 特征接口

本扩展把 Fabric 固定版本发布到现有 WeKnora，并提供明确名单的文档特征与检索回执。所有上传、元数据更新、解析取消和分块禁用调用现役 HTTP API；向量读取仅在 PostgreSQL 的强制只读事务中执行。它不启动 Go 主服务、消费原有队列或修改 Neo4j，Fabric 的既有持久发布程序负责重试。

2026-10-30 核查的线上实例来自 `sync_v0.8.0`、提交 `6f2c2955`、数据库版本 90，支持 custom_metadata、content_revision、index_status。zktest 原 checkout 是 `e01ac7cf` 的 v0.6.2；不要用旧 Go 主服务连接线上新库。本分支只添加这个与部署 API 兼容的扩展，不替换学生主服务或部署文件。

## 运行

使用 Python 3.10+，在独立虚拟环境安装 `extensions/kdd/requirements.txt`，然后从仓库根目录运行：

```sh
WEKNORA_KDD_CONFIG=/private/path/runtime.json python -m uvicorn \
  extensions.kdd.app:app_factory --factory --host 127.0.0.1 --port 18563 --workers 1 --no-access-log
python -m unittest extensions.kdd.test_service -v
```

本轮隔离验证复用了 zktest 上已有的 BioRadar Python 环境；运行时不导入其业务模块。开发服务只监听回环地址。上游 URL、API key、数据库连接和模型私有配置由运维填入 600 权限的配置文件，不放入 Git。数据库账户应限制为只读；连接还强制 `default_transaction_read_only`、只读事务和 15 秒语句超时。

`runtime.json` 包含 `token`、`upstream_url`、`upstream_api_key`、`db`（psycopg 连接参数对象）、`models`（现役 embedding/rerank 的固定模型配置）、`journal_path`、`allowed_kb_ids` 和 `source_mappings`。每条映射是 `{source_instance, source_id, knowledge_base_id, scopes:[字符串]}`。每次授权重新读取映射，撤销即时生效。不要把客户端的 scope 字符串当作授权。生产使用独立配置和持久 `journal_path`，重启时保留 SQLite 回执库；不要运行多个副本或删回执后重发。

当前模型协议支持现有 generic/OpenAI 兼容的 embedding/rerank。每次实际模型调用核对数据库里的配置版本；配置漂移报冲突或明确降级，不继续谎报旧版本。`model_version` 是配置 revision 的哈希，权重文件版本由上游未报告时 `artifact_version=null`。

## 最小调用顺序

请求均使用 `Authorization: Bearer <服务令牌>`。字段、错误码与示例形状见 [contract.json](contract.json)，服务的 `/openapi.json` 由实际 Pydantic 请求类型生成。

1. `POST /api/v1/kdd/publications` 传入明确 KB、授权 scope、source_instance/source_id/source_record_id/version_id/raw_sha256、同身份 raw_locator、title 和清洗正文 content。content_sha256 若提供必须与 UTF-8 正文一致。raw_sha256 是 Fabric 原件，不能用清洗正文哈希替代。
2. 同一固定版本重放返回同一 knowledge_id。正文、原件哈希、locator 或 scope 不一致返回 409。新 version_id 形成新知识条目；旧版通过明确 disable 操作撤回或标为 superseded，不原地覆盖。
3. `GET /api/v1/kdd/publications/{publication_id}` 查询上传、解析、实际文本块向量覆盖、索引和原件绑定状态。只有解析完成/收尾且所有文本块确有启用向量、身份一致才返回 ready。知识库摘要计数不用于推断索引就绪。
4. `POST /api/v1/kdd/features` 给定 1–100 个完整固定版本 members。响应列出每个文档特征与 excluded，返回实际清洗文本、版本、1024 维向量、模型配置版本、分块配置和聚合规则。单次总计最多 10 MB 文本、10000 块，单文档最多 2000 块；超过应拆分明确名单。它不通过 top-k 选择样本。
5. `POST /api/v1/kdd/search` 调用同一套 embedding 模型，再分别使用现役关键词/向量召回、RRF 合并和现役 rerank 模型。响应说明每通道执行、失败降级、模型用量是否报告、模型与索引版本；全部召回失败返回 502。
6. `POST /api/v1/kdd/publications/{publication_id}/disable` 先关闭扩展读取，再取消未完成解析、逐块调用原 API 禁用并从实际索引验证。撤销来源映射后仍允许对本服务既有绑定做无正文状态查询和清理；features/search 始终执行当前授权。

## 身份、向量与恢复

publication_id 是 `[knowledge_base_id,source_instance,source_id,source_record_id,version_id]` 规范 JSON 数组的 SHA-256（UTF-8，ensure_ascii=False，separators=(',',':')）。原件身份原子写入上传 metadata。原生异步解析会保存上传时的 Knowledge 快照，因此解析 completed 后才复制到原生 API 可读的 custom_metadata；binding pending 时由 Fabric 用同一 publication 重放完成投影，不重新上传。原子身份或已有投影冲突仍报错，空投影不冒充撤权。chunk 以现有 knowledge_id 外键绑定，扩展逐块返回身份，不创建第二套权威来源表。

上传内容增加一行固定版本注释，使相同正文的两个版本不会被原生文件去重合并。content_sha256 对应清洗正文，indexed_text_sha256 对应实际上传内容。分块配置通过本次上传 process_config 固定；块向量可包含 WeKnora 的上下文标题。include_chunks 会同时返回实际 embeddings.content 的哈希。features 返回的正文去除扩展注释（取自不可变发布回执），原始原件仍回 Fabric 读取。真实登记的标题可能超过原生展示列 255 字符；接口接受最多 500 字符并在回执/特征中保留完整标题，只把原生展示列裁到 255，返回 display_title_truncated，身份和正文哈希保持不变。

聚合仅选启用的原始 text 块、source_type=0 且 source_id=chunk_id 的向量，先按块 L2 归一化，求均值，再对文档向量 L2 归一化。父块、摘要、自动问题、图实体不作为独立研究。实际向量缺失、维度不符、模型记录消失、模型配置变化或人工修改块时不返回已验证向量。文档向量不直接代表项目或独立研究；Radar 仍须完成单位/队列映射和适用性评价。

发送前持久记录回执。丢响应时按原子上传 metadata 和确定文件名查找唯一知识条目；没有唯一结果则 409 保留 unknown，不自动盲重发。进程失败后使用原 publication_id 查询/重放。清理失败已禁止特征与检索读取，重复 disable 可继续完成向量排除。资料正文与回执库均是受控运行数据，不提交仓库。

图谱审核与文档索引分别有状态。本扩展所有上传关闭自动抽图、自动问题和摘要；唐的 Fabric 审核→WeKnora 图镜像→Neo4j 链保持原有入口与权威关系。

## 本轮证据

本轮以独立 synthetic KB 验证幂等、冲突、双版本、真实向量/重排、撤权与索引禁用；业务四源使用另一独立 KB，二者分开。机器可读实测回执在协调运行目录，包含 `synthetic`、实际模型、scope/版本和每项检查结果。单元测试覆盖丢响应恢复、来源授权、哈希、模型来源缺失、解析完成但缺向量、编辑后的向量、撤权后清理。真实四方向材料还需通过 Fabric 发布和 Radar 消费验收，不能把 synthetic 工程测试视作研究效果。
