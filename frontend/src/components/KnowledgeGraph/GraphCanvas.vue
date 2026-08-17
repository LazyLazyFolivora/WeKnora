<template>
  <div ref="containerRef" class="graph-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import ForceGraph from 'force-graph'
import type { GraphNodeView, KGData, KGPhase } from '@/types/knowledge-graph'
import { ENTITY_COLORS, prettifyRelation } from '@/types/knowledge-graph'

const NODE_APPEAR_MS = 900
const EDGE_GROW_MS = 900
const PHASE_ZOOM_MS = 800
const LABEL_PULSE_MS = 600

/** easeOutBack：带轻微过冲（overshoot）的弹入缓动，节点出现带"弹跳感" */
function easeOutBack(x: number): number {
  const c1 = 1.70158
  const c3 = c1 + 1
  return 1 + c3 * Math.pow(x - 1, 3) + c1 * Math.pow(x - 1, 2)
}

/** ease-out cubic，生长段缓动（快→慢收尾） */
function easeOutCubic(x: number): number {
  return 1 - Math.pow(1 - x, 3)
}

/** 解析边两端点坐标。source/target 可能是节点对象，也可能是 string id */
function linkPoints(l: any) {
  const nodes = graph?.graphData().nodes ?? []
  const resolve = (x: any) => {
    if (x && typeof x === 'object' && 'x' in x) return { x: x.x ?? 0, y: x.y ?? 0, id: x.id }
    const n = nodes.find((n: any) => n.id === x)
    if (n) return { x: n.x ?? 0, y: n.y ?? 0, id: n.id }
    return { x: 0, y: 0, id: x }
  }
  return { s: resolve(l.source), t: resolve(l.target) }
}

function drawGrowingLink(l: any, ctx: CanvasRenderingContext2D, globalScale: number) {
  const { s, t } = linkPoints(l)
  const dx = t.x - s.x, dy = t.y - s.y
  const len = Math.hypot(dx, dy)
  if (len < 0.001) return          // 自环 / 两端点重叠：跳过
  const scale = 1 / globalScale    // 保持屏幕像素恒定，与节点绘制一致

  const now = performance.now()
  const p = Math.max(0, Math.min(1, (now - (l.createdAt ?? now)) / EDGE_GROW_MS))
  const grow = easeOutCubic(p)

  const tipX = s.x + dx * grow
  const tipY = s.y + dy * grow

  // ① 底层淡线：完整长度、低透明度，提示目标方向
  ctx.beginPath()
  ctx.moveTo(s.x, s.y)
  ctx.lineTo(t.x, t.y)
  ctx.strokeStyle = 'rgba(255,255,255,0.08)'
  ctx.lineWidth = 1.5 * scale
  ctx.stroke()

  // ② 已生长段：从源到当前尖端
  ctx.beginPath()
  ctx.moveTo(s.x, s.y)
  ctx.lineTo(tipX, tipY)
  ctx.strokeStyle = 'rgba(96,165,250,0.7)'
  ctx.lineWidth = 2 * scale
  ctx.stroke()

  // ③ 尖端脉冲发光点（生长过程中的"头"）
  const pulse = 1 + 0.6 * Math.sin(now / 60)
  ctx.beginPath()
  ctx.arc(tipX, tipY, 3.5 * pulse * scale, 0, 2 * Math.PI)
  ctx.fillStyle = 'rgba(147,197,253,0.95)'
  ctx.fill()

  // ④ 悬停节点时，显示与其相连的边的关系标签
  if (grow >= 1 && hoveredNode && (s.id === hoveredNode.id || t.id === hoveredNode.id)) {
    const mx = (s.x + t.x) / 2, my = (s.y + t.y) / 2
    ctx.font = `${9 * scale}px system-ui, sans-serif`
    ctx.textAlign = 'center'
    ctx.textBaseline = 'bottom'
    ctx.fillStyle = 'rgba(156,163,175,0.85)'
    ctx.fillText(prettifyRelation(l.relationType), mx, my - 2 * scale)
  }
}

interface Props {
  graphData: KGData
  entityColors?: Record<string, string>
  refreshKey?: number
  currentPhase?: KGPhase
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
let focusNodeId: string | null = null

// ── 动画循环 ──
let animUntil = 0
let animRaf: number | null = null
let phasePulseAt = 0

function kickAnimation(ms: number) {
  animUntil = Math.max(animUntil, performance.now() + ms)
  if (animRaf == null) tickAnim()
}
function tickAnim() {
  if (graph) graph.refresh()
  if (performance.now() < animUntil) animRaf = requestAnimationFrame(tickAnim)
  else animRaf = null
}

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
    const prevLinks = graph.graphData().links
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

    // 3.5 deep_dive：新节点从现有质心向外辐射（近似聚焦），并标记焦点节点
    const newCount = currNodes.length - prevNodes.length
    if (props.currentPhase === 'deep_dive' && newCount > 0) {
      let cx = 0, cy = 0, cnt = 0
      for (const n of currNodes) {
        if (posMap.has(n.id)) { cx += n.x; cy += n.y; cnt++ }
      }
      if (cnt > 0) { cx /= cnt; cy /= cnt }
      let latest: any = null
      for (const n of currNodes) {
        if (!posMap.has(n.id)) {
          const ang = Math.random() * Math.PI * 2
          const rad = 24 + Math.random() * 48
          n.fx = cx + Math.cos(ang) * rad
          n.fy = cy + Math.sin(ang) * rad
          if (!latest || (n.createdAt ?? 0) > (latest.createdAt ?? 0)) latest = n
        }
      }
      if (latest) focusNodeId = latest.id
    }

    // 4. 仅对新节点解冻（力布局收敛后释放）
    const newEdgeCount = graph.graphData().links.length - prevLinks.length
    if (newCount > 0 || newEdgeCount > 0) {
      kickAnimation(Math.max(NODE_APPEAR_MS, EDGE_GROW_MS) + 200)
    }
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

      const now = performance.now()
      const age = now - (node.createdAt ?? now)
      const p = Math.max(0, Math.min(1, age / NODE_APPEAR_MS)) // 夹到 [0,1]，修复负值
      const ease = 1 - Math.pow(1 - p, 3)                       // 透明度用 cubic ease-out
      const pop = easeOutBack(p)                                // 缩放用 back ease-out
      const appearAlpha = ease                                  // 0 → 1 淡入
      const appearScale = 0.1 + 0.9 * pop                       // 0.1 → ~1.09 → 1.0 弹入

      if (dim) ctx.globalAlpha = 0.15 * appearAlpha

      if (!dim) {
        ctx.globalAlpha = appearAlpha                       // 光晕也淡入
        ctx.beginPath()
        ctx.arc(node.x, node.y, (r + 5 * scale) * appearScale, 0, 2 * Math.PI)
        ctx.fillStyle = color + '18'
        ctx.fill()
      }

      ctx.globalAlpha = (dim ? 0.15 : 1) * appearAlpha      // ★ 主圆现在会淡入
      ctx.beginPath()
      ctx.arc(node.x, node.y, r * appearScale, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()

      ctx.globalAlpha = (dim ? 0.15 : 1) * appearAlpha      // ★ 高光同步淡入
      ctx.beginPath()
      ctx.arc(node.x - r * 0.3 * appearScale, node.y - r * 0.3 * appearScale, r * 0.35 * appearScale, 0, 2 * Math.PI)
      ctx.fillStyle = 'rgba(255,255,255,0.3)'
      ctx.fill()

      if (dim) ctx.globalAlpha = 0.2 * appearAlpha

      if (props.currentPhase === 'deep_dive' && node.id === focusNodeId && !dim) {
        const phase = (now / 800) % 1
        const ringR = r * appearScale + phase * 22 * scale
        ctx.globalAlpha = (1 - phase) * 0.6
        ctx.beginPath()
        ctx.arc(node.x, node.y, ringR, 0, 2 * Math.PI)
        ctx.strokeStyle = '#a78bfa'
        ctx.lineWidth = 1.5 * scale
        ctx.stroke()
      }

      if (globalScale > 0.8) {
        let labelScale = 1
        let labelAlpha = 1
        if (phasePulseAt > 0) {
          const lp = Math.min(1, (now - phasePulseAt) / LABEL_PULSE_MS)
          if (lp < 1) {
            labelScale = 1 + 0.5 * (1 - lp)
            labelAlpha = lp
          }
        }
        const fs = 11 * scale * labelScale
        ctx.font = `bold ${fs}px system-ui, sans-serif`
        ctx.textAlign = 'center'
        ctx.textBaseline = 'top'
        ctx.fillStyle = '#fff'
        ctx.globalAlpha = (dim ? 0.2 : 1) * appearAlpha * labelAlpha
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
    .linkCanvasObjectMode(() => 'replace')     // 完全接管链路渲染
    .linkCanvasObject(drawGrowingLink)
    .linkDirectionalParticles(0)               // 关闭默认粒子，由尖端脉冲替代
    .linkLabel((l: any) => l.relationType)
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
  kickAnimation(Math.max(NODE_APPEAR_MS, EDGE_GROW_MS) + 200)   // ★ 首次渲染也触发生长

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

// 阶段切换：zoomToFit 过渡 + 节点标签 pop
watch(
  () => props.currentPhase,
  (nv, ov) => {
    if (nv && nv !== ov) {
      phasePulseAt = performance.now()
      if (graph) graph.zoomToFit(PHASE_ZOOM_MS, 40)
      kickAnimation(PHASE_ZOOM_MS + 300)
    }
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
