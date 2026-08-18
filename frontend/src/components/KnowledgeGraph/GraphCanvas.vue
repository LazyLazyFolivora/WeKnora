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

  // 矛盾边用红色虚线 + 闪烁；普通边蓝色实线
  const isContra = isContradiction(l)
  const dimColor = isContra ? 'rgba(239,68,68,0.25)' : 'rgba(255,255,255,0.08)'
  const growColor = isContra
    ? (blinkOn.value ? 'rgba(239,68,68,0.8)' : 'rgba(239,68,68,0.25)')
    : 'rgba(96,165,250,0.7)'
  const tipColor = isContra ? 'rgba(239,68,68,0.95)' : 'rgba(147,197,253,0.95)'

  // ① 底层淡线：完整长度、低透明度，提示目标方向
  ctx.beginPath()
  ctx.moveTo(s.x, s.y)
  ctx.lineTo(t.x, t.y)
  if (isContra) ctx.setLineDash([6, 4])
  ctx.strokeStyle = dimColor
  ctx.lineWidth = (isContra ? 2 : 1.5) * scale
  ctx.stroke()
  ctx.setLineDash([])

  // ② 已生长段：从源到当前尖端
  ctx.beginPath()
  ctx.moveTo(s.x, s.y)
  ctx.lineTo(tipX, tipY)
  if (isContra) ctx.setLineDash([6, 4])
  ctx.strokeStyle = growColor
  ctx.lineWidth = 2 * scale
  ctx.stroke()
  ctx.setLineDash([])

  // ③ 尖端脉冲发光点（生长过程中的"头"）
  const pulse = 1 + 0.6 * Math.sin(now / 60)
  ctx.beginPath()
  ctx.arc(tipX, tipY, 3.5 * pulse * scale, 0, 2 * Math.PI)
  ctx.fillStyle = tipColor
  ctx.fill()

  // ④ 悬停节点时，显示与其相连的边的关系标签
  if (grow >= 1 && hoveredNode && (s.id === hoveredNode.id || t.id === hoveredNode.id)) {
    const mx = (s.x + t.x) / 2, my = (s.y + t.y) / 2
    ctx.font = `${9 * scale}px system-ui, sans-serif`
    ctx.textAlign = 'center'
    ctx.textBaseline = 'bottom'
    ctx.fillStyle = isContra ? 'rgba(239,68,68,0.85)' : 'rgba(156,163,175,0.85)'
    ctx.fillText(prettifyRelation(l.relationType), mx, my - 2 * scale)
  }
}

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
  currentPhase: () => 'broad_search',
})

const emit = defineEmits<{
  nodeClick: [node: GraphNodeView]
  nodeHover: [node: GraphNodeView | null]
}>()

const containerRef = ref<HTMLDivElement>()
const minimapRef = ref<HTMLCanvasElement>()
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
    lastNodeCount = data.nodes.length
    lastLinkCount = data.links.length
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
    // 自定义节点渲染：浮现动画 + 三态 status 视觉 + deep_dive 焦点环
    .nodeCanvasObject((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const dim = isDimmed(node)
      const scale = 1 / globalScale
      const r = 8 * scale
      const baseColor = props.entityColors[node.entityType] || '#888'
      // 三态视觉：planned 灰、searching 淡（降透明度）、confirmed 实色
      let color = baseColor
      let statusAlpha = 1
      if (node.status === 'planned') color = '#6b7280'
      else if (node.status === 'searching') statusAlpha = 0.66

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
        ctx.fillStyle = baseColor + '18'
        ctx.fill()
      }

      ctx.globalAlpha = (dim ? 0.15 : statusAlpha) * appearAlpha      // ★ 主圆现在会淡入
      ctx.beginPath()
      ctx.arc(node.x, node.y, r * appearScale, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()

      ctx.globalAlpha = (dim ? 0.15 : statusAlpha) * appearAlpha      // ★ 高光同步淡入
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
    // 链接宽度：矛盾连线加粗，hover 时高亮关联连线
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
    .linkCanvasObjectMode(() => 'replace')     // 完全接管链路渲染（矛盾边在 drawGrowingLink 内处理）
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
