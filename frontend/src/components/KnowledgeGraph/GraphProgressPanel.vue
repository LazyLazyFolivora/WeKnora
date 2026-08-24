<template>
  <div class="kg-progress-panel" :class="{ 'is-collapsed': panelCollapsed }">
    <div class="kg-panel-header" @click="panelCollapsed = !panelCollapsed">
      <span class="kg-phase" :class="phaseClass">
        {{ phaseText }}
      </span>
      <span class="kg-timer">{{ elapsedTime }}</span>
      <span class="kg-toggle-btn">{{ panelCollapsed ? '◂' : '▾' }}</span>
    </div>
    <div v-show="!panelCollapsed" class="kg-panel-body">
      <div class="kg-counts">
        <div v-for="(count, type) in entityCounts" :key="type" class="kg-count-item">
          <span class="kg-dot" :style="{ background: entityColors[type as EntityType] || '#888' }"></span>
          <span>{{ ENTITY_TYPE_NAMES[type as EntityType] || type }}</span>
          <span class="kg-count-num">{{ count }}</span>
        </div>
      </div>
      <div v-if="recentDiscoveries.length" class="kg-timeline">
        <div v-for="(item, idx) in recentDiscoveries" :key="idx" class="kg-timeline-item">
          [{{ item.time }}] {{ item.text }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { KGPhase, EntityType } from '@/types/knowledge-graph'
import { ENTITY_COLORS, ENTITY_TYPE_NAMES } from '@/types/knowledge-graph'

interface Props {
  currentPhase: KGPhase
  startTime: number | null
  entityCounts: Record<string, number>
  recentDiscoveries: Array<{ time: string; text: string }>
  isComplete?: boolean
  frozenElapsed?: number | null
}

const props = defineProps<Props>()

const now = ref(Date.now())
const panelCollapsed = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const phaseText = computed(() => {
  if (props.isComplete) return 'Done'
  return props.currentPhase === 'broad_search' ? 'Searching...' : 'Deep diving...'
})

const phaseClass = computed(() => {
  if (props.isComplete) return 'phase-done'
  return props.currentPhase === 'broad_search' ? 'phase-breadth' : 'phase-depth'
})

const elapsedTime = computed(() => {
  // 依赖 now.value 以确保每秒触发重算
  void now.value
  // 已完成时隐藏计时
  if (props.isComplete) return ''
  // startTime 尚未就绪（null），不显示计时
  if (props.startTime == null) return ''
  // 优先使用预计算的冻结时长（历史消息）
  if (props.frozenElapsed != null) {
    const min = String(Math.floor(props.frozenElapsed / 60)).padStart(2, '0')
    const sec = String(props.frozenElapsed % 60).padStart(2, '0')
    return `Elapsed ${min}:${sec}`
  }
  // 实时计时
  const elapsed = Math.max(0, Math.floor((Date.now() - props.startTime) / 1000))
  const min = String(Math.floor(elapsed / 60)).padStart(2, '0')
  const sec = String(elapsed % 60).padStart(2, '0')
  return `${min}:${sec}`
})

const entityColors = ENTITY_COLORS

function startTimer() {
  if (timer) return
  timer = setInterval(() => { now.value = Date.now() }, 1000)
}

function stopTimer() {
  if (timer) { clearInterval(timer); timer = null }
}

// 完成时停止计时器
watch(() => props.isComplete, (done) => {
  if (done) stopTimer()
}, { immediate: true })

// startTime 从 null → 有值时，启动计时器
watch(() => props.startTime, (t) => {
  if (t != null && !props.isComplete) startTimer()
}, { immediate: true })

onMounted(() => {
  if (props.startTime != null && !props.isComplete) {
    startTimer()
  }
})

onUnmounted(() => {
  stopTimer()
})
</script>

<style scoped>
.kg-progress-panel {
  background: rgba(26, 34, 54, 0.88);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(42, 58, 92, 0.6);
  border-radius: 12px;
  font-size: 12px;
  min-width: 180px;
  transition: all 0.2s ease;
}

.kg-panel-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
}

.kg-panel-header:hover {
  background: rgba(99, 102, 241, 0.08);
  border-radius: 12px;
}

.kg-panel-body {
  padding: 0 12px 8px 12px;
}

.is-collapsed .kg-panel-header {
  padding: 6px 10px;
}

.kg-phase {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.phase-breadth {
  background: #3b82f6;
  color: white;
}

.phase-depth {
  background: #8b5cf6;
  color: white;
}

.phase-done {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
}

.kg-timer {
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: #e8edf5;
  margin-left: auto;
}

.kg-toggle-btn {
  font-size: 10px;
  color: #5a6a8c;
  flex-shrink: 0;
}

.kg-counts {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 6px;
}

.kg-count-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: #8899bb;
}

.kg-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.kg-count-num {
  font-weight: 700;
  color: #e8edf5;
  margin-left: auto;
}

.kg-timeline {
  max-height: 80px;
  overflow-y: auto;
  border-top: 1px solid #2a3a5c;
  padding-top: 6px;
}

.kg-timeline-item {
  font-size: 10px;
  color: #5a6a8c;
  padding: 2px 0;
  line-height: 1.4;
}

.kg-timeline-item:first-child {
  color: #8899bb;
}
</style>
