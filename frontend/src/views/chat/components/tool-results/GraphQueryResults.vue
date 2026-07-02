<template>
  <div class="graph-query-results">
    <!-- Graph Configuration Card -->
    <div v-if="data.graph_config" class="stats-card">
      <div class="stats-title">{{ $t('chat.graphConfigTitle') }}</div>
      <div class="info-field">
        <span class="field-label">{{ $t('chat.entityTypesLabel') }}</span>
        <span class="field-value">{{ data.graph_config.nodes.join(', ') }}</span>
      </div>
      <div class="info-field">
        <span class="field-label">{{ $t('chat.relationTypesLabel') }}</span>
        <span class="field-value">{{ data.graph_config.relations.join(', ') }}</span>
      </div>
    </div>

    <!-- Chunk Results (from query_knowledge_graph) -->
    <div v-if="data.results && data.results.length > 0" class="results-list">
      <div class="results-header">
        {{ $t('chat.graphResultsHeader', { count: data.count }) }}
      </div>

      <div
        v-for="result in data.results"
        :key="result.chunk_id"
        class="result-card"
      >
        <div class="result-header" @click="toggleResult(result.chunk_id)">
          <div class="result-title">
            <span class="result-index">#{{ result.result_index }}</span>
            <span class="relevance-badge" :class="getRelevanceClass(result.relevance_level)">
              {{ getRelevanceLabel(result.relevance_level) }}
            </span>
            <span class="knowledge-title">{{ result.knowledge_title }}</span>
          </div>
          <div class="result-meta">
            <span class="score">{{ (result.score * 100).toFixed(0) }}%</span>
            <span class="expand-icon" :class="{ expanded: expandedResults.includes(result.chunk_id) }">
              ▶
            </span>
          </div>
        </div>

        <div class="result-content" :class="{ expanded: expandedResults.includes(result.chunk_id) }">
          <div class="full-content">{{ result.content }}</div>
        </div>
      </div>
    </div>

    <!-- Graph Data (from graph_query tool: nodes + relations) -->
    <div v-else-if="data.graph_data && data.graph_data.nodes && data.graph_data.nodes.length > 0" class="graph-data">
      <div class="results-header">
        {{ $t('chat.graphDataHeader', { nodes: data.graph_data.total_nodes ?? data.graph_data.nodes.length, edges: data.graph_data.total_edges ?? data.graph_data.relations.length }) }}
      </div>

      <div v-if="data.graph_data.nodes.length > 0" class="nodes-section">
        <div class="section-label">{{ $t('chat.graphNodesLabel') }} ({{ data.graph_data.nodes.length }})</div>
        <div
          v-for="(node, idx) in data.graph_data.nodes"
          :key="idx"
          class="node-card"
        >
          <div class="result-header" @click="toggleResult('node-' + idx)">
            <div class="result-title">
              <span class="result-index">#{{ idx + 1 }}</span>
              <span class="knowledge-title">{{ node.name }}</span>
            </div>
            <div class="result-meta">
              <span v-if="node.attributes && node.attributes.length > 0" class="attr-count">{{ node.attributes.length }} attrs</span>
              <span class="expand-icon" :class="{ expanded: expandedResults.includes('node-' + idx) }">
                ▶
              </span>
            </div>
          </div>
          <div class="result-content" :class="{ expanded: expandedResults.includes('node-' + idx) }">
            <div v-if="node.attributes && node.attributes.length > 0" class="node-attrs">
              <span
                v-for="(attr, i) in node.attributes"
                :key="i"
                class="attr-tag"
              >{{ attr }}</span>
            </div>
            <div v-if="node.chunks && node.chunks.length > 0" class="node-chunks">
              <div class="attr-label">Chunks:</div>
              <div v-for="(chunk, i) in node.chunks" :key="i" class="chunk-text">{{ chunk }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="data.graph_data.relations && data.graph_data.relations.length > 0" class="relations-section">
        <div class="section-label">{{ $t('chat.graphRelationsLabel') }} ({{ data.graph_data.relations.length }})</div>
        <div
          v-for="(rel, idx) in data.graph_data.relations"
          :key="idx"
          class="relation-card"
        >
          <span class="rel-node">{{ rel.node1 }}</span>
          <span class="rel-arrow">—[{{ rel.type }}]→</span>
          <span class="rel-node">{{ rel.node2 }}</span>
        </div>
      </div>
    </div>

    <div v-else class="empty-state">
      {{ $t('chat.graphNoResults') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { GraphQueryResultsData, GraphQueryGraphData, RelevanceLevel } from '@/types/tool-results';
import { useI18n } from 'vue-i18n';

const props = defineProps<{
  data: GraphQueryResultsData;
}>();

const { t } = useI18n();

const expandedResults = ref<string[]>([]);

const toggleResult = (chunkId: string) => {
  const index = expandedResults.value.indexOf(chunkId);
  if (index > -1) {
    expandedResults.value.splice(index, 1);
  } else {
    expandedResults.value.push(chunkId);
  }
};

const getRelevanceClass = (level: RelevanceLevel): string => {
  const classMap: Record<RelevanceLevel, string> = {
    'High Relevance': 'high',
    'Medium Relevance': 'medium',
    'Low Relevance': 'low',
    'Weak Relevance': 'weak',
  };
  return classMap[level] || 'weak';
};

const getRelevanceLabel = (level: RelevanceLevel): string => {
  const labelMap: Record<RelevanceLevel, string> = {
    'High Relevance': t('chat.relevanceHigh'),
    'Medium Relevance': t('chat.relevanceMedium'),
    'Low Relevance': t('chat.relevanceLow'),
    'Weak Relevance': t('chat.relevanceWeak'),
  };
  return labelMap[level] || level;
};
</script>

<style lang="less" scoped>
@import './tool-results.less';

.graph-query-results {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.results-header {
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  padding: 4px 0;
}

.result-index {
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  font-weight: 600;
}

.knowledge-title {
  font-size: 13px;
  color: var(--td-text-color-primary);
  flex: 1;
}

.score {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  font-weight: 500;
}

.graph-data {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.nodes-section,
.relations-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 4px 0;
  border-bottom: 1px solid var(--td-border-level-1-color);
}

.node-card {
  border: 1px solid var(--td-border-level-1-color);
  border-radius: 6px;
  overflow: hidden;
}

.node-attrs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 8px 12px;
}

.attr-tag {
  display: inline-block;
  padding: 2px 8px;
  background: var(--td-bg-color-component);
  border-radius: 4px;
  font-size: 11px;
  color: var(--td-text-color-secondary);
  font-family: monospace;
}

.attr-count {
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.attr-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
  margin-bottom: 4px;
}

.node-chunks {
  padding: 0 12px 8px;
}

.chunk-text {
  font-size: 12px;
  color: var(--td-text-color-primary);
  padding: 4px 0;
  border-bottom: 1px solid var(--td-border-level-1-color);
}

.relation-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid var(--td-border-level-1-color);
  border-radius: 6px;
  font-size: 13px;
}

.rel-node {
  font-weight: 500;
  color: var(--td-brand-color);
}

.rel-arrow {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  font-family: monospace;
}
</style>

