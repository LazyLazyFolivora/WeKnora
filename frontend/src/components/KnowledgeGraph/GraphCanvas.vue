<template>
  <div class="graph-canvas">
    <div ref="containerRef" class="graph-canvas-inner" />
    <template v-for="label in startLabels" :key="label.name">
      <div class="kg-start-label" :style="{ left: label.x + 'px', top: label.y + 'px' }">
        {{ label.text }}
      </div>
    </template>
    <!-- 小地图 -->
    <canvas
      ref="minimapRef"
      class="kg-minimap"
      width="150"
      height="100"
      @click="onMinimapClick"
    />
  </div>
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
  startEntityNames?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  entityColors: () => ENTITY_COLORS,
  startEntityNames: () => [],
})

const emit = defineEmits<{
  nodeClick: [node: GraphNodeView]
  nodeHover: [node: GraphNodeView | null]
}>()

const containerRef = ref<HTMLDivElement>()
const minimapRef = ref<HTMLCanvasElement>()
let graph: any = null
let hoveredNode: GraphNodeView | null = null

// ── 矛盾发现闪烁：600ms 周期交替明暗 ──
const blinkOn = ref(true)
let blinkTimer: ReturnType<typeof setInterval> | null = null

// ── 辅助：判断连线是否矛盾（force-graph 会将 source/target 解析为对象）──
function isContradiction(l: any): boolean {
  return l.contradiction === true || (l as any).is_contradiction === true
}

// ── 起始节点标签（多标签 HTML 覆盖层）──
const startLabels = ref<Array<{ name: string; x: number; y: number; text: string }>>([])
let labelRafId = 0
function updateStartLabels() {
  if (!graph || !containerRef.value) { labelRafId = requestAnimationFrame(updateStartLabels); return }
  const gNodes = graph.graphData().nodes
  const names = props.startEntityNames
  // 标签：只有 EntityPlanned 事件提供的起始实体才显示
  if (names.length === 0) { startLabels.value = [] }
  else {
    const result: Array<{ name: string; x: number; y: number; text: string }> = []
    for (const name of names) {
      const node = gNodes.find((n: any) => n.name === name)
      if (node && node.x != null && node.y != null) {
        const pos = graph.graph2ScreenCoords(node.x, node.y)
        result.push({ name: node.name, x: pos.x, y: pos.y - 18, text: node.name })
      }
    }
    startLabels.value = result
  }
  // 小地图每帧都画，不受标签影响
  drawMinimap()
  labelRafId = requestAnimationFrame(updateStartLabels)
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

// ── 防抖更新：只在数据真正变化时才调用 graphData，避免拖拽时重置力模拟 ──
let updatePending = false
let lastNodeCount = 0
let lastLinkCount = 0
function scheduleUpdate(data: KGData) {
  if (updatePending) return
  // 只有节点数或连线数变化时才需要更新 force-graph
  if (data.nodes.length === lastNodeCount && data.links.length === lastLinkCount) return
  updatePending = true
  requestAnimationFrame(() => {
    updatePending = false
    if (!graph) return
    lastNodeCount = data.nodes.length
    lastLinkCount = data.links.length
    graph.graphData(data)
  })
}

// ── 小地图 ──
const MM_W = 150
const MM_H = 100
const MM_PAD = 10 // 内边距

function drawMinimap() {
  if (!graph || !minimapRef.value) return
  const ctx = minimapRef.value.getContext('2d')
  if (!ctx) return
  const nodes = graph.graphData().nodes
  if (nodes.length === 0) { ctx.clearRect(0, 0, MM_W, MM_H); return }

  ctx.clearRect(0, 0, MM_W, MM_H)

  // 计算所有节点的包围盒
  let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
  for (const n of nodes) {
    if (n.x == null || n.y == null) continue
    if (n.x < minX) minX = n.x
    if (n.x > maxX) maxX = n.x
    if (n.y < minY) minY = n.y
    if (n.y > maxY) maxY = n.y
  }
  if (!isFinite(minX)) return

  // 扩展包围盒，留出边距
  const pad = 50
  minX -= pad; maxX += pad; minY -= pad; maxY += pad
  const gw = maxX - minX || 1
  const gh = maxY - minY || 1
  const scale = Math.min((MM_W - MM_PAD * 2) / gw, (MM_H - MM_PAD * 2) / gh)
  const ox = MM_PAD + ((MM_W - MM_PAD * 2) - gw * scale) / 2
  const oy = MM_PAD + ((MM_H - MM_PAD * 2) - gh * scale) / 2

  function toMM(gx: number, gy: number) {
    return { x: ox + (gx - minX) * scale, y: oy + (gy - minY) * scale }
  }

  // 绘制连线（极淡）
  const links = graph.graphData().links
  ctx.strokeStyle = 'rgba(255,255,255,0.06)'
  ctx.lineWidth = 0.5
  for (const l of links) {
    const s = typeof l.source === 'object' ? l.source : nodes.find((n: any) => n.id === l.source)
    const t = typeof l.target === 'object' ? l.target : nodes.find((n: any) => n.id === l.target)
    if (!s || !t || s.x == null || t.x == null) continue
    const a = toMM(s.x, s.y)
    const b = toMM(t.x, t.y)
    ctx.beginPath()
    ctx.moveTo(a.x, a.y)
    ctx.lineTo(b.x, b.y)
    ctx.stroke()
  }

  // 绘制节点
  for (const n of nodes) {
    if (n.x == null || n.y == null) continue
    const p = toMM(n.x, n.y)
    const color = props.entityColors[n.entityType] || '#888'
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.arc(p.x, p.y, 2, 0, Math.PI * 2)
    ctx.fill()
  }

  // 绘制当前视口矩形
  if (containerRef.value && nodes.length > 0) {
    const el = containerRef.value
    const cw = el.clientWidth
    const ch = el.clientHeight
    const zoom = graph.zoom()
    // 以 graph center 为基准绘制视口矩形
    const center = graph.centerAt()
    const vl = center.x - cw / (2 * zoom)
    const vt = center.y - ch / (2 * zoom)
    const vr = center.x + cw / (2 * zoom)
    const vb = center.y + ch / (2 * zoom)
    const tl = toMM(vl, vt)
    const br = toMM(vr, vb)
    ctx.strokeStyle = 'rgba(96,165,250,0.7)'
    ctx.lineWidth = 1.5
    ctx.strokeRect(tl.x, tl.y, br.x - tl.x, br.y - tl.y)
  }
}

function onMinimapClick(e: MouseEvent) {
  if (!graph || !minimapRef.value || !containerRef.value) return
  const rect = minimapRef.value.getBoundingClientRect()
  const mx = e.clientX - rect.left
  const my = e.clientY - rect.top

  const nodes = graph.graphData().nodes
  if (nodes.length === 0) return

  // 与 drawMinimap 相同的坐标映射
  let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
  for (const n of nodes) {
    if (n.x == null || n.y == null) continue
    if (n.x < minX) minX = n.x
    if (n.x > maxX) maxX = n.x
    if (n.y < minY) minY = n.y
    if (n.y > maxY) maxY = n.y
  }
  if (!isFinite(minX)) return
  const pad = 50
  minX -= pad; maxX += pad; minY -= pad; maxY += pad
  const gw = maxX - minX || 1
  const gh = maxY - minY || 1
  const scale = Math.min((MM_W - MM_PAD * 2) / gw, (MM_H - MM_PAD * 2) / gh)
  const ox = MM_PAD + ((MM_W - MM_PAD * 2) - gw * scale) / 2
  const oy = MM_PAD + ((MM_H - MM_PAD * 2) - gh * scale) / 2

  // 反算图谱坐标
  const gx = (mx - ox) / scale + minX
  const gy = (my - oy) / scale + minY
  graph.centerAt(gx, gy, 400) // 400ms 平滑过渡
}

onMounted(() => {
  if (!containerRef.value) return

  graph = ForceGraph()(containerRef.value)
    .backgroundColor('rgba(15, 15, 35, 0.4)')
    .nodeVal(12)
    // 内置渲染：nodeColor 按 status 区分视觉
    .nodeColor((n: any) => {
      const base = props.entityColors[n.entityType] || '#888'
      if (isDimmed(n)) return base + '33'
      if (n.status === 'planned') return '#6b7280'     // 灰色：预生成
      if (n.status === 'searching') return base + 'aa'  // 淡色：搜索中
      return base                                        // 实色：已确认
    })
    .nodeLabel(() => '')
    // 链接样式：矛盾连线红色虚线 + 闪烁，hover 时高亮关联连线
    .linkWidth((l: any) => {
      if (isContradiction(l)) return 2
      if (!hoveredNode) return 1.5
      const s = typeof l.source === 'object' ? (l.source as any).id : l.source
      const t = typeof l.target === 'object' ? (l.target as any).id : l.target
      return (s === hoveredNode.id || t === hoveredNode.id) ? 2.5 : 0.5
    })
    .linkColor((l: any) => {
      if (isContradiction(l)) {
        return blinkOn.value ? 'rgba(239,68,68,0.8)' : 'rgba(239,68,68,0.25)'
      }
      if (!hoveredNode) return 'rgba(255,255,255,0.12)'
      const s = typeof l.source === 'object' ? (l.source as any).id : l.source
      const t = typeof l.target === 'object' ? (l.target as any).id : l.target
      return (s === hoveredNode.id || t === hoveredNode.id)
        ? 'rgba(96,165,250,0.6)'
        : 'rgba(255,255,255,0.04)'
    })
    .linkLineDash((l: any) => isContradiction(l) ? [6, 4] : null)
    .linkDirectionalParticles((l: any) => isContradiction(l) ? 3 : 1)
    .linkDirectionalParticleWidth((l: any) => isContradiction(l) ? 2.5 : 1.5)
    .linkDirectionalParticleSpeed(0.004)
    .linkDirectionalParticleColor((l: any) => isContradiction(l)
      ? 'rgba(239,68,68,0.7)'
      : 'rgba(96,165,250,0.5)')
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

  // 居中到节点实际中心（而非 0,0），避免力模拟偏移导致小地图不对齐
  const gNodes = props.graphData.nodes
  if (gNodes.length > 0) {
    let cx = 0, cy = 0
    for (const n of gNodes) { if (n.x != null) cx += n.x; if (n.y != null) cy += n.y }
    cx /= gNodes.length; cy /= gNodes.length
    graph.centerAt(cx, cy, 0)
  } else {
    graph.centerAt(0, 0, 0)
  }
  graph.zoom(1.2, 0)

  // 启动起始节点标签跟踪
  labelRafId = requestAnimationFrame(updateStartLabels)

  // 启动矛盾连线闪烁
  blinkTimer = setInterval(() => { blinkOn.value = !blinkOn.value }, 600)
})

// refreshKey 变化时用防抖 + 冻结策略更新
watch(
  () => props.refreshKey,
  () => {
    if (graph) scheduleUpdate(props.graphData)
  },
)

onUnmounted(() => {
  cancelAnimationFrame(labelRafId)
  if (blinkTimer) clearInterval(blinkTimer)
  if (graph) graph._destructor?.()
})
</script>

<style scoped>
.graph-canvas {
  width: 100%;
  height: 100%;
  min-height: 300px;
  position: relative;
}

.graph-canvas-inner {
  width: 100%;
  height: 100%;
}

.kg-start-label {
  position: absolute;
  transform: translate(-50%, -100%);
  pointer-events: none;
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.6), 0 0 8px rgba(0, 0, 0, 0.3);
  white-space: nowrap;
  z-index: 5;
}

.kg-minimap {
  position: absolute;
  bottom: 8px;
  right: 8px;
  width: 150px;
  height: 100px;
  border-radius: 6px;
  background: rgba(15, 15, 35, 0.75);
  border: 1px solid rgba(255, 255, 255, 0.08);
  cursor: pointer;
  z-index: 10;
}
</style>
