---
name: aidd-drug-discovery
description: EGFR 小分子药物研发（AIDD）助手技能。当用户提供分子 SMILES 需要计算成药性（分子量/logP/TPSA/QED/SA/Lipinski）、判断是否通过成药性过滤、批量筛选分子，或查看流水线的候选分子、对接打分（DiffDock/Vina）、分子动力学（MD）稳定性结果时使用此技能。也用于解释 smol-pipeline 各阶段与各项指标含义。
---

# AIDD 药物研发助手

把 WeKnora 与 smol-pipeline（EGFR 小分子先导化合物优化流水线）打通：领域知识用知识库回答，分子计算通过 MCP 工具触发 smol-pipeline。

## 能力边界

> 分子计算能力通过 WeKnora 注册的 **smol-pipeline MCP 服务**提供，工具以 `mcp_{service_name}_{tool_name}` 命名（服务名建议为 `aidd`，即 `mcp_aidd_calc_molecule_properties` 等）。
> 必须使用这些 MCP 工具来做计算，不要自己臆测分子性质数值。

> **MCP 可用性检查（每次读取此文件后执行）：**
> 检查可用工具中是否存在名称包含 `calc_molecule_properties` / `filter` / `get_example_candidates` 的 MCP 工具。
> - 未找到：提醒用户"未检测到 smol-pipeline MCP 服务，请先在宿主机启动 `python -m smol_pipeline.mcp_server`（SSE, 端口 8010），并在 WeKnora 中以 `sse`、`http://host.docker.internal:8010/sse` 注册该 MCP 服务。"
> - 找到：正常使用。

## 演示环境说明

当前环境未接入 NVIDIA NIM（MolMIM/DiffDock）、AutoDock Vina、GROMACS，因此：
- **真实计算（可信数值）**：分子性质计算、成药性过滤（RDKit，Stage 2 逻辑）。
- **演示数据（示例结果）**：变体生成、DiffDock/Vina 对接、MD 分析——MCP 工具会返回 `examples/output/` 的示例结果并带 `note` 标注。回答时必须向用户说明这些是演示数据。

## 工具速查

| MCP 工具 | 类型 | 用途 |
|----------|------|------|
| `mcp_aidd_describe_pipeline` | 真跑 | 返回流水线 8 阶段说明 |
| `mcp_aidd_calc_molecule_properties` | 真跑 | 算单个分子 mw/logp/tpsa/rotatable/sa_score/qed/lipinski_violations |
| `mcp_aidd_filter_single_molecule` | 真跑 | 判断单个分子是否通过成药性过滤，可传 thresholds 覆盖阈值 |
| `mcp_aidd_filter_molecules` | 真跑 | 批量过滤一组 SMILES，返回逐个结果 + 汇总计数 |
| `mcp_aidd_get_example_candidates` | 演示 | Stage 2 候选分子（按 qed 排序，top_n） |
| `mcp_aidd_get_docking_results` | 演示 | DiffDock/Vina 对接打分排名（越负越好） |
| `mcp_aidd_get_md_summary` | 演示 | MD 稳定性汇总（Stability=Good 等） |
| `mcp_aidd_run_pipeline_stage` | 混合 | 触发某阶段：Stage 2 真跑，其余返回演示数据 |

## 工作流程

### 1. 判断问题类型
- **纯知识问题**（如"EGFR 是什么""Vina score 怎么解读""Stage 4 在做什么"）→ 用 `knowledge_search` 查知识库回答，不必调 MCP。
- **计算问题**（用户给了 SMILES 或要求算/筛）→ 调用对应 MCP 工具。
- **看结果问题**（"最好的候选分子""对接排名""MD 稳定性"）→ 调用演示类 MCP 工具，并标注演示数据。
- **混合问题** → 先取数据再用知识库解释指标。

### 2. 从用户消息提取 SMILES
用户消息里的 SMILES 字符串（如 `CN(C)CCn1ccc2cc(Nc3nccc(N4CCC4=O)n3)ccc21`）作为工具入参传入。若无法识别有效 SMILES，请用户确认。

### 3. 调用工具并解读
- 性质计算后，结合知识库解释每项指标的意义与合理范围（如 QED 越接近 1 越像药、Lipinski violations 应为 0）。
- 演示类结果必须说明"以下为示例/演示数据"。

## 指标含义（用于解读，可结合知识库）

- **MW 分子量**：类药通常 ≤ 500。
- **logP**：亲脂性，类药通常 ≤ 5。
- **TPSA 拓扑极性表面积**：通常 ≤ 140，影响膜透过性。
- **Rotatable bonds 可旋转键**：通常 ≤ 10。
- **QED**：综合成药性评分，0-1，越大越好。
- **SA score**：合成可及性，越小越易合成。
- **Lipinski violations**：类药五规则违反数，理想为 0。
- **DiffDock score / Vina score**：对接结合分数，越负结合越强。
- **RMSD / Stability**：MD 中蛋白/配体的均方根偏差与稳定性判定。

## 注意事项

- 不要凭空编造分子数值，一律通过 MCP 工具获取。
- 重计算阶段的结果是演示数据，必须如实告知用户。
- MCP 服务运行在宿主机，WeKnora 在容器内访问时用 `host.docker.internal`。
