import { ref, computed, watch } from 'vue'
import { get as httpGet } from '@/utils/request'
import type {
  GraphNodeView, GraphEdgeView, GraphRunView, KGData,
  KGPhase, GraphFullResponse, GraphNodeAPI, GraphEdgeAPI,
} from '@/types/knowledge-graph'
import {
  normalizeEntityType, edgeId, PHASE_NAMES,
} from '@/types/knowledge-graph'

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
  const startTime = ref(Date.now())
  const recentDiscoveries = ref<Array<{ time: string; text: string }>>([])
  const entityCounts = ref<Record<string, number>>({})
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
  function upsertNode(name: string, opts: { entityType?: string; status?: "searching" | "confirmed"; observations?: string[]; sourceKb?: string }) {
    const existing = nodesMap.get(name)
    const nt = normalizeEntityType(opts.entityType)
    if (existing) {
      if (opts.entityType) existing.entityType = nt
      if (opts.status) existing.status = opts.status
      if (opts.observations) existing.observations = opts.observations
      if (opts.sourceKb) existing.sourceKb = opts.sourceKb
    } else {
      nodesMap.set(name, { id: name, name, entityType: nt, status: opts.status ?? "searching", observations: opts.observations ?? [], sourceKb: opts.sourceKb, createdAt: performance.now() })
    }
  }
  function upsertEdge(s: string, t: string, r: string) {
    const id = edgeId(s, t, r)
    if (!edgesMap.has(id)) edgesMap.set(id, { id, source: s, target: t, relationType: r, createdAt: performance.now() })
  }
  function applyAgentGraphEvent(payload: Record<string, any>) {
    const ev = payload.event_type as string
    switch (ev) {
      case "EntitySearching": {
        const n = payload.entity_name as string
        if (n) upsertNode(n, { entityType: payload.entity_type, status: "searching", sourceKb: payload.source_kb })
        break
      }
      case "EntityConfirmed": {
        const n = payload.entity_name as string
        if (n) { upsertNode(n, { entityType: payload.entity_type, status: "confirmed", observations: payload.observations ?? [] }); addDiscovery(`发现${payload.entity_type||""} ${n}`, payload.timestamp) }
        break
      }
      case "RelationFound": {
        const s=payload.source_entity as string, t=payload.target_entity as string, r=payload.relation_type as string
        if(s&&t&&r){upsertEdge(s,t,r);addDiscovery(`${s} → ${r} → ${t}`,payload.timestamp)}
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
      case "RunComplete": isComplete.value=true;if(run.value){run.value.isComplete=true;if(payload.total_steps!==undefined)run.value.totalSteps=payload.total_steps}addDiscovery("✅ 图谱构建完成",payload.timestamp);break
    }
    updateEntityCounts();syncToRefs()
  }
  function patchRun(payload: Record<string, any>) {
    if(!run.value){run.value={phase:payload.phase||"broad_search",phaseSubtitle:payload.subtitle,step:payload.step,totalSteps:payload.total_steps,entitiesFound:payload.entities_found,relationsFound:payload.relations_found}}
    else{if(payload.phase)run.value.phase=payload.phase;if(payload.subtitle!==undefined)run.value.phaseSubtitle=payload.subtitle;if(payload.step!==undefined)run.value.step=payload.step;if(payload.total_steps!==undefined)run.value.totalSteps=payload.total_steps;if(payload.entities_found!==undefined)run.value.entitiesFound=payload.entities_found;if(payload.relations_found!==undefined)run.value.relationsFound=payload.relations_found}
  }
  async function fetchFullGraph(sessionId: string, messageId: string) {
    try {
      const res = await httpGet(`/api/v1/sessions/${sessionId}/messages/${messageId}/graph`, { params: { after_seq: 0 } })
      const data = (res as any).data ?? res
      replaceGraph(data.nodes ?? [], data.edges ?? [], data.run)
      lastSeq.value = data.last_seq ?? 0
    } catch (err) { console.error("[KG] fetchFullGraph failed:", err) }
  }
  function replaceGraph(apiNodes: GraphNodeAPI[], apiEdges: GraphEdgeAPI[], apiRun: GraphRunView | null) {
    const prevCreatedAt = new Map<string, number>()
    for (const n of nodesMap.values()) prevCreatedAt.set(n.name, n.createdAt ?? performance.now())
    const prevEdgeCreatedAt = new Map<string, number>()
    for (const e of edgesMap.values()) prevEdgeCreatedAt.set(e.id, e.createdAt ?? performance.now())
    nodesMap.clear();edgesMap.clear()
    const now = performance.now()
    apiNodes.forEach((n, idx) => {
      nodesMap.set(n.entity_name, { id: n.entity_name, name: n.entity_name, entityType: normalizeEntityType(n.entity_type), status: n.status ?? "confirmed", observations: n.observations ?? [], sourceKb: n.source_kb, createdAt: prevCreatedAt.get(n.entity_name) ?? now + idx * 15 })
    })
    for(const e of apiEdges){const id=edgeId(e.source_entity,e.target_entity,e.relation_type);edgesMap.set(id,{id,source:e.source_entity,target:e.target_entity,relationType:e.relation_type,createdAt:prevEdgeCreatedAt.get(id) ?? performance.now()})}
    if(apiRun){run.value={phase:apiRun.phase,phaseSubtitle:apiRun.phase_subtitle,step:apiRun.step,totalSteps:apiRun.total_steps,entitiesFound:apiRun.entities_found,relationsFound:apiRun.relations_found,isComplete:apiRun.is_complete};if(apiRun.phase)currentPhase.value=apiRun.phase;if(apiRun.phase_subtitle)phaseDescription.value=apiRun.phase_subtitle}
    updateEntityCounts();syncToRefs()
  }
  function reset() {
    nodesMap.clear();edgesMap.clear();run.value=null;lastSeq.value=0;isComplete.value=false
    currentPhase.value="broad_search";phaseDescription.value="正在广域检索生物医学实体..."
    startTime.value=Date.now();recentDiscoveries.value=[];entityCounts.value={};syncToRefs()
  }
  watch([nodes,links],()=>{refreshKey.value++},{deep:true})
  return { nodes,links,graphData,run,currentPhase,phaseDescription,startTime,isComplete,recentDiscoveries,entityCounts,totalNodes,totalLinks,refreshKey,applyAgentGraphEvent,patchRun,fetchFullGraph,replaceGraph,reset }
}