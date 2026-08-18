/**
 * Knowledge Graph Types
 * 知识图谱实时渲染 - 类型定义
 * 对齐后端协议: docs/api/agent-graph-frontend-protocol.md
 */

// ─── 实体类型 ────────────────────────────────────────────────

export type EntityType =
  | 'gene' | 'variant' | 'drug' | 'compound' | 'disease'
  | 'pathway' | 'target' | 'literature' | 'finding'
  | 'cell_line' | 'tissue' | 'unknown'

/** 实体类型颜色映射（对齐后端协议 §4.1） */
export const ENTITY_COLORS: Record<EntityType, string> = {
  gene: '#2F6FED',
  variant: '#7C5CFC',
  drug: '#0F9F6E',
  compound: '#0D9488',
  disease: '#E11D48',
  pathway: '#D97706',
  target: '#2563EB',
  literature: '#64748B',
  finding: '#DB2777',
  cell_line: '#0891B2',
  tissue: '#CA8A04',
  unknown: '#94A3B8',
}

/** 实体类型中文名 */
export const ENTITY_TYPE_NAMES: Record<EntityType, string> = {
  gene: '基因',
  variant: '变异',
  drug: '药物',
  compound: '化合物',
  disease: '疾病',
  pathway: '通路',
  target: '靶点',
  literature: '文献',
  finding: '发现',
  cell_line: '细胞系',
  tissue: '组织',
  unknown: '未知',
}

// ─── 图谱节点 / 边视图 ──────────────────────────────────────

export type NodeStatus = 'planned' | 'searching' | 'confirmed'

/** 图谱节点（前端视图层，对齐协议 §0 GraphNodeView） */
export interface GraphNodeView {
  id: string              // = entity_name（同 message 内唯一）
  name: string
  entityType: EntityType
  status: NodeStatus      // searching = 虚线/脉冲, confirmed = 实色
  observations: string[]
  sourceKb?: string
  // force-graph 渲染坐标（运行时赋值）
  x?: number
  y?: number
  fx?: number | null
  fy?: number | null
}

/** 图谱边（前端视图层，对齐协议 §0 GraphEdgeView） */
export interface GraphEdgeView {
  id: string              // = `${source}\0${target}\0${relation}`
  source: string | GraphNodeView
  target: string | GraphNodeView
  relationType: string
  contradiction?: boolean // true = 与已有证据矛盾，红色虚线 + 闪烁
}

/** 图谱数据（传给 force-graph） */
export interface KGData {
  nodes: GraphNodeView[]
  links: GraphEdgeView[]
}

// ─── 研究阶段 ────────────────────────────────────────────────

export type KGPhase = 'broad_search' | 'deep_dive'

/** 阶段中文名 */
export const PHASE_NAMES: Record<KGPhase, string> = {
  broad_search: '广域检索',
  deep_dive: '深入挖掘',
}

// ─── 运行状态 ────────────────────────────────────────────────

/** 图谱运行状态（对齐协议 §0 GraphRunView） */
export interface GraphRunView {
  phase: KGPhase
  phaseSubtitle?: string
  step?: number
  totalSteps?: number
  entitiesFound?: number
  relationsFound?: number
  isComplete?: boolean
  startedAt?: number  // Unix 秒，后端 started_at
}

// ─── SSE 事件类型（对齐后端协议 §2.2 / §4） ─────────────────

/** SSE data 层结构（response_type = "agent_graph"） */
export interface AgentGraphSSEData {
  seq: number
  graph_event: GraphEventType
  tool_call_id: string
  payload: Record<string, any>
}

export type GraphEventType =
  | 'EntityPlanned'
  | 'EntitySearching'
  | 'EntityConfirmed'
  | 'RelationFound'
  | 'PhaseChange'
  | 'Progress'
  | 'LiteratureSearching'
  | 'RunComplete'

// ─── GET API 响应 ────────────────────────────────────────────

export interface GraphNodeAPI {
  entity_name: string
  entity_type: string
  status: NodeStatus
  observations?: string[]
  source_kb?: string
}

export interface GraphEdgeAPI {
  source_entity: string
  target_entity: string
  relation_type: string
  contradiction?: boolean
}

export interface GraphRunAPI {
  phase: KGPhase
  phase_subtitle?: string
  step?: number
  total_steps?: number
  entities_found?: number
  relations_found?: number
  is_complete?: boolean
  started_at?: string  // RFC3339, e.g. "2026-08-17T12:00:00Z"
}

export interface GraphFullResponse {
  nodes: GraphNodeAPI[]
  edges: GraphEdgeAPI[]
  run: GraphRunAPI | null
  last_seq: number
}

// ─── 关系类型中文映射 ────────────────────────────────────────

export const RELATION_LABELS: Record<string, string> = {
  ASSOCIATED_WITH: '相关',
  TREATS: '治疗',
  INHIBITS: '抑制',
  ACTIVATES: '激活',
  TARGETS: '靶向',
  BINDS: '结合',
  SUPPORTS: '支持',
  MEMBER_OF_PATHWAY: '属于通路',
}

/** 将大写蛇形关系类型转为可读标签 */
export function prettifyRelation(relationType: string): string {
  return RELATION_LABELS[relationType]
    ?? relationType.replace(/_/g, ' ').toLowerCase()
}

// ─── 工具函数 ────────────────────────────────────────────────

/** 将后端 entity_type 归一化为前端 EntityType（别名归一，协议 §4.1） */
export function normalizeEntityType(raw: string | null | undefined): EntityType {
  if (!raw) return 'unknown'
  const lower = raw.toLowerCase()
  const aliasMap: Record<string, EntityType> = {
    protein: 'gene',
    gene_set: 'pathway',
    phenotype: 'disease',
    paper: 'literature',
  }
  return (aliasMap[lower] ?? lower) as EntityType
}

/** 根据 entity_type 获取颜色 */
export function colorForEntityType(t: string | null | undefined): string {
  const key = normalizeEntityType(t)
  return ENTITY_COLORS[key] ?? ENTITY_COLORS.unknown
}

/** 边 key 生成：`${source}\0${target}\0${relation}` */
export function edgeId(source: string, target: string, relation: string): string {
  return `${source}\0${target}\0${relation}`
}
