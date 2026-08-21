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
    <!-- 自动追踪按钮 -->
    <button
      class="kg-follow-btn"
      :class="{ active: autoFollow }"
      @click="toggleFollow"
      :title="autoFollow ? '自动追踪中（拖拽/缩放时暂停）' : '点击恢复自动追踪'"
    >
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="3"/>
        <path d="M12 2v4M12 18v4M2 12h4M18 12h4"/>
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import ForceGraph from 'force-graph'
import type { GraphNodeView, KGData, KGPhase } from '@/types/knowledge-graph'
import { ENTITY_COLORS } from '@/types/knowledge-graph'

interface Props {
  graphData: KGData
  entityColors?: Record<string, string>
  refreshKey?: number
  startEntityNames?: string[]
  currentPhase?: KGPhase
}

const props = withDefaults(defineProps<Props>(), {
  entityColors: () => ENTITY_COLORS,
  startEntityNames: () => [],
  currentPhase: 'broad_search',
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

/** 图中是否存在矛盾边（用于决定是否持续温热力模拟，驱动闪烁持续重绘） */
function hasContradiction(): boolean {
  return props.graphData.links.some(isContradiction)
}

// ── 节点浮现 / 边生长动画 ──
const NODE_ANIM_MS = 600   // 节点从出现到完全可见的时长
const EDGE_ANIM_MS = 1200  // 边从出现到"生长完成"的时长
const animatingNodeIds = new Set<string>()  // 正在浮现的节点 ID
const animatingEdgeIds = new Set<string>()  // 正在生长的边 ID
let animRafId = 0
let prevNodeIds = new Set<string>()  // 上次 update 时的节点 ID 集合，用于检测新增
let prevEdgeIds = new Set<string>()  // 上次 update 时的边 ID 集合

/** easeOutCubic: 0→1 缓出 */
function easeOutCubic(t: number): number { return 1 - Math.pow(1 - t, 3) }

/** 将 #RRGGBB 转为 rgba(r,g,b,alpha) */
function hexToRgba(hex: string, alpha: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

/** 启动动画循环：在动画期间保持力模拟活跃以触发重绘 */
function startAnimLoop() {
  let lastWarm = 0
  function tick() {
    // 无浮现/生长动画且无矛盾边时停止温热（避免空转烧 CPU）
    if (animatingNodeIds.size === 0 && animatingEdgeIds.size === 0 && !hasContradiction()) return
    const now = Date.now()
    // 每200ms 检查一次，若力模拟已冷却则重新温热，保持持续重绘
    // （驱动节点浮现/边生长动画推进，以及矛盾边的 600ms 闪烁）
    if (graph && now - lastWarm > 200) {
      try {
        graph.d3ReheatSimulation?.(0.05)
      } catch { /* d3ReheatSimulation 不存在时静默忽略 */ }
      lastWarm = now
    }
    animRafId = requestAnimationFrame(tick)
  }
  cancelAnimationFrame(animRafId)
  animRafId = requestAnimationFrame(tick)
}

// ── 起始节点标签（多标签 HTML 覆盖层）──
const startLabels = ref<Array<{ name: string; x: number; y: number; text: string }>>([])
let labelRafId = 0
function updateStartLabels() {
  if (!graph || !containerRef.value) { labelRafId = requestAnimationFrame(updateStartLabels); return }
  const gNodes = graph.graphData().nodes
  const names = props.startEntityNames
  
  const result: Array<{ name: string; x: number; y: number; text: string }> = []
  
  // 优先显示 startEntityNames 中的节点
  if (names.length > 0) {
    for (const name of names) {
      const node = gNodes.find((n: any) => n.name === name)
      if (node && node.x != null && node.y != null) {
        const pos = graph.graph2ScreenCoords(node.x, node.y)
        result.push({ name: node.name, x: pos.x, y: pos.y - 18, text: node.name })
      }
    }
  } else if (gNodes.length > 0) {
    // 按置信度从高到低排序，显示前15%的节点标签
    const sortedNodes = [...gNodes]
      .filter(n => n.x != null && n.y != null)
      .sort((a, b) => {
        const confA = (a as any).confidence ?? 0.5
        const confB = (b as any).confidence ?? 0.5
        return confB - confA
      })
    const topCount = Math.max(1, Math.ceil(sortedNodes.length * 0.15))
    const nodesToShow = sortedNodes.slice(0, topCount)
    for (const node of nodesToShow) {
      const pos = graph.graph2ScreenCoords(node.x!, node.y!)
      result.push({ name: node.name, x: pos.x, y: pos.y - 18, text: node.name })
    }
  }
  
  startLabels.value = result
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

// ── 自动追踪新节点 ──
const autoFollow = ref(true)
let lastPanTime = 0
const PAN_THROTTLE_MS = 1000 // 两次自动平移的最小间隔

function toggleFollow() { autoFollow.value = !autoFollow.value }

/** 将视口平滑平移到指定坐标列表的中心（延迟读取，等力模拟定位） */
function panToPositions(positions: Array<{x: number, y: number}>) {
  if (!graph || !autoFollow.value || positions.length === 0) return
  const now = Date.now()
  if (now - lastPanTime < PAN_THROTTLE_MS) return
  lastPanTime = now
  // 节点少时缩小视口（给排斥力留容错空间），节点多时正常追踪
  const nodeCount = graph.graphData()?.nodes?.length ?? 0
  const zoomOut = nodeCount < 5 ? 0.6 : nodeCount < 10 ? 0.8 : 1.0
  setTimeout(() => {
    if (!graph) return
    let cx = 0, cy = 0
    for (const p of positions) { cx += p.x; cy += p.y }
    cx /= positions.length; cy /= positions.length
    // 如果需要缩小，先缩再移；否则只移
    if (zoomOut < 1) {
      const curZoom = graph.zoom()
      graph.zoom(curZoom * zoomOut, 600)
    }
    graph.centerAt(cx, cy, 800)
    drawMinimap()
  }, 300)
}

/** 适配：将节点数组转为坐标列表传给 panToPositions */
function panToNewNodes(newNodes: GraphNodeView[]) {
  const positions = newNodes.filter(n => n.x != null && n.y != null).map(n => ({ x: n.x!, y: n.y! }))
  panToPositions(positions)
}

/** 适配：将新边列表转为坐标列表传给 panToPositions */
function panToNewEdges(newEdges: { source: any; target: any }[]) {
  const positions: Array<{x: number, y: number}> = []
  for (const e of newEdges) {
    const s = typeof e.source === 'object' ? e.source : null
    const t = typeof e.target === 'object' ? e.target : null
    if (s?.x != null && t?.x != null) {
      positions.push({ x: (s.x + t.x) / 2, y: (s.y + t.y) / 2 })
    }
  }
  panToPositions(positions)
}

/** 图谱生长完成后，缩放展示全貌并关闭追踪 */
function fitGraphToView() {
  if (!graph) return
  autoFollow.value = false
  setTimeout(() => {
    if (!graph) return
    const gData = graph.graphData()
    if (!gData?.nodes?.length) return
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
    for (const n of gData.nodes) {
      const x = n.x, y = n.y
      if (x != null && isFinite(x) && y != null && isFinite(y)) {
        if (x < minX) minX = x; if (x > maxX) maxX = x
        if (y < minY) minY = y; if (y > maxY) maxY = y
      }
    }
    if (minX === Infinity) return
    const pad = 60
    const gw = maxX - minX + pad * 2
    const gh = maxY - minY + pad * 2
    const cw = containerRef.value?.clientWidth ?? 300
    const ch = containerRef.value?.clientHeight ?? 300
    let k = Math.min(cw / gw, ch / gh)
    k = Math.min(k, 1.5); k = Math.max(k, 0.1)
    const cx = (minX + maxX) / 2, cy = (minY + maxY) / 2
    // 直接通过 graph API 设 zoom（无动画）
    graph.zoom(k, 0)
    // 等 2 帧让 d3-zoom 内部 transform 完全同步后再 centerAt
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (!graph) return
        graph.centerAt(cx, cy, 1200)
        drawMinimap()
      })
    })
  }, 2000)
}

// ── 防抖更新：只在数据真正变化时才调用 graphData，避免拖拽时重置力模拟 ──
let updatePending = false
let lastNodeCount = 0
let lastLinkCount = 0
let hasRecentered = false // 是否已做首次居中（只在首批节点到达后执行一次）
function scheduleUpdate(data: KGData) {
  if (updatePending) return
  // 只有节点数或连线数变化时才需要更新 force-graph
  if (data.nodes.length === lastNodeCount && data.links.length === lastLinkCount) return
  updatePending = true

  // ── 检测新增节点/边，注册动画（先收集新增 id，再覆盖 prev 集合）──
  let hasNew = false
  const addedNodeIds = new Set<string>()
  const addedEdgeIds = new Set<string>()
  for (const n of data.nodes) {
    if (!prevNodeIds.has(n.id)) {
      addedNodeIds.add(n.id)
      animatingNodeIds.add(n.id)
      hasNew = true
      setTimeout(() => {
        animatingNodeIds.delete(n.id)
        if (animatingNodeIds.size === 0 && animatingEdgeIds.size === 0) cancelAnimationFrame(animRafId)
      }, NODE_ANIM_MS + 50) // +50ms 余量
    }
  }
  for (const l of data.links) {
    if (!prevEdgeIds.has(l.id)) {
      addedEdgeIds.add(l.id)
      animatingEdgeIds.add(l.id)
      hasNew = true
      setTimeout(() => {
        animatingEdgeIds.delete(l.id)
        if (animatingNodeIds.size === 0 && animatingEdgeIds.size === 0) cancelAnimationFrame(animRafId)
      }, EDGE_ANIM_MS + 50)
    }
  }
  prevNodeIds = new Set(data.nodes.map(n => n.id))
  prevEdgeIds = new Set(data.links.map(l => l.id))
  if (hasNew) startAnimLoop()

  // ── 自动追踪新节点/新边 ──
  if (hasNew) {
    const newNodeList = data.nodes.filter(n => addedNodeIds.has(n.id))
    panToNewNodes(newNodeList)
    // 新边也触发追踪（边的两端中点）
    const newEdgeList = data.links.filter(l => addedEdgeIds.has(l.id))
    if (newEdgeList.length > 0) panToNewEdges(newEdgeList)
  }

  requestAnimationFrame(() => {
    updatePending = false
    if (!graph) return
    lastNodeCount = data.nodes.length
    lastLinkCount = data.links.length
    graph.graphData(data)

    // 首批节点到达后，居中视口到节点实际中心（修正小地图视口矩形位置）
    if (!hasRecentered && data.nodes.length > 0) {
      hasRecentered = true
      let cx = 0, cy = 0, cnt = 0
      for (const n of data.nodes) {
        if (n.x != null && n.y != null) { cx += n.x; cy += n.y; cnt++ }
      }
      if (cnt > 0) {
        cx /= cnt; cy /= cnt
        graph.centerAt(cx, cy, 400) // 400ms 平滑过渡到中心
      }
    }
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

  // 绘制当前视口矩形（蓝色框 = 主画布可见区域）
  if (containerRef.value) {
    const cw = containerRef.value.clientWidth
    const ch = containerRef.value.clientHeight
    // 用 screen2GraphCoords 转换屏幕四角→图谱坐标，最可靠
    const tl_g = graph.screen2GraphCoords(0, 0)
    const br_g = graph.screen2GraphCoords(cw, ch)
    const tl = toMM(tl_g.x, tl_g.y)
    const br = toMM(br_g.x, br_g.y)
    // 裁剪到小地图画布范围内
    const x1 = Math.max(0, Math.min(MM_W, tl.x))
    const y1 = Math.max(0, Math.min(MM_H, tl.y))
    const x2 = Math.max(0, Math.min(MM_W, br.x))
    const y2 = Math.max(0, Math.min(MM_H, br.y))
    if (x2 > x1 && y2 > y1) {
      ctx.fillStyle = 'rgba(96,165,250,0.12)'
      ctx.fillRect(x1, y1, x2 - x1, y2 - y1)
      ctx.strokeStyle = 'rgba(96,165,250,0.8)'
      ctx.lineWidth = 1.5
      ctx.strokeRect(x1, y1, x2 - x1, y2 - y1)
    }
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
    // 节点值：force-graph 用 sqrt(nodeVal) 作为 hover 命中区域半径，144 → 12px
    .nodeVal(() => 144)
    // 自定义节点绘制：光圈套实心点 + 发光（照搬 obsidian graph 风格）
    .nodeCanvasObject((n: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const base = props.entityColors[n.entityType] || '#888'
      // status 区分：planned 灰色 / searching 半透明 / confirmed 实色
      let color = base
      let statusAlpha = 1
      if (n.status === 'planned') { color = '#94A3B8'; statusAlpha = 0.5 }
      else if (n.status === 'searching') { statusAlpha = 0.6 }

      // 节点浮现动画：透明度 + 半径从小到大缓出
      let progress = 1
      if (animatingNodeIds.has(n.id) && n._createdAt) {
        progress = Math.min(1, (Date.now() - n._createdAt) / NODE_ANIM_MS)
      }
      const animAlpha = easeOutCubic(progress)
      const animScale = 0.3 + 0.7 * easeOutCubic(progress)
      // hover 非相邻节点淡出
      const dimAlpha = isDimmed(n) ? 0.18 : 1
      const alpha = animAlpha * dimAlpha * statusAlpha

      const isHover = hoveredNode && hoveredNode.id === n.id
      const r = (12 * (isHover ? 1.15 : 1) * animScale) / globalScale

      // 外圈：半透明填充 + 实色描边 + 发光
      ctx.save()
      ctx.shadowColor = color
      ctx.shadowBlur = 8 / globalScale
      ctx.beginPath()
      ctx.fillStyle = hexToRgba(color, 0.13 * alpha)
      ctx.strokeStyle = hexToRgba(color, alpha)
      ctx.lineWidth = 2 / globalScale
      ctx.arc(n.x, n.y, r, 0, Math.PI * 2)
      ctx.fill()
      ctx.stroke()
      ctx.restore()

      // 内点：实心小圆点（半径 = 外圈 0.3 倍）
      ctx.beginPath()
      ctx.fillStyle = hexToRgba(color, 0.85 * alpha)
      ctx.arc(n.x, n.y, r * 0.3, 0, Math.PI * 2)
      ctx.fill()
    })
    .nodeLabel(() => '')
    // 链接样式：矛盾连线红色虚线 + 闪烁，hover 时高亮关联连线，宽度反映证据强度
    .linkWidth((l: any) => {
      if (isContradiction(l)) return 2
      // 根据 strength (0..1) 计算边宽度，增强 0.8-0.9 区间的差异
      const rawStrength = typeof l.strength === 'number' ? l.strength : 0.5
      // 使用幂函数增强差异：(x - 0.5) * 2 将 [0.5, 1] 映射到 [0, 1]，再平方增强
      const normalized = Math.pow(Math.max(0, (rawStrength - 0.5) * 2), 2)
      const baseWidth = 0.8 + normalized * 3.2 // 0.8..4
      if (!hoveredNode) return baseWidth
      const s = typeof l.source === 'object' ? (l.source as any).id : l.source
      const t = typeof l.target === 'object' ? (l.target as any).id : l.target
      if (s === hoveredNode.id || t === hoveredNode.id) {
        // 高亮时根据 strength 显示不同宽度：强度越高越粗
        return 0.8 + normalized * 3.2 // 保持与基础宽度一致
      }
      return 0.5 // 非关联边变细
    })
    .linkColor((l: any) => {
      if (isContradiction(l)) {
        return blinkOn.value ? 'rgba(239,68,68,0.8)' : 'rgba(239,68,68,0.25)'
      }
      // 根据 strength 调整透明度（基础量级对齐 obsidian 的 ~0.5）
      const rawStrength = typeof l.strength === 'number' ? l.strength : 0.5
      const normalized = Math.pow(Math.max(0, (rawStrength - 0.5) * 2), 2)
      const opacity = 0.15 + normalized * 0.45 // 0.15..0.6
      // 冷灰蓝边（近 obsidian 的 #aaa，带一点蓝）
      if (!hoveredNode) return `rgba(160,175,200,${opacity})`
      const s = typeof l.source === 'object' ? (l.source as any).id : l.source
      const t = typeof l.target === 'object' ? (l.target as any).id : l.target
      if (s === hoveredNode.id || t === hoveredNode.id) {
        // 相邻边高亮到 0.9（obsidian 焦点值）
        return 'rgba(160,190,255,0.9)'
      }
      // 非相邻边淡出到 0.08（obsidian 焦点值）
      return 'rgba(160,175,200,0.08)'
    })
    .linkLineDash((l: any) => isContradiction(l) ? [6, 4] : null)
    .linkDirectionalParticles((l: any) => {
      if (isContradiction(l)) return 3
      // 边生长动画：新边用更多粒子模拟流动生长
      if (animatingEdgeIds.has(l.id)) return 5
      // 阶段差异化：广度检索更活跃，深度挖掘更沉稳
      return PHASE_FORCE_CONFIG[props.currentPhase]?.particleCount ?? 1
    })
    .linkDirectionalParticleWidth((l: any) => isContradiction(l) ? 2.5 : 1.5)
    .linkDirectionalParticleSpeed((l: any) => {
      if (isContradiction(l)) return 0.004
      // 边生长动画：新边粒子速度更快
      if (animatingEdgeIds.has(l.id)) return 0.018
      // 阶段差异化
      return PHASE_FORCE_CONFIG[props.currentPhase]?.particleSpeed ?? 0.004
    })
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
    .onNodeDrag(() => { autoFollow.value = false }) // 拖拽时暂停追踪
    .onNodeDragEnd(() => { autoFollow.value = true }) // 释放节点后立即恢复
    .enableNodeDrag(true)
    .d3AlphaDecay(0.02)
    .d3VelocityDecay(0.4)
    .warmupTicks(30)
    .cooldownTicks(100)
    .cooldownTime(3000)

  // 根据初始 phase 设置力布局参数
  applyPhaseForces(props.currentPhase)

  graph.graphData(props.graphData)

  // 初始化动画追踪集合（初始数据不需要动画）
  prevNodeIds = new Set(props.graphData.nodes.map(n => n.id))
  prevEdgeIds = new Set(props.graphData.links.map(l => l.id))
  lastNodeCount = props.graphData.nodes.length
  lastLinkCount = props.graphData.links.length

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

  // 启动矛盾连线闪烁：翻转亮度；存在矛盾边时重新温热力模拟以触发重绘
  // （不依赖 startAnimLoop，确保历史回放/边属性更新等场景下也能持续闪烁）
  blinkTimer = setInterval(() => {
    blinkOn.value = !blinkOn.value
    if (graph && hasContradiction()) {
      try { graph.d3ReheatSimulation?.(0.05) } catch { /* 静默忽略 */ }
    }
  }, 600)
})

// refreshKey 变化时用防抖 + 冻结策略更新
watch(
  () => props.refreshKey,
  () => {
    if (graph) scheduleUpdate(props.graphData)
  },
)

// 阶段切换时调整力布局参数
watch(
  () => props.currentPhase,
  (phase) => {
    if (phase) applyPhaseForces(phase)
  },
)

// ── 阶段切换缩放脉冲：外部通过 ref 调用 ──
function zoomPulse() {
  if (!graph) return
  const currentZoom = graph.zoom()
  graph.zoom(currentZoom * 0.85, 400)   // 缩小
  setTimeout(() => graph.zoom(currentZoom, 500), 420) // 恢复
}

// ── 广度/深度阶段差异化力布局参数 ──
const PHASE_FORCE_CONFIG = {
  broad_search: {
    chargeStrength: -400,    // 强排斥 → 节点散开
    linkDistance: 250,        // 远距离 → 大范围扫描感
    alphaDecay: 0.015,       // 慢冷却 → 持续运动
    particleCount: 2,        // 更多粒子 → 活跃流动感
    particleSpeed: 0.006,    // 更快粒子
  },
  deep_dive: {
    chargeStrength: -120,    // 弱排斥 → 节点紧凑聚焦
    linkDistance: 120,        // 短距离 → 聚焦深入感
    alphaDecay: 0.03,        // 快冷却 → 快速稳定
    particleCount: 1,        // 更少粒子 → 沉稳
    particleSpeed: 0.003,    // 更慢粒子
  },
}

function applyPhaseForces(phase: KGPhase) {
  if (!graph) return
  const cfg = PHASE_FORCE_CONFIG[phase] ?? PHASE_FORCE_CONFIG.broad_search
  graph.d3Force('charge')?.strength(cfg.chargeStrength)
  graph.d3Force('link')?.distance(cfg.linkDistance)
  // 通过 d3AlphaDecay 调整冷却速度；重新加热力模拟让节点响应新参数
  graph.d3AlphaDecay(cfg.alphaDecay)
  try {
    // force-graph 内部的 d3-force simulation
    const sim = (graph as any).d3Force?.()
    if (sim && typeof sim.alpha === 'function') {
      sim.alpha(0.3).restart()
    }
  } catch { /* 静默忽略 */ }
}

defineExpose({ zoomPulse, fitGraphToView })

onUnmounted(() => {
  cancelAnimationFrame(labelRafId)
  cancelAnimationFrame(animRafId)
  if (blinkTimer) clearInterval(blinkTimer)
  animatingNodeIds.clear()
  animatingEdgeIds.clear()
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
  font-size: 12px;
  font-weight: 600;
  color: #e8edf5;
  background: rgba(17, 24, 39, 0.82);
  border: 1px solid rgba(255, 255, 255, 0.06);
  padding: 2px 8px;
  border-radius: 6px;
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

.kg-follow-btn {
  position: absolute;
  bottom: 116px;
  right: 8px;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: rgba(15, 15, 35, 0.75);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}
.kg-follow-btn:hover {
  background: rgba(30, 30, 60, 0.85);
  color: rgba(255, 255, 255, 0.7);
}
.kg-follow-btn.active {
  color: #60a5fa;
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(30, 30, 60, 0.85);
}
</style>
