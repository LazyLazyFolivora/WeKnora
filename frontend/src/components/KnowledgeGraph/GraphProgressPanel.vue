<template>
  <div class="kg-progress-panel" :class="{ 'is-collapsed': panelCollapsed }">
    <div class="kg-panel-header" @click="panelCollapsed = !panelCollapsed">
      <span class="kg-phase" :class="currentPhase === 'broad_search' ? 'phase-breadth' : 'phase-depth'">
        {{ currentPhase === 'broad_search' ? '🔍 广域检索中...' : '🎯 深入挖掘中...' }}
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
import { computed, ref, onMounted, onUnmounted } from 'vue'
import type { KGPhase, EntityType } from '@/types/knowledge-graph'
import { ENTITY_COLORS, ENTITY_TYPE_NAMES } from '@/types/knowledge-graph'

interface Props {
  currentPhase: KGPhase
  startTime: number
  entityCounts: Record<string, number>
  recentDiscoveries: Array<{ time: string; text: string }>
}

const props = defineProps<Props>()

const now = ref(Date.now())
const panelCollapsed = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const elapsedTime = computed(() => {
  const elapsed = Math.floor((now.value - props.startTime) / 1000)
  const min = String(Math.floor(elapsed / 60)).padStart(2, '0')
  const sec = String(elapsed % 60).padStart(2, '0')
  return `${min}:${sec}`
})

const entityColors = ENTITY_COLORS

onMounted(() => {
  timer = setInterval(() => { now.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.kg-progress-panel {
  background: rgba(15, 15, 35, 0.85);
  backdrop-filter: blur(12px);
  border: none;
  border-radius: 8px;
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
  background: rgba(255, 255, 255, 0.04);
  border-radius: 8px;
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

.kg-timer {
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: #e5e7eb;
  margin-left: auto;
}

.kg-toggle-btn {
  font-size: 10px;
  color: #6b7280;
  flex-shrink: 0;
}

.kg-counts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  margin-bottom: 6px;
}

.kg-count-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: #9ca3af;
}

.kg-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.kg-count-num {
  font-weight: 700;
  color: #e5e7eb;
  margin-left: auto;
}

.kg-timeline {
  max-height: 80px;
  overflow-y: auto;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 6px;
}

.kg-timeline-item {
  font-size: 10px;
  color: #6b7280;
  padding: 2px 0;
  line-height: 1.4;
}

.kg-timeline-item:first-child {
  color: #d1d5db;
}
</style>
