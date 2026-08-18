<template>
  <div class="knowledge-graph" v-show="hasEverHadData">
    <div class="kg-header">
      <span class="kg-title">KnowledgeGraph</span>
      <span class="kg-stats">{{ totalNodes }} entities · {{ totalLinks }} relations</span>
      <button class="kg-toggle" @click="collapsed = !collapsed">
        {{ collapsed ? '▸' : '▾' }}
      </button>
    </div>
    <div v-show="!collapsed" class="kg-body">
      <div class="kg-canvas-wrapper">
        <GraphCanvas
          :graph-data="graphData"
          :refresh-key="refreshKey"
          :start-entity-names="startEntityNames"
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
      </div>
      <div v-if="hoveredNode" class="kg-tooltip">
        <div class="kg-tooltip-name">{{ hoveredNode.name }}</div>
        <div class="kg-tooltip-type" :style="{ background: entityColors[hoveredNode.entityType] }">
          {{ ENTITY_TYPE_NAMES[hoveredNode.entityType] }}
        </div>
        <div v-if="hoveredNode.observations?.length" class="kg-tooltip-desc">
          {{ hoveredNode.observations[0] }}
        </div>
      </div>
    </div>
  </div>
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
  applyAgentGraphEvent, fetchFullGraph, reset,
} = useKnowledgeGraph()

const hoveredNode = ref<GraphNodeView | null>(null)
const collapsed = ref(false)
const entityColors = ENTITY_COLORS
const hasEverHadData = ref(false)

// 一旦有过数据，就永远保持可见（用户手动折叠）
watch(
  () => totalNodes.value,
  (n) => { if (n > 0) hasEverHadData.value = true },
  { immediate: true },
)

function handleNodeClick(node: GraphNodeView) {
  // 节点点击交互（后续可扩展为弹窗）
  console.log('[KG] node click:', node.name, node)
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

  // 2. 有 sessionId/messageId 时从后端拉取完整图谱（覆盖回放结果，确保数据完整）
  if (props.sessionId && props.messageId) {
    fetchFullGraph(props.sessionId, props.messageId)
  }
})

onUnmounted(() => {
  window.removeEventListener('kg-sse-event', handleKGEvent)
})

// 从 agentEventStream 回放图谱事件重建图谱
function replayGraphEvents() {
  const stream = props.agentEventStream as any[] | undefined
  if (!stream?.length) return
  for (const evt of stream) {
    if (evt.type === 'agent_graph' && evt.payload) {
      applyAgentGraphEvent(evt.payload)
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
// 监听 agentEventStream 变化回放图谱事件 + 检测 stop
watch(
  () => props.agentEventStream,
  (stream) => {
    if (stream?.some((e: any) => e.type === 'stop')) {
      isComplete.value = true
    }
    // 回放重建图谱
    replayGraphEvents()
  },
  { deep: true, immediate: true },
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
  color: #6b7280;
  margin-left: auto;
}

.kg-toggle {
  padding: 0 4px;
  border: none;
  background: transparent;
  color: #6b7280;
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s;
  line-height: 1;
}

.kg-toggle:hover {
  color: #e5e7eb;
}

.kg-body {
  position: relative;
}

.kg-canvas-wrapper {
  position: relative;
  width: 100%;
  height: 550px;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(15, 15, 35, 0.4);
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
  background: rgba(15, 15, 35, 0.92);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 8px 12px;
  z-index: 10;
  max-width: 250px;
}

.kg-tooltip-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
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
  color: #9ca3af;
  line-height: 1.5;
}
</style>
