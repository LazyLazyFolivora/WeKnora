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
import { ref, onMounted, watch, nextTick, onUnmounted } from 'vue'
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
let resizeObserver: ResizeObserver | null = null
let initialFitDone = false // 容器首次可见后只执行一次 fitToView
let lastFitWidth = 0 // 上次 fitToView 时的容器宽度，用于检测显著尺寸变化

// ── Bug fix: 阻止 wheel 事件冒泡，防止缩放到极限时页面滚动 ──
function onContainerWheel(e: WheelEvent) {
  e.preventDefault()
}

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
const NODE_ANIM_MS = 900   // 节点从出现到完全可见的时长（配合 5-10s 随机消费节奏，渐入更从容）
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

  // hover 时只显示被点亮节点自己的标签
  if (hoveredNode) {
    const node = gNodes.find((n: any) => n.id === hoveredNode!.id)
    if (node && node.x != null && node.y != null) {
      const pos = graph.graph2ScreenCoords(node.x, node.y)
      result.push({ name: node.name, x: pos.x, y: pos.y - 18, text: node.name })
    }
  } else if (names.length > 0) {
    // 无 hover：显示所有起始节点标签
    for (const name of names) {
      const node = gNodes.find((n: any) => n.name === name)
      if (node && node.x != null && node.y != null) {
        const pos = graph.graph2ScreenCoords(node.x, node.y)
        result.push({ name: node.name, x: pos.x, y: pos.y - 18, text: node.name })
      }
    }
  } else if (gNodes.length > 0) {
    // 降级：按置信度从高到低排序，显示前15%的节点标签
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
const FIT_OVERSCALE = 1.15 // 正常大小：图比可视区稍大（整体溢出倍数）
const MIN_ZOOM_FACTOR = 0.2 // 缩小下限：正常大小的五分之一
const MAX_ZOOM_FACTOR = 5 // 放大上限：正常大小的 5 倍
const PAN_MARGIN_FACTOR = 0.5 // 平移边界：视图中心最多超出图包围盒半个视口（与缩小下限对应）
// 待对焦的新节点（对象引用）：力模拟稳定后读实时坐标精确对准，避免用初始位置快照导致偏焦
let pendingFocus: GraphNodeView[] = []

/** 让整图处于「正常大小」（稍大于可视区）并居中，同时设缩小下限，避免缩成一小点 */
function fitToView(duration = 600) {
  if (!graph) return
  const gData = graph.graphData()
  if (!gData?.nodes?.length) return
  const bbox = graph.getGraphBbox()
  if (!bbox) return
  const gw = (bbox.x[1] - bbox.x[0]) || 1
  const gh = (bbox.y[1] - bbox.y[0]) || 1
  const cw = containerRef.value?.clientWidth ?? 300
  const ch = containerRef.value?.clientHeight ?? 300
  const k = Math.min(cw / gw, ch / gh) * FIT_OVERSCALE
  const cx = (bbox.x[0] + bbox.x[1]) / 2
  const cy = (bbox.y[0] + bbox.y[1]) / 2
  graph.minZoom(k * MIN_ZOOM_FACTOR)
  graph.maxZoom(k * MAX_ZOOM_FACTOR)
  graph.centerAt(cx, cy, duration)
  graph.zoom(k, duration)
  drawMinimap()
}

// ── 平移边界：视图中心最多超出图包围盒半个视口，防止把图整体拖出屏幕（zoom 由 minZoom/maxZoom 限制，这里限制 centerAt 平移）──
// 调试日志用：安全格式化数值（undefined → ?），打印纯文本而非结构体，方便复制
const num = (v: any, d = 1) => (typeof v === 'number' && isFinite(v) ? v.toFixed(d) : '?')
let isClampingPan = false
function clampViewPan() {
  if (!graph) return
  const bbox = graph.getGraphBbox()
  if (!bbox) return
  const c = graph.centerAt()
  const zoom = graph.zoom() || 1
  const cw = containerRef.value?.clientWidth ?? 300
  const ch = containerRef.value?.clientHeight ?? 300
  // 半个视口对应的图坐标距离，作为可拖出边界的最大余量（PAN_MARGIN_FACTOR）
  const mx = (cw * PAN_MARGIN_FACTOR) / zoom
  const my = (ch * PAN_MARGIN_FACTOR) / zoom
  const nx = Math.min(bbox.x[1] + mx, Math.max(bbox.x[0] - mx, c.x))
  const ny = Math.min(bbox.y[1] + my, Math.max(bbox.y[0] - my, c.y))
  if (nx !== c.x || ny !== c.y) {
    console.log(`[KG:clampPan] 拉回 from=(${num(c.x)},${num(c.y)}) to=(${num(nx)},${num(ny)}) zoom=${num(zoom, 2)} bbox.x=[${num(bbox.x[0])},${num(bbox.x[1])}] bbox.y=[${num(bbox.y[0])},${num(bbox.y[1])}]`)
    graph.centerAt(nx, ny)
  }
}

function toggleFollow() { autoFollow.value = !autoFollow.value }

/** 对焦：镜头中心对准节点实时质心，仅平移不缩放（避免图谱生长时因包围盒变大导致 zoom 下降＝卡片缩小） */
function focusToNodes(nodes: GraphNodeView[]) {
  if (!graph || !autoFollow.value) return
  // 读取节点「实时」坐标（力模拟稳定后即最终位置），而非初始快照，保证对准不偏焦
  const positions = nodes.filter(n => n.x != null && n.y != null).map(n => ({ x: n.x!, y: n.y! }))
  if (positions.length === 0) return
  let cx = 0, cy = 0
  for (const p of positions) { cx += p.x; cy += p.y }
  cx /= positions.length; cy /= positions.length

  const cw = containerRef.value?.clientWidth ?? 300
  const ch = containerRef.value?.clientHeight ?? 300

  // 仅更新 minZoom/maxZoom 边界（图 bbox 变化后允许用户缩放范围调整），不改变当前缩放级别
  const bbox = graph.getGraphBbox()
  if (bbox) {
    const gw = (bbox.x[1] - bbox.x[0]) || 1
    const gh = (bbox.y[1] - bbox.y[0]) || 1
    const k = Math.min(cw / gw, ch / gh) * FIT_OVERSCALE
    graph.minZoom(k * MIN_ZOOM_FACTOR)
    graph.maxZoom(k * MAX_ZOOM_FACTOR)
  }

  console.log(`[KG:focus] 平移到新节点 ids=${nodes.map(n => n.id).join(',')} 质心=(${num(cx)},${num(cy)}) zoom保持=${num(graph.zoom(), 3)}`)
  // 镜头平滑对准新节点质心，不调用 graph.zoom() 以保持当前缩放
  graph.centerAt(cx, cy, 600)
  const cImmediate = graph.centerAt()
  console.log(`[KG:focus] centerAt后立即 center=(${num(cImmediate.x)},${num(cImmediate.y)}) zoom=${num(graph.zoom(), 3)}`)
  setTimeout(() => {
    if (!graph) return
    const center = graph.centerAt()
    const screenPos = graph.graph2ScreenCoords(cx, cy)
    console.log(`[KG:focus] centerAt后700ms center=(${num(center.x)},${num(center.y)}) zoom=${num(graph.zoom(), 3)} 目标=(${num(cx)},${num(cy)}) 偏差=(${num(center.x - cx, 2)},${num(center.y - cy, 2)}) 节点屏幕=(${num(screenPos.x)},${num(screenPos.y)}) 视口中心=(${num(cw / 2)},${num(ch / 2)}) 画布=${graph.width()}x${graph.height()}`)
  }, 700)
  drawMinimap()
}

/** 图谱生长完成后，缩放展示全貌并关闭追踪 */
function fitGraphToView() {
  if (!graph) return
  autoFollow.value = false
  setTimeout(() => {
    if (!graph) return
    fitToView(800)
  }, 100)
}

// ── 防抖更新：只在数据真正变化时才调用 graphData，避免拖拽时重置力模拟 ──
let updatePending = false
let lastNodeCount = 0
let lastLinkCount = 0
let lastForceNodeCount = 0
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

  // ── 记录待对焦的新节点：等力模拟稳定后（onEngineStop）读实时坐标精确对准 ──
  if (hasNew) {
    pendingFocus = data.nodes.filter(n => addedNodeIds.has(n.id))
  }

  requestAnimationFrame(() => {
    updatePending = false
    if (!graph) return
    lastNodeCount = data.nodes.length
    lastLinkCount = data.links.length
    // 仅节点规模变化时才更新力参数，避免单纯边新增时改变 linkDistance/charge 引发抖动
    if (data.nodes.length !== lastForceNodeCount) {
      lastForceNodeCount = data.nodes.length
      refreshForces(data.nodes.length)
    }
    graph.graphData(data)
    invalidateMinimapBBox()
  })
}

// ── 小地图 ──
const MM_W = 150
const MM_H = 100
const MM_PAD = 10 // 内边距
const MM_BBOX_PAD = 50 // 包围盒外扩边距（图谱坐标）

// 小地图包围盒缓存：只在数据变更 / 力模拟停止时重算，避免逐帧重算导致抖动
let minimapBBox: { minX: number, maxX: number, minY: number, maxY: number } | null = null

/** 从当前图谱节点重算小地图包围盒（含外扩边距），无有效节点时返回 null */
function computeMinimapBBox() {
  const g = graph?.graphData()
  if (!g?.nodes?.length) return null
  let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
  for (const n of g.nodes) {
    if (n.x == null || n.y == null) continue
    if (n.x < minX) minX = n.x
    if (n.x > maxX) maxX = n.x
    if (n.y < minY) minY = n.y
    if (n.y > maxY) maxY = n.y
  }
  if (!isFinite(minX)) return null
  return {
    minX: minX - MM_BBOX_PAD,
    maxX: maxX + MM_BBOX_PAD,
    minY: minY - MM_BBOX_PAD,
    maxY: maxY + MM_BBOX_PAD,
  }
}

/** 取当前有效包围盒（缓存优先，缺失时重算） */
function getMinimapBBox() {
  if (!minimapBBox) minimapBBox = computeMinimapBBox()
  return minimapBBox
}

/** 数据变更 / 力模拟停止后，重算包围盒缓存 */
function invalidateMinimapBBox() {
  minimapBBox = computeMinimapBBox()
}

function drawMinimap() {
  if (!graph || !minimapRef.value) return
  const ctx = minimapRef.value.getContext('2d')
  if (!ctx) return

  ctx.clearRect(0, 0, MM_W, MM_H)

  const bbox = getMinimapBBox()
  if (!bbox) return

  const minX = bbox.minX, maxX = bbox.maxX, minY = bbox.minY, maxY = bbox.maxY
  const gw = maxX - minX || 1
  const gh = maxY - minY || 1
  const scale = Math.min((MM_W - MM_PAD * 2) / gw, (MM_H - MM_PAD * 2) / gh)
  const ox = MM_PAD + ((MM_W - MM_PAD * 2) - gw * scale) / 2
  const oy = MM_PAD + ((MM_H - MM_PAD * 2) - gh * scale) / 2

  function toMM(gx: number, gy: number) {
    return { x: ox + (gx - minX) * scale, y: oy + (gy - minY) * scale }
  }

  const nodes = graph.graphData().nodes

  // 绘制连线（极淡）
  const links = graph.graphData().links
  ctx.strokeStyle = 'rgba(42, 58, 92, 0.6)'
  ctx.lineWidth = 0.5
  for (const l of links) {
    if (l.synthetic) continue
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

  // 绘制当前视口矩形（蓝色框 = 主画布可见区域），用 canvas 裁剪代替逐角 clamp 避免边缘失真
  if (containerRef.value) {
    const cw = containerRef.value.clientWidth
    const ch = containerRef.value.clientHeight
    const tl_g = graph.screen2GraphCoords(0, 0)
    const br_g = graph.screen2GraphCoords(cw, ch)
    const tl = toMM(tl_g.x, tl_g.y)
    const br = toMM(br_g.x, br_g.y)
    const x1 = Math.min(tl.x, br.x)
    const y1 = Math.min(tl.y, br.y)
    const x2 = Math.max(tl.x, br.x)
    const y2 = Math.max(tl.y, br.y)
    ctx.save()
    ctx.beginPath()
    ctx.rect(MM_PAD, MM_PAD, MM_W - MM_PAD * 2, MM_H - MM_PAD * 2)
    ctx.clip()
    ctx.fillStyle = 'rgba(96,165,250,0.12)'
    ctx.fillRect(x1, y1, x2 - x1, y2 - y1)
    ctx.strokeStyle = 'rgba(96,165,250,0.8)'
    ctx.lineWidth = 1.5
    ctx.strokeRect(x1, y1, x2 - x1, y2 - y1)
    ctx.restore()
  }
}

function onMinimapClick(e: MouseEvent) {
  if (!graph || !minimapRef.value || !containerRef.value) return
  const rect = minimapRef.value.getBoundingClientRect()
  const mx = e.clientX - rect.left
  const my = e.clientY - rect.top

  const bbox = getMinimapBBox()
  if (!bbox) return

  const minX = bbox.minX, maxX = bbox.maxX, minY = bbox.minY, maxY = bbox.maxY
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

// 计算图谱可用宽度：不能直接测 .graph-canvas-inner 或 .msg-with-avatar 的当前宽度，
// 它们都是 shrink-to-fit，会随画布宽度反向缩窄，形成闭环（画布窄 → 行窄 → 量出来窄 → 永远涨不回）。
// 改为读行的 max-width（85%，getComputedStyle 已解析成像素）——它来自稳定的 .msg_list，
// 不随内容缩，得到的是行的最大可用宽度，减去头像与 gap 即图谱真实可用宽度。
function measureGraphWidth(): number {
  const el = containerRef.value
  if (!el) return 0
  const row = el.closest('.msg-with-avatar') as HTMLElement | null
  if (row) {
    const cs = getComputedStyle(row)
    const gap = parseFloat(cs.columnGap || cs.gap) || 0
    const avatar = row.querySelector('.msg-avatar') as HTMLElement | null
    const avatarW = avatar ? avatar.offsetWidth : 0
    // 行的最大可用宽度：max-width 可能是 "85%" 或已解析的 "816px"，两种情况都兼容
    const maxW = cs.maxWidth
    const pct = maxW.includes('%') ? parseFloat(maxW) / 100 : 0
    const stableRowW = pct > 0
      ? (row.parentElement?.clientWidth ?? row.clientWidth) * pct
      : (parseFloat(maxW) || row.clientWidth)
    const w = stableRowW - avatarW - gap
    if (w > 0) return w
  }
  return el.getBoundingClientRect().width // 兜底：非聊天场景回落原逻辑
}

// 同步画布尺寸到真实容器：宽度用 measureGraphWidth（稳定祖先），高度用容器 rect。
// 抽成独立函数，供 ResizeObserver 与「首节点出现」双路调用。
function syncGraphSize() {
  if (!graph || !containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  const w = measureGraphWidth()
  const h = rect.height
  // 隐藏/折叠态尺寸为 0 时跳过，避免把画布锁成 300px 导致「宽度变窄」
  if (w <= 0 || h <= 0) return
  graph.width(w)
  graph.height(h)
  if (!initialFitDone) {
    // 首次变为可见：基于真实视口尺寸做一次 fitToView
    initialFitDone = true
    lastFitWidth = w
    setTimeout(() => {
      if (!graph) return
      fitToView(600)
    }, 100) // 短延迟确保 graph.width/height 已生效
  } else if (!autoFollow.value && Math.abs(w - lastFitWidth) > 40) {
    // 宽度显著变化（侧栏/窗口缩放）且非自动追踪中，重新适配全貌
    lastFitWidth = w
    setTimeout(() => {
      if (!graph) return
      fitToView(600)
    }, 100)
  }
}

onMounted(() => {
  if (!containerRef.value) return

  // 显式设置画布尺寸为容器实际大小：force-graph 不显式设置时默认 window.innerWidth/innerHeight，
  // 导致 centerAt 以错误的内尺寸取中，节点视觉上偏离可视区中心（画布内中心 ≠ 容器中心）
  const cw = containerRef.value.clientWidth || 300
  const ch = containerRef.value.clientHeight || 300

  graph = ForceGraph()(containerRef.value)
    .width(cw)
    .height(ch)
    .backgroundColor('rgba(15, 15, 35, 0.4)')
    // 节点值：force-graph 用 sqrt(nodeVal) 作为 hover 命中区域半径，144 → 12px
    .nodeVal(() => 144)
    // 自定义节点绘制：光圈套实心点 + 发光（照搬 obsidian graph 风格）
    .nodeCanvasObject((n: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const base = props.entityColors[n.entityType] || '#888'
      // status 区分：planned 空心灰三角 / searching 半透明 / confirmed 实色发光
      const isPlanned = n.status === 'planned'
      const isSearching = n.status === 'searching'
      const color = isPlanned ? '#94A3B8' : base
      const statusAlpha = isPlanned ? 1 : (isSearching ? 0.6 : 1)

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

      if (isPlanned) {
        // 预生成节点：空心灰色三角形（指向上），不显示实体类型色，与已确认实体的圆形明确区分
        const sin60 = Math.sqrt(3) / 2
        ctx.save()
        ctx.strokeStyle = hexToRgba(color, 0.7 * alpha)
        ctx.lineWidth = 1.5 / globalScale
        ctx.lineJoin = 'round'
        ctx.beginPath()
        ctx.moveTo(n.x, n.y - r)                    // 上顶点
        ctx.lineTo(n.x - r * sin60, n.y + r * 0.5)  // 左下
        ctx.lineTo(n.x + r * sin60, n.y + r * 0.5)  // 右下
        ctx.closePath()
        ctx.stroke()
        ctx.restore()
        return
      }

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
      if (l.synthetic) return 0.4 + (typeof l.strength === 'number' ? l.strength : 0.5) * 1.2
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
      if (l.synthetic) return 'rgba(160,175,200,0.10)'
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
    .linkLineDash((l: any) => l.synthetic ? [2, 4] : (isContradiction(l) ? [6, 4] : null))
    .linkDirectionalParticles((l: any) => {
      if (l.synthetic) return 0
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
    .linkLabel((l: any) => l.synthetic ? '' : l.relationType)
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
    .warmupTicks(0)
    .cooldownTicks(100)
    .cooldownTime(3000)
    .onEngineStop(() => {
      // 力模拟稳定后重算小地图包围盒，修正节点最终落位
      invalidateMinimapBBox()
      drawMinimap()
      const pendStr = pendingFocus.map(n => `${n.id}@(${num(n.x)},${num(n.y)})`).join(' ')
      const cNow = graph.centerAt()
      console.log(`[KG:engineStop] pending=[${pendStr}] center=(${num(cNow.x)},${num(cNow.y)}) zoom=${num(graph.zoom(), 3)}`)
      // 节点坐标已到最终位置：精确对焦新节点（读实时坐标，避免初始位置快照偏焦）
      if (pendingFocus.length > 0) {
        focusToNodes(pendingFocus)
        pendingFocus = []
      }
    })
    .onZoom(() => {
      // 用户拖拽/缩放时钳制平移范围，防止把图整体拖出屏幕
      if (isClampingPan) return
      isClampingPan = true
      clampViewPan()
      isClampingPan = false
    })

  // 根据初始 phase 设置力布局参数
  applyPhaseForces(props.currentPhase)
  // 初始数据也按实际规模设力参数（applyPhaseForces 在无数据时按 0 规模设了偏小值）
  refreshForces(props.graphData.nodes.length)

  graph.graphData(props.graphData)

  // 锚定图质心到原点，防止力模拟漂移导致节点跑出可视区（“画布过大”根因）
  const centerForce = graph.d3Force('center')
  if (centerForce) {
    centerForce.x(0)
    centerForce.y(0)
  }

  // 初始化动画追踪集合（初始数据不需要动画）
  prevNodeIds = new Set(props.graphData.nodes.map(n => n.id))
  prevEdgeIds = new Set(props.graphData.links.map(l => l.id))
  lastNodeCount = props.graphData.nodes.length
  lastLinkCount = props.graphData.links.length

  // 等力模拟 warmup 定位后，进入「正常大小」并设缩小下限
  // 注意：不在此处调用 fitToView，因为组件可能还被 v-show 隐藏，
  // clientWidth=0 会走 fallback(300) 导致缩放系数偏大。
  // 改由 ResizeObserver 检测到容器首次获得真实尺寸时触发 fitToView。

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

  // 容器尺寸变化时同步画布尺寸（宽度 100% 响应式，窗口缩放会改变容器宽度）
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => { syncGraphSize() })
    resizeObserver.observe(containerRef.value)
  }

  // ── Bug fix: 阻止 wheel 冒泡，防止缩放到极限时页面滚动 ──
  if (containerRef.value) {
    containerRef.value.addEventListener('wheel', onContainerWheel, { passive: false })
  }
})

// refreshKey 变化时用防抖 + 冻结策略更新
watch(
  () => props.refreshKey,
  () => {
    if (graph) scheduleUpdate(props.graphData)
  },
)

// 首节点出现 = 图谱从 v-show 隐藏变可见：ResizeObserver 在 display:none→可见 的首帧时序不稳，
// 主动在 nextTick + 双 rAF（浏览器布局稳定后）补一次尺寸同步，避免画布停在挂载时的 300px。
watch(
  () => props.graphData.nodes.length,
  (n, old) => {
    if (n > 0 && old === 0) {
      nextTick(() => {
        requestAnimationFrame(() => requestAnimationFrame(() => syncGraphSize()))
      })
    }
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
    chargeStrength: -750,    // 强排斥 → 节点散开（图越大动态再放大）
    linkDistance: 700,        // 远距离 → 大范围扫描感（图越大动态再加长）
    alphaDecay: 0.015,       // 慢冷却 → 持续运动
    particleCount: 2,        // 更多粒子 → 活跃流动感
    particleSpeed: 0.006,    // 更快粒子
  },
  deep_dive: {
    chargeStrength: -300,    // 弱排斥 → 节点紧凑聚焦（图越大动态再放大）
    linkDistance: 380,        // 短距离 → 聚焦深入感（图越大动态再加长）
    alphaDecay: 0.03,        // 快冷却 → 快速稳定
    particleCount: 1,        // 更少粒子 → 沉稳
    particleSpeed: 0.003,    // 更慢粒子
  },
}

/** 图规模越大，边长与斥力越强，防止边多时节点挤成一团（以 8 节点为基准，缓增并封顶防参数爆炸） */
function forceSpreadScale(nodeCount: number): number {
  return Math.min(Math.pow(Math.max(1, nodeCount) / 8, 0.35), 2.2)
}

/** 依据当前图规模更新力参数（不 reheat，由 graphData/阶段切换触发重布局） */
function refreshForces(nodeCount: number) {
  if (!graph) return
  const cfg = PHASE_FORCE_CONFIG[props.currentPhase] ?? PHASE_FORCE_CONFIG.broad_search
  const s = forceSpreadScale(nodeCount)
  graph.d3Force('link')?.distance(cfg.linkDistance * s)
  graph.d3Force('charge')?.strength(cfg.chargeStrength * s)
}

function applyPhaseForces(phase: KGPhase) {
  if (!graph) return
  const cfg = PHASE_FORCE_CONFIG[phase] ?? PHASE_FORCE_CONFIG.broad_search
  // 通过 d3AlphaDecay 调整冷却速度；重新加热力模拟让节点响应新参数
  graph.d3AlphaDecay(cfg.alphaDecay)
  refreshForces(graph.graphData()?.nodes?.length ?? 0)
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
  if (resizeObserver) resizeObserver.disconnect()
  // ── Bug fix: 移除 wheel 事件监听 ──
  if (containerRef.value) {
    containerRef.value.removeEventListener('wheel', onContainerWheel)
  }
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
  border: 1px solid #2a3a5c;
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
  border-radius: 12px;
  background: rgba(26, 34, 54, 0.88);
  border: 1px solid rgba(42, 58, 92, 0.6);
  cursor: pointer;
  z-index: 10;
}

.kg-follow-btn {
  position: absolute;
  bottom: 116px;
  right: 8px;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: rgba(26, 34, 54, 0.88);
  border: 1px solid rgba(42, 58, 92, 0.6);
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}
.kg-follow-btn:hover {
  background: rgba(26, 34, 54, 0.94);
  color: rgba(255, 255, 255, 0.7);
}
.kg-follow-btn.active {
  color: #60a5fa;
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(26, 34, 54, 0.94);
}
</style>
