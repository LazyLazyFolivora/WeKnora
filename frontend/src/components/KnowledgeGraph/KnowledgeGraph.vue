<template>
  <div class="knowledge-graph" v-show="hasEverHadData">
    <div class="kg-header">
      <span class="kg-title">KnowledgeGraph</span>
      <span class="kg-stats">{{ totalNodes }} entities · {{ totalLinks }} relations</span>
      <button class="kg-action-btn" @click="isFullscreen = true" title="全屏查看">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M8 3H5a2 2 0 00-2 2v3m18 0V5a2 2 0 00-2-2h-3m0 18h3a2 2 0 002-2v-3M3 16v3a2 2 0 002 2h3"/>
        </svg>
      </button>
      <button class="kg-action-btn" @click="collapsed = !collapsed">
        {{ collapsed ? '▸' : '▾' }}
      </button>
    </div>
    <div v-show="!collapsed" class="kg-body">
      <div class="kg-canvas-outer" :class="isComplete ? '' : `kg-phase-${currentPhase}`">
        <div class="kg-canvas-wrapper">
        <GraphCanvas
          ref="graphCanvasRef"
          :graph-data="graphData"
          :refresh-key="refreshKey"
          :start-entity-names="startEntityNames"
          :current-phase="currentPhase"
          @node-click="handleNodeClick"
          @node-hover="handleNodeHover"
        />
        <GraphProgressPanel
          :current-phase="currentPhase"
          :start-time="startTime"
          :entity-counts="entityCounts"
          :recent-discoveries="recentDiscoveries"
          :is-complete="isComplete"
          :frozen-elapsed="frozenElapsed"
          class="kg-progress-overlay"
        />
        <!-- 阶段切换标签动画 -->
        <Transition name="kg-phase-flash">
          <div v-if="phaseFlashVisible" class="kg-phase-flash" :key="currentPhase">
            {{ currentPhase === 'broad_search' ? '广域检索' : '深入挖掘' }}
          </div>
        </Transition>
        </div>
      </div>
      <!-- 悬停提示 -->
      <div v-if="hoveredNode && !detailNode" class="kg-tooltip">
        <div class="kg-tooltip-name">{{ hoveredNode.name }}</div>
        <div class="kg-tooltip-type" :style="{ background: entityColors[hoveredNode.entityType], color: tooltipTypeColor(hoveredNode.entityType) }">
          {{ ENTITY_TYPE_NAMES[hoveredNode.entityType] }}
        </div>
        <div v-if="hoveredNode.observations?.length" class="kg-tooltip-desc">
          {{ hoveredNode.observations[0] }}
        </div>
      </div>
      <!-- 节点详情弹窗 -->
      <Transition name="kg-detail-fade">
        <div v-if="detailNode" class="kg-detail-card" @click.stop>
          <div class="kg-detail-header">
            <div class="kg-detail-title">{{ detailNode.name }}</div>
            <button class="kg-detail-close" @click="detailNode = null">✕</button>
          </div>
          <div class="kg-detail-type" :style="{ background: entityColors[detailNode.entityType], color: tooltipTypeColor(detailNode.entityType) }">
            {{ ENTITY_TYPE_NAMES[detailNode.entityType] || detailNode.entityType }}
          </div>
          <div v-if="detailNode.sourceKb" class="kg-detail-meta">
            <span class="kg-detail-label">来源</span>
            <span>{{ detailNode.sourceKb }}</span>
          </div>
          <div class="kg-detail-meta">
            <span class="kg-detail-label">状态</span>
            <span v-if="detailNode.status === 'planned'" class="kg-detail-status planned">
              <svg class="kg-detail-status-icon" viewBox="0 0 16 16" width="12" height="12">
                <polygon points="8,2 2,13 14,13" fill="none" stroke="#94A3B8" stroke-width="1.5" stroke-linejoin="round"/>
              </svg>
              预生成 — AI 从问题中预判的关键实体，待检索确认
            </span>
            <span v-else-if="detailNode.status === 'confirmed'">已确认</span>
            <span v-else>搜索中</span>
          </div>
          <div v-if="detailNode.observations?.length" class="kg-detail-section">
            <div class="kg-detail-section-title">观测描述</div>
            <div v-for="(obs, idx) in detailNode.observations" :key="idx" class="kg-detail-obs">
              {{ obs }}
            </div>
          </div>
          <div v-else class="kg-detail-empty">暂无详细描述</div>
        </div>
      </Transition>
    </div>
  </div>
  <!-- 全屏覆盖层 -->
  <Teleport to="body">
    <Transition name="kg-fullscreen-fade">
      <div v-if="isFullscreen" class="kg-fullscreen-overlay" @click.self="isFullscreen = false">
        <button class="kg-fullscreen-exit" @click="isFullscreen = false" title="退出全屏 (Esc)">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 14h3a2 2 0 012 2v3m4-5h3a2 2 0 012 2v3M14 4v3a2 2 0 01-2 2H9M4 10V7a2 2 0 012-2h3"/>
          </svg>
        </button>
        <div class="kg-fullscreen-container" :class="isComplete ? '' : `kg-phase-${currentPhase}`">
          <div class="kg-canvas-wrapper">
            <GraphCanvas
              ref="fullscreenCanvasRef"
              :graph-data="graphData"
              :refresh-key="refreshKey"
              :start-entity-names="startEntityNames"
              :current-phase="currentPhase"
              @node-click="handleNodeClick"
              @node-hover="handleNodeHover"
            />
            <GraphProgressPanel
              :current-phase="currentPhase"
              :start-time="startTime"
              :entity-counts="entityCounts"
              :recent-discoveries="recentDiscoveries"
              :is-complete="isComplete"
              :frozen-elapsed="frozenElapsed"
              class="kg-progress-overlay"
            />
            <!-- 全屏时的 tooltip -->
            <div v-if="hoveredNode && !detailNode" class="kg-tooltip">
              <div class="kg-tooltip-name">{{ hoveredNode.name }}</div>
              <div class="kg-tooltip-type" :style="{ background: entityColors[hoveredNode.entityType], color: tooltipTypeColor(hoveredNode.entityType) }">
                {{ ENTITY_TYPE_NAMES[hoveredNode.entityType] }}
              </div>
              <div v-if="hoveredNode.observations?.length" class="kg-tooltip-desc">
                {{ hoveredNode.observations[0] }}
              </div>
            </div>
            <!-- 全屏时的详情弹窗 -->
            <Transition name="kg-detail-fade">
              <div v-if="detailNode" class="kg-detail-card" @click.stop>
              <div class="kg-detail-header">
                <div class="kg-detail-title">{{ detailNode.name }}</div>
                <button class="kg-detail-close" @click="detailNode = null">✕</button>
              </div>
              <div class="kg-detail-type" :style="{ background: entityColors[detailNode.entityType], color: tooltipTypeColor(detailNode.entityType) }">
                {{ ENTITY_TYPE_NAMES[detailNode.entityType] || detailNode.entityType }}
              </div>
              <div v-if="detailNode.sourceKb" class="kg-detail-meta">
                <span class="kg-detail-label">来源</span>
                <span>{{ detailNode.sourceKb }}</span>
              </div>
              <div class="kg-detail-meta">
                <span class="kg-detail-label">状态</span>
                <span v-if="detailNode.status === 'planned'" class="kg-detail-status planned">
                  <svg class="kg-detail-status-icon" viewBox="0 0 16 16" width="12" height="12">
                    <polygon points="8,2 2,13 14,13" fill="none" stroke="#94A3B8" stroke-width="1.5" stroke-linejoin="round"/>
                  </svg>
                  预生成 — AI 从问题中预判的关键实体，待检索确认
                </span>
                <span v-else-if="detailNode.status === 'confirmed'">已确认</span>
                <span v-else>搜索中</span>
              </div>
              <div v-if="detailNode.observations?.length" class="kg-detail-section">
                <div class="kg-detail-section-title">观测描述</div>
                <div v-for="(obs, idx) in detailNode.observations" :key="idx" class="kg-detail-obs">
                  {{ obs }}
                </div>
              </div>
              <div v-else class="kg-detail-empty">暂无详细描述</div>
            </div>
          </Transition>
          </div> <!-- /kg-canvas-wrapper -->
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import GraphCanvas from './GraphCanvas.vue'
import GraphProgressPanel from './GraphProgressPanel.vue'
import type { GraphNodeView } from '@/types/knowledge-graph'
import { ENTITY_COLORS, ENTITY_TYPE_NAMES } from '@/types/knowledge-graph'
import { useKnowledgeGraph } from '@/composables/useKnowledgeGraph'

interface Props {
  sessionId?: string
  messageId?: string
  agentEventStream?: any[]
  isCompleted?: boolean
}

const props = defineProps<Props>()

const {
  graphData, currentPhase, startTime, isComplete, frozenElapsed,
  recentDiscoveries, entityCounts,
  totalNodes, totalLinks, refreshKey, startEntityNames,
  applyAgentGraphEvent, fetchFullGraph, reset, destroy,
} = useKnowledgeGraph()

const graphCanvasRef = ref<InstanceType<typeof GraphCanvas> | null>(null)
const hoveredNode = ref<GraphNodeView | null>(null)
const detailNode = ref<GraphNodeView | null>(null)
const collapsed = ref(true)
const isFullscreen = ref(false)
const entityColors = ENTITY_COLORS
const hasEverHadData = ref(false)
const fullscreenCanvasRef = ref<InstanceType<typeof GraphCanvas> | null>(null)

/** 根据实体背景色亮度返回对比文字颜色（浅底深字、深底白字） */
function tooltipTypeColor(entityType: string): string {
  const hex = entityColors[entityType] || '#888'
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  // 相对亮度（ITU-R BT.601）
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  // 亮度 > 0.65 视为浅色背景 → 用知识图谱深蓝背景色作文字色
  return luminance > 0.65 ? '#111827' : '#fff'
}

// ── 全屏 Escape 退出 ──
function onEscapeKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && isFullscreen.value) isFullscreen.value = false
}
onMounted(() => { document.addEventListener('keydown', onEscapeKey) })
onUnmounted(() => { document.removeEventListener('keydown', onEscapeKey) })

// ── 阶段切换过渡动画 ──
const phaseFlashVisible = ref(false)

watch(
  () => currentPhase.value,
  (newPhase, oldPhase) => {
    if (newPhase !== oldPhase && oldPhase != null) {
      phaseFlashVisible.value = true
      setTimeout(() => { phaseFlashVisible.value = false }, 1200)
      graphCanvasRef.value?.zoomPulse()
    }
  },
)

// 首次出现图谱数据时自动展开，之后保持用户手动折叠状态
watch(
  () => totalNodes.value,
  (n) => {
    if (n > 0 && !hasEverHadData.value) {
      hasEverHadData.value = true
      collapsed.value = false
    }
  },
  { immediate: true },
)

function handleNodeClick(node: GraphNodeView) {
  if (detailNode.value?.id === node.id) {
    detailNode.value = null
  } else {
    detailNode.value = node
  }
}

function handleNodeHover(node: GraphNodeView | null) {
  hoveredNode.value = node
}

// 通过 CustomEvent 全局事件接收 SSE 事件（绕过 Vue prop 响应式限制）
function handleKGEvent(evt: Event) {
  const e = (evt as CustomEvent).detail
  if (e?.type === 'agent_graph' && e.payload) {
    applyAgentGraphEvent(e.payload)
  }
}

onMounted(() => {
  window.addEventListener('kg-sse-event', handleKGEvent)

  // 1. 先从 agentEventStream 回放已有事件
  replayGraphEvents()

  // 2. 始终拉取图数据恢复计时器（started_at）
  //    实时会话：仅恢复计时器，不加载节点（走 SSE 缓冲）
  //    历史消息：全量加载图数据
  if (props.sessionId && props.messageId) {
    fetchFullGraph(props.sessionId, props.messageId, props.isCompleted)
  }
})

onUnmounted(() => {
  window.removeEventListener('kg-sse-event', handleKGEvent)
  destroy()
})

// 从 agentEventStream 回放图谱事件重建图谱（绕过缓冲，立即渲染）
function replayGraphEvents() {
  const stream = props.agentEventStream as any[] | undefined
  if (!stream?.length) return
  for (const evt of stream) {
    if (evt.type === 'agent_graph' && evt.payload) {
      applyAgentGraphEvent(evt.payload, { immediate: true })
    }
  }
}

// 当消息完成时：停止计时
watch(
  () => props.isCompleted,
  (completed) => {
    if (completed) {
      isComplete.value = true
    }
  },
)
// 监听 agentEventStream 变化：检测 stop 事件（图谱事件通过 kg-sse-event 走缓冲）
watch(
  () => props.agentEventStream,
  (stream) => {
    if (!stream?.length) return
    if (stream.some((e: any) => e.type === 'stop')) {
      isComplete.value = true
    }
  },
  { deep: true },
)
// 图谱生长完成后：自动缩放展示全貌
watch(
  () => isComplete.value,
  (done) => { if (done) graphCanvasRef.value?.fitGraphToView() },
)
// 三重保险：轮询检测 is_completed 属性（应对 Vue 深层响应式丢失）
let pollTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  pollTimer = setInterval(() => {
    if (props.isCompleted && !isComplete.value) {
      isComplete.value = true
    }
  }, 500)
})
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.knowledge-graph {
  position: relative;
  width: 100%;
}

.kg-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}

.kg-title {
  font-size: 11px;
  font-weight: 600;
  color: #60a5fa;
  letter-spacing: 0.5px;
}

.kg-stats {
  font-size: 10px;
  color: #8899bb;
  margin-left: auto;
}

.kg-action-btn {
  padding: 2px 6px;
  border: none;
  background: transparent;
  color: #5a6a8c;
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.kg-action-btn:hover {
  color: #e8edf5;
}

.kg-body {
  position: relative;
}

.kg-canvas-outer {
  position: relative;
  border-radius: 10px;
  padding: 3px; /* 发光带宽度 */
}
.kg-canvas-wrapper {
  position: relative;
  z-index: 1; /* 盖住 ::before / ::after 伪元素，只留 3px 间隙透出光带 */
  width: 100%;
  height: 550px;
  border-radius: 8px;
  overflow: hidden;
  background: linear-gradient(135deg, #0a0f1a 0%, #0f1729 50%, #0a1020 100%);
}
/* ── 阶段流转发光边框（外层 ::before 柔光 + ::after 清晰光线） ── */
.kg-phase-broad_search::before,
.kg-phase-deep_dive::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: conic-gradient(
    from var(--border-angle, 0deg),
    transparent 0%,
    color-mix(in srgb, var(--glow-color) 25%, transparent) 8%,
    var(--glow-color) 16%,
    color-mix(in srgb, var(--glow-color) 25%, transparent) 24%,
    transparent 32%
  );
  filter: blur(6px);
  animation: kg-border-flow 3s linear infinite;
  pointer-events: none;
}
.kg-phase-broad_search::after,
.kg-phase-deep_dive::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: conic-gradient(
    from var(--border-angle, 0deg),
    transparent 0%,
    color-mix(in srgb, var(--glow-color) 50%, transparent) 10%,
    var(--glow-color) 15%,
    color-mix(in srgb, var(--glow-color) 50%, transparent) 20%,
    transparent 28%
  );
  animation: kg-border-flow 3s linear infinite;
  pointer-events: none;
}
.kg-phase-broad_search { --glow-color: #3b82f6; }
.kg-phase-deep_dive    { --glow-color: #8b5cf6; }

@keyframes kg-border-flow {
  to { --border-angle: 360deg; }
}

@property --border-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

/* ── 全屏容器同样保留阶段流转发光边框 ── */
.kg-fullscreen-container.kg-phase-broad_search,
.kg-fullscreen-container.kg-phase-deep_dive {
  padding: 3px;
  border-radius: 15px;
}
.kg-fullscreen-container.kg-phase-broad_search { --glow-color: #3b82f6; }
.kg-fullscreen-container.kg-phase-deep_dive    { --glow-color: #8b5cf6; }
.kg-fullscreen-container.kg-phase-broad_search::before,
.kg-fullscreen-container.kg-phase-deep_dive::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 15px;
  background: conic-gradient(
    from var(--border-angle, 0deg),
    transparent 0%,
    color-mix(in srgb, var(--glow-color) 25%, transparent) 8%,
    var(--glow-color) 16%,
    color-mix(in srgb, var(--glow-color) 25%, transparent) 24%,
    transparent 32%
  );
  filter: blur(8px);
  animation: kg-border-flow 3s linear infinite;
  pointer-events: none;
}
.kg-fullscreen-container.kg-phase-broad_search::after,
.kg-fullscreen-container.kg-phase-deep_dive::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 15px;
  background: conic-gradient(
    from var(--border-angle, 0deg),
    transparent 0%,
    color-mix(in srgb, var(--glow-color) 60%, transparent) 10%,
    var(--glow-color) 15%,
    color-mix(in srgb, var(--glow-color) 60%, transparent) 20%,
    transparent 28%
  );
  animation: kg-border-flow 3s linear infinite;
  pointer-events: none;
}
/* 全屏容器内的 canvas-wrapper 填充剩余空间 */
.kg-fullscreen-container .kg-canvas-wrapper {
  width: 100%;
  height: 100%;
  border-radius: 12px;
  z-index: 1;
}

.kg-progress-overlay {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 10;
}

.kg-tooltip {
  position: absolute;
  bottom: 12px;
  left: 12px;
  background: rgba(17, 24, 39, 0.94);
  backdrop-filter: blur(12px);
  border: 1px solid #2a3a5c;
  border-radius: 8px;
  padding: 8px 12px;
  z-index: 10;
  max-width: 250px;
}

.kg-tooltip-name {
  font-size: 14px;
  font-weight: 700;
  color: #e8edf5;
  margin-bottom: 4px;
}

.kg-tooltip-type {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  margin-bottom: 4px;
}

.kg-tooltip-desc {
  font-size: 11px;
  color: #8899bb;
  line-height: 1.5;
}

/* ── 阶段切换标签动画 ── */
.kg-phase-flash {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: rgba(17, 24, 39, 0.94);
  backdrop-filter: blur(12px);
  border: 1px solid #2a3a5c;
  border-radius: 12px;
  padding: 12px 24px;
  font-size: 18px;
  font-weight: 700;
  color: #e8edf5;
  z-index: 20;
  pointer-events: none;
  white-space: nowrap;
}

.kg-phase-flash-enter-active {
  transition: all 0.35s cubic-bezier(0.22, 1, 0.36, 1);
}
.kg-phase-flash-leave-active {
  transition: all 0.4s cubic-bezier(0.55, 0, 1, 0.45);
}
.kg-phase-flash-enter-from {
  opacity: 0;
  transform: translate(-50%, -50%) scale(0.7);
}
.kg-phase-flash-enter-to {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
}
.kg-phase-flash-leave-from {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
}
.kg-phase-flash-leave-to {
  opacity: 0;
  transform: translate(-50%, -50%) scale(1.15);
}

/* ── 节点详情弹窗 ── */
.kg-detail-card {
  position: absolute;
  bottom: 12px;
  left: 12px;
  background: rgba(17, 24, 39, 0.94);
  backdrop-filter: blur(16px);
  border: 1px solid #2a3a5c;
  border-radius: 10px;
  padding: 14px 16px;
  z-index: 15;
  max-width: 320px;
  min-width: 220px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}

.kg-detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.kg-detail-title {
  font-size: 15px;
  font-weight: 700;
  color: #e8edf5;
  line-height: 1.3;
  word-break: break-all;
}

.kg-detail-close {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: rgba(255, 255, 255, 0.06);
  color: #5a6a8c;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
  line-height: 1;
}
.kg-detail-close:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #e8edf5;
}

.kg-detail-type {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  margin-bottom: 10px;
}

.kg-detail-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: #8899bb;
  margin-bottom: 4px;
}

.kg-detail-label {
  color: #5a6a8c;
  font-weight: 600;
  min-width: 32px;
}

.kg-detail-status.planned {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #94A3B8;
}
.kg-detail-status-icon {
  flex-shrink: 0;
}

.kg-detail-section {
  margin-top: 8px;
  border-top: 1px solid #2a3a5c;
  padding-top: 8px;
}

.kg-detail-section-title {
  font-size: 10px;
  font-weight: 600;
  color: #5a6a8c;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 4px;
}

.kg-detail-obs {
  font-size: 11px;
  color: #8899bb;
  line-height: 1.5;
  padding: 2px 0;
}

.kg-detail-empty {
  font-size: 11px;
  color: #5a6a8c;
  font-style: italic;
}

/* ── 全屏覆盖层 ── */
.kg-fullscreen-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
}
.kg-fullscreen-container {
  width: 90vw;
  height: 90vh;
  position: relative;
  border-radius: 12px;
  overflow: visible;
  background: linear-gradient(135deg, #0a0f1a 0%, #0f1729 50%, #0a1020 100%);
}
.kg-fullscreen-container .kg-progress-overlay {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 10;
}
.kg-fullscreen-container .kg-tooltip {
  position: absolute;
  bottom: 12px;
  left: 12px;
  z-index: 10;
}
.kg-fullscreen-container .kg-detail-card {
  position: absolute;
  bottom: 12px;
  left: 12px;
  z-index: 15;
}
/* 退出按钮 */
.kg-fullscreen-exit {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 10;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(8px);
  color: rgba(255, 255, 255, 0.6);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}
.kg-fullscreen-exit:hover {
  background: rgba(0, 0, 0, 0.7);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.3);
}
/* 进出动画 */
.kg-fullscreen-fade-enter-active,
.kg-fullscreen-fade-leave-active {
  transition: opacity 0.25s ease;
}
.kg-fullscreen-fade-enter-from,
.kg-fullscreen-fade-leave-to {
  opacity: 0;
}

/* ── 详情卡进出动画 ── */
.kg-detail-fade-enter-active {
  transition: all 0.25s cubic-bezier(0.22, 1, 0.36, 1);
}
.kg-detail-fade-leave-active {
  transition: all 0.2s cubic-bezier(0.55, 0, 1, 0.45);
}
.kg-detail-fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.kg-detail-fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
