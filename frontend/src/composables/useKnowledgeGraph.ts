import { ref, computed } from 'vue'
import { get as httpGet } from '@/utils/request'
import type {
  GraphNodeView, GraphEdgeView, GraphRunView, KGData,
  KGPhase, GraphFullResponse, GraphNodeAPI, GraphEdgeAPI,
} from '@/types/knowledge-graph'
import {
  normalizeEntityType, edgeId, PHASE_NAMES,
} from '@/types/knowledge-graph'

// ── 事件缓冲配置 ──
// 解决后端事件流不均匀导致的节点批量冒出、关系瞬间连接等问题
const BUFFER_MIN_MS = 5000   // 消费间隔下限（ms）
const BUFFER_MAX_MS = 10000  // 消费间隔上限（ms）
const SPRINT_MS = 250        // 收尾冲刺：完成信号后剩余缓冲的快速消费间隔
const BUFFER_FAST_MS = 3000  // 缓冲堆积时的加速消费间隔（防止图谱落后回答太多）
const BUFFER_HIGH_WATER = 10 // 缓冲水位阈值：待消费事件数超过此值则加速
const IMMEDIATE_EVENT_TYPES = new Set([
  'PhaseChange', 'Progress', 'LiteratureSearching', 'RunComplete',
])

export function useKnowledgeGraph() {
  const nodesMap = new Map<string, GraphNodeView>()
  const edgesMap = new Map<string, GraphEdgeView>()
  const run = ref<GraphRunView | null>(null)
  const lastSeq = ref(0)
  const isComplete = ref(false)
  const nodes = ref<GraphNodeView[]>([])
  const links = ref<GraphEdgeView[]>([])
  const refreshKey = ref(0)
  const currentPhase = ref<KGPhase>("broad_search")
  const phaseDescription = ref("正在广域检索生物医学实体...")
  const startTime = ref<number | null>(null) // null = 尚未开始，图谱有数据后才启动计时
  const recentDiscoveries = ref<Array<{ time: string; text: string }>>([])
  const entityCounts = ref<Record<string, number>>({})
  // 历史消息预计算时长（秒），非 null 时 GraphProgressPanel 直接显示此值
  const frozenElapsed = ref<number | null>(null)
  // 记录最先出现的实体名称（用户问题涉及的起始实体）
  const startEntityNames = ref<string[]>([])
  // ── 事件缓冲队列 ──
  const eventBuffer: Record<string, any>[] = []
  let consumeTimer: ReturnType<typeof setTimeout> | null = null
  const totalNodes = computed(() => nodes.value.length)
  const totalLinks = computed(() => links.value.length)
  const graphData = computed<KGData>(() => ({ nodes: nodes.value, links: links.value }))

  function formatTime(ts?: number): string {
    const d = new Date(ts ?? Date.now())
    return `${String(d.getHours()).padStart(2,"0")}:${String(d.getMinutes()).padStart(2,"0")}:${String(d.getSeconds()).padStart(2,"0")}`
  }
  function syncToRefs() {
    nodes.value = Array.from(nodesMap.values())
    links.value = Array.from(edgesMap.values())
    refreshKey.value++
  }
  function updateEntityCounts() {
    const counts: Record<string, number> = {}
    for (const n of nodesMap.values()) { counts[n.entityType] = (counts[n.entityType] || 0) + 1 }
    entityCounts.value = counts
  }
  function addDiscovery(text: string, ts?: number) {
    recentDiscoveries.value = [{ time: formatTime(ts), text }, ...recentDiscoveries.value].slice(0, 8)
  }
  // 状态等级：只升级不降级
  const STATUS_RANK: Record<string, number> = { planned: 0, searching: 1, confirmed: 2 }

  function upsertNode(name: string, opts: { entityType?: string; status?: "planned" | "searching" | "confirmed"; observations?: string[]; sourceKb?: string; confidence?: number }) {
    const existing = nodesMap.get(name)
    const nt = normalizeEntityType(opts.entityType)
    if (existing) {
      if (opts.entityType) existing.entityType = nt
      if (opts.status) {
        const cur = STATUS_RANK[existing.status] ?? -1
        const next = STATUS_RANK[opts.status] ?? -1
        if (next >= cur) existing.status = opts.status // 只升不降
      }
      if (opts.observations) existing.observations = opts.observations
      if (opts.sourceKb) existing.sourceKb = opts.sourceKb
      if (opts.confidence !== undefined) existing.confidence = opts.confidence
    } else {
      // 新节点给随机初始位置，避免全部堆在 (0,0)
      const angle = Math.random() * 2 * Math.PI
      const dist = 80 + Math.random() * 120
      nodesMap.set(name, {
        id: name, name, entityType: nt,
        status: opts.status ?? "planned",
        observations: opts.observations ?? [],
        sourceKb: opts.sourceKb,
        confidence: opts.confidence,
        x: Math.cos(angle) * dist,
        y: Math.sin(angle) * dist,
        _createdAt: Date.now(), // 记录创建时间，用于浮现动画
      })
    }
  }
  function upsertEdge(s: string, t: string, r: string, opts?: { contradiction?: boolean; strength?: number }) {
    const id = edgeId(s, t, r)
    if (edgesMap.has(id)) {
      // 后写覆盖同名边（允许 contradiction/strength 状态更新）
      const existing = edgesMap.get(id)!
      if (opts?.contradiction !== undefined) existing.contradiction = opts.contradiction
      if (opts?.strength !== undefined) existing.strength = opts.strength
    } else {
      edgesMap.set(id, { id, source: s, target: t, relationType: r, contradiction: opts?.contradiction, strength: opts?.strength ?? 0.5, _createdAt: Date.now() })
    }
  }
  // ── 缓冲消费者 ──
  function randomConsumeDelay() {
    return BUFFER_MIN_MS + Math.random() * (BUFFER_MAX_MS - BUFFER_MIN_MS)
  }
  // 消费间隔：平时随机 5-10s（生长感），缓冲堆积超过水位时加速（追赶回答进度）
  function consumeDelay() {
    return eventBuffer.length >= BUFFER_HIGH_WATER ? BUFFER_FAST_MS : randomConsumeDelay()
  }
  function startConsumer() {
    if (consumeTimer) return
    const tick = () => {
      if (eventBuffer.length === 0) { stopConsumer(); return }
      processEventNow(eventBuffer.shift()!)
      consumeTimer = setTimeout(tick, consumeDelay())
    }
    consumeTimer = setTimeout(tick, consumeDelay())
  }
  function stopConsumer() {
    if (consumeTimer != null) { clearTimeout(consumeTimer); consumeTimer = null }
  }
  // 收尾冲刺：以 SPRINT_MS 短间隔快速消费剩余缓冲，保留渐入生长感（而非瞬间全冒出来）
  let isSprinting = false
  function flushBuffer() {
    if (isSprinting) return
    isSprinting = true
    stopConsumer()
    const sprint = () => {
      if (eventBuffer.length === 0) { isSprinting = false; isComplete.value = true; return }
      processEventNow(eventBuffer.shift()!)
      consumeTimer = setTimeout(sprint, SPRINT_MS)
    }
    sprint()
  }
  // 完成信号（回答完毕 / 图谱 RunComplete）：冲刺消费剩余缓冲，清空后标记完成
  function complete() {
    flushBuffer()
  }
  // ── 立即处理（内部） ──
  function processEventNow(payload: Record<string, any>) {
    const ev = payload.event_type as string
    switch (ev) {
      case "EntityPlanned": {
        const n = payload.entity_name as string
        if (n) {
          const confidence = typeof payload.confidence === 'number' ? payload.confidence : undefined
          upsertNode(n, { entityType: payload.entity_type, status: "planned", sourceKb: payload.source_kb, confidence })
          // EntityPlanned = 用户问题中的关键实体，记录为起始节点
          if (!startEntityNames.value.includes(n)) {
            startEntityNames.value.push(n)
          }
        }
        break
      }
      case "EntitySearching": {
        const n = payload.entity_name as string
        if (n) {
          const confidence = typeof payload.confidence === 'number' ? payload.confidence : undefined
          upsertNode(n, { entityType: payload.entity_type, status: "searching", sourceKb: payload.source_kb, confidence })
          // EntitySearching 不记录为起始节点，只有 EntityPlanned 才是起始节点
        }
        break
      }
      case "EntityConfirmed": {
        const n = payload.entity_name as string
        if (n) { upsertNode(n, { entityType: payload.entity_type, status: "confirmed", observations: payload.observations ?? [] }); addDiscovery(`发现${payload.entity_type||""} ${n}`, payload.timestamp) }
        break
      }
      case "RelationFound": {
        const s=payload.source_entity as string, t=payload.target_entity as string, r=payload.relation_type as string
        const contradiction = payload.contradiction === true || payload.is_contradiction === true
        const strength = typeof payload.strength === 'number' ? payload.strength : undefined
        // 调试日志：验证后端是否发送 strength
        console.log(`[KG] RelationFound: ${s} -> ${r} -> ${t} strength=${strength} contradiction=${contradiction}`)
        if(s&&t&&r){upsertEdge(s,t,r,{ contradiction, strength });addDiscovery(`${contradiction?'⚠️ 矛盾: ':''}${s} → ${r} → ${t}`,payload.timestamp)}
        break
      }
      case "PhaseChange": {
        const p=payload.phase as KGPhase
        if(p){currentPhase.value=p;phaseDescription.value=payload.subtitle||PHASE_NAMES[p]||p;addDiscovery(`阶段切换: ${PHASE_NAMES[p]||p}`,payload.timestamp)}
        break
      }
      case "Progress": {
        if(run.value){if(payload.step!==undefined)run.value.step=payload.step;if(payload.total_steps!==undefined)run.value.totalSteps=payload.total_steps;if(payload.entities_found!==undefined)run.value.entitiesFound=payload.entities_found;if(payload.relations_found!==undefined)run.value.relationsFound=payload.relations_found}
        break
      }
      case "LiteratureSearching": addDiscovery("正在检索相关文献...",payload.timestamp);break
      case "RunComplete": if(run.value){run.value.isComplete=true;if(payload.total_steps!==undefined)run.value.totalSteps=payload.total_steps}addDiscovery("✅ 图谱构建完成",payload.timestamp);break
    }
    // 首个 SSE 事件到达时，记录图谱开始生长的时刻作为计时起点
    if (startTime.value === null) {
      startTime.value = Date.now()
    }
    updateEntityCounts();syncToRefs()
  }
  // ── 公开入口：带缓冲的事件处理 ──
  function applyAgentGraphEvent(payload: Record<string, any>, options?: { immediate?: boolean }) {
    const ev = payload.event_type as string
    if (IMMEDIATE_EVENT_TYPES.has(ev) || options?.immediate) {
      processEventNow(payload)
      // RunComplete: 冲刺消费缓冲区中剩余事件，清空后标记完成
      if (ev === 'RunComplete') complete()
      return
    }
    // 节点/边事件进入缓冲队列，以固定节奏消费
    eventBuffer.push(payload)
    startConsumer()
  }
  function patchRun(payload: Record<string, any>) {
    if(!run.value){run.value={phase:payload.phase||"broad_search",phaseSubtitle:payload.subtitle,step:payload.step,totalSteps:payload.total_steps,entitiesFound:payload.entities_found,relationsFound:payload.relations_found}}
    else{if(payload.phase)run.value.phase=payload.phase;if(payload.subtitle!==undefined)run.value.phaseSubtitle=payload.subtitle;if(payload.step!==undefined)run.value.step=payload.step;if(payload.total_steps!==undefined)run.value.totalSteps=payload.total_steps;if(payload.entities_found!==undefined)run.value.entitiesFound=payload.entities_found;if(payload.relations_found!==undefined)run.value.relationsFound=payload.relations_found}
  }
  async function fetchFullGraph(sessionId: string, messageId: string, completed?: boolean) {
    try {
      const res = await httpGet(`/api/v1/sessions/${sessionId}/messages/${messageId}/graph`, { params: { after_seq: 0 } })
      const data = (res as any).data ?? res
      // 仅在 agent 已完成时全量加载图数据（避免覆盖 SSE 缓冲中的实时数据）
      if (completed) {
        replaceGraph(data.nodes ?? [], data.edges ?? [], data.run)
      }
      lastSeq.value = data.last_seq ?? 0
      // 始终从 started_at 恢复计时器（无论是否完成）
      if (data.run?.started_at) {
        const startMs = new Date(data.run.started_at).getTime()
        if (startMs > 0) {
          startTime.value = startMs
          if (data.run.is_complete) {
            isComplete.value = true
            frozenElapsed.value = Math.max(0, Math.floor((Date.now() - startMs) / 1000))
          }
        }
      }
      // 如果 startEntityNames 为空，从 API 响应中提取 status="planned" 的节点作为起始节点
      if (startEntityNames.value.length === 0 && data.nodes?.length) {
        for (const n of data.nodes) {
          if (n.status === 'planned' && !startEntityNames.value.includes(n.entity_name)) {
            startEntityNames.value.push(n.entity_name)
          }
        }
      }
    } catch (err: any) {
      const status = err?.status || err?.response?.status || 'unknown'
      const msg = err?.message || err?.response?.data?.message || String(err)
      console.error(`[KG] fetchFullGraph failed: status=${status} msg=${msg}`)
    }
  }
  function replaceGraph(apiNodes: GraphNodeAPI[], apiEdges: GraphEdgeAPI[], apiRun: GraphRunView | null) {
    nodesMap.clear();edgesMap.clear()
    for(const n of apiNodes){nodesMap.set(n.entity_name,{id:n.entity_name,name:n.entity_name,entityType:normalizeEntityType(n.entity_type),status:n.status??"confirmed",observations:n.observations??[],sourceKb:n.source_kb,confidence:n.confidence})}
    // 补全边引用的缺失节点（后端 edges 可能引用了 nodes 中没有的实体）
    for(const e of apiEdges){
      if(e.source_entity && !nodesMap.has(e.source_entity)){
        nodesMap.set(e.source_entity,{id:e.source_entity,name:e.source_entity,entityType:'unknown',status:'confirmed',observations:[]})
      }
      if(e.target_entity && !nodesMap.has(e.target_entity)){
        nodesMap.set(e.target_entity,{id:e.target_entity,name:e.target_entity,entityType:'unknown',status:'confirmed',observations:[]})
      }
      const id=edgeId(e.source_entity,e.target_entity,e.relation_type);edgesMap.set(id,{id,source:e.source_entity,target:e.target_entity,relationType:e.relation_type,contradiction:e.contradiction,strength:e.strength??0.5})
    }
    if(apiRun){run.value={phase:apiRun.phase,phaseSubtitle:apiRun.phase_subtitle,step:apiRun.step,totalSteps:apiRun.total_steps,entitiesFound:apiRun.entities_found,relationsFound:apiRun.relations_found,isComplete:apiRun.is_complete,startedAt:apiRun.started_at};if(apiRun.phase)currentPhase.value=apiRun.phase;if(apiRun.phase_subtitle)phaseDescription.value=apiRun.phase_subtitle
      // 同步顶层 isComplete（供 GraphProgressPanel 计时停止）
      if(apiRun.is_complete) isComplete.value = true
      // 用后端 started_at 还原计时起点（RFC3339 字符串 → ms）
      if(apiRun.started_at) {
        const t = new Date(apiRun.started_at).getTime()
        if (t > 0) startTime.value = t
      }
    }
    updateEntityCounts();syncToRefs()
  }
  function reset() {
    nodesMap.clear();edgesMap.clear();run.value=null;lastSeq.value=0;isComplete.value=false
    currentPhase.value="broad_search";phaseDescription.value="正在广域检索生物医学实体..."
    startEntityNames.value=[];frozenElapsed.value=null
    startTime.value=null;recentDiscoveries.value=[];entityCounts.value={};syncToRefs()
    stopConsumer();eventBuffer.length=0
  }
  function destroy() { stopConsumer();eventBuffer.length=0 }
  // 注意：不要 deep watch nodes/links，force-graph 拖拽会修改 node.x/y
  // 导致 refreshKey 无限递增 → scheduleUpdate 重置力布局 → 图谱消失
  // refreshKey 已在 syncToRefs() 中正确递增
  return { nodes,links,graphData,run,currentPhase,phaseDescription,startTime,isComplete,frozenElapsed,recentDiscoveries,entityCounts,totalNodes,totalLinks,refreshKey,startEntityNames,applyAgentGraphEvent,patchRun,fetchFullGraph,replaceGraph,reset,destroy,complete }
}