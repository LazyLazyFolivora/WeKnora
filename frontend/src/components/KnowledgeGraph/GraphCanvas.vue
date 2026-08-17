<template>
  <div ref="containerRef" class="graph-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import ForceGraph from 'force-graph'
import type { GraphNodeView, KGData } from '@/types/knowledge-graph'
import { ENTITY_COLORS } from '@/types/knowledge-graph'

interface Props {
  graphData: KGData
  entityColors?: Record<string, string>
  refreshKey?: number
}

const props = withDefaults(defineProps<Props>(), {
  entityColors: () => ENTITY_COLORS,
})

const emit = defineEmits<{
  nodeClick: [node: GraphNodeView]
  nodeHover: [node: GraphNodeView | null]
}>()

const containerRef = ref<HTMLDivElement>()
let graph: any = null
let hoveredNode: GraphNodeView | null = null

// ── 邻接缓存：hover 时 O(1) 查找 ──
const adjacentIds = new Set<string>()
function rebuildAdjacency(node: GraphNodeView | null) {
  adjacentIds.clear()
  if (!node) return
  for (const l of props.graphData.links) {
    const s = typeof l.source === 'object' ? (l.source as any).id : l.source
    const t = typeof l.target === 'object' ? (l.target as any).id : l.target
    if (s === node.id) adjacentIds.add(t)
    if (t === node.id) adjacentIds.add(s)
  }
}
function isDimmed(n: any): boolean {
  return !!hoveredNode && n.id !== hoveredNode.id && !adjacentIds.has(n.id)
}

// ── 防抖更新 + 防飘动：冻结已有节点，只让新节点参与布局 ──
let updatePending = false

function scheduleUpdate(data: KGData) {
  if (updatePending) return
  updatePending = true
  requestAnimationFrame(() => {
    if (!graph) { updatePending = false; return }

    // 1. 记录已有节点位置
    const prevNodes = graph.graphData().nodes
    const posMap = new Map<string, { fx: number; fy: number }>()
    for (const n of prevNodes) {
      posMap.set(n.id, { fx: n.x, fy: n.y })
    }

    // 2. 设置新数据，新节点不设 fx/fy 让力布局放置
    graph.graphData(data)

    // 3. 对已有节点立即冻结位置，防止飘动
    const currNodes = graph.graphData().nodes
    for (const n of currNodes) {
      const prev = posMap.get(n.id)
      if (prev) {
        n.fx = prev.fx
        n.fy = prev.fy
      }
      // 新节点 fx/fy = undefined，由力模拟自然放置
    }

    // 4. 仅对新节点解冻（力布局收敛后释放）
    const newCount = currNodes.length - prevNodes.length
    if (newCount > 0) {
      setTimeout(() => {
        for (const n of graph.graphData().nodes) {
          if (!posMap.has(n.id)) {
            n.fx = null
            n.fy = null
          }
        }
      }, 800)
    }

    updatePending = false
  })
}

onMounted(() => {
  if (!containerRef.value) return

  graph = ForceGraph()(containerRef.value)
    .backgroundColor('rgba(15, 15, 35, 0.4)')
    .nodeVal(12)
    .nodeColor((n: any) => {
      const color = props.entityColors[n.entityType] || '#888'
      return isDimmed(n) ? color + '33' : color
    })
    .nodeLabel(() => '')
    .nodeCanvasObject((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const dim = isDimmed(node)
      const scale = 1 / globalScale
      const r = 8 * scale
      const color = props.entityColors[node.entityType] || '#888'

      if (dim) ctx.globalAlpha = 0.15

      if (!dim) {
        ctx.beginPath()
        ctx.arc(node.x, node.y, r + 5 * scale, 0, 2 * Math.PI)
        ctx.fillStyle = color + '18'
        ctx.fill()
      }

      ctx.beginPath()
      ctx.arc(node.x, node.y, r, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()

      ctx.beginPath()
      ctx.arc(node.x - r * 0.3, node.y - r * 0.3, r * 0.35, 0, 2 * Math.PI)
      ctx.fillStyle = 'rgba(255,255,255,0.3)'
      ctx.fill()

      if (dim) ctx.globalAlpha = 0.2

      if (globalScale > 0.8) {
        const fs = 11 * scale
        ctx.font = `bold ${fs}px system-ui, sans-serif`
        ctx.textAlign = 'center'
        ctx.textBaseline = 'top'
        ctx.fillStyle = '#fff'
        ctx.fillText(node.name, node.x, node.y + r + 3 * scale)
      }

      ctx.globalAlpha = 1
    })
    .nodePointerAreaPaint((node: any, color: string, ctx: CanvasRenderingContext2D) => {
      ctx.beginPath()
      ctx.arc(node.x, node.y, 14, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()
    })
    .linkWidth(1.5)
    .linkColor(() => 'rgba(255,255,255,0.12)')
    .linkDirectionalParticles(1)
    .linkDirectionalParticleWidth(1.5)
    .linkDirectionalParticleSpeed(0.004)
    .linkDirectionalParticleColor(() => 'rgba(96,165,250,0.5)')
    .linkLabel((l: any) => l.relationType)
    .linkDirectionalArrowLength(5)
    .linkDirectionalArrowRelPos(1)
    .linkCurvature(0.1)
    .onNodeHover((node: any) => {
      hoveredNode = node
      rebuildAdjacency(node)
      containerRef.value!.style.cursor = node ? 'pointer' : 'default'
      emit('nodeHover', node)
    })
    .onNodeClick((node: any) => {
      emit('nodeClick', node)
    })
    .enableNodeDrag(true)
    .d3AlphaDecay(0.02)
    .d3VelocityDecay(0.4)
    .warmupTicks(30)
    .cooldownTicks(100)
    .cooldownTime(3000)

  graph.d3Force('charge')?.strength(-300)
  graph.d3Force('link')?.distance(200)

  graph.graphData(props.graphData)

  // 初始居中 + 固定已有节点
  setTimeout(() => {
    if (graph) {
      graph.centerAt(0, 0, 0)
      graph.zoom(1.2, 0)
      // 布局收敛后冻结所有节点位置
      const nodes = graph.graphData().nodes
      for (const n of nodes) { n.fx = n.x; n.fy = n.y }
    }
  }, 500)
})

// refreshKey 变化时用防抖 + 冻结策略更新
watch(
  () => props.refreshKey,
  () => {
    if (graph) scheduleUpdate(props.graphData)
  },
)

onUnmounted(() => {
  if (graph) graph._destructor?.()
})
</script>

<style scoped>
.graph-canvas {
  width: 100%;
  height: 100%;
  min-height: 300px;
}
</style>
