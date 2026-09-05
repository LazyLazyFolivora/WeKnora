/**
 * 功能开关配置
 * 管理租户级别的功能可见性控制
 * tenant_id=10000 为管理员租户，保留全部功能
 */

import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

/**
 * 当前是否为管理员租户
 */
export const isAdminTenant = () => {
  const authStore = useAuthStore()
  return authStore.isAdminTenant
}

/**
 * 非管理员租户隐藏的功能列表
 */
export const HIDDEN_FEATURES_FOR_NORMAL_TENANT = [
  'knowledgeBaseEditor:advancedSettings',  // 知识库编辑器高级设置
  'knowledgeBaseEditor:allSettings',       // 知识库编辑器全部设置项（模型、解析、存储、分块等）
  'sidebar:workspaceTab',                  // 侧边栏"本空间"标签
] as const

export type HiddenFeature = typeof HIDDEN_FEATURES_FOR_NORMAL_TENANT[number]

/**
 * 检查某功能是否对当前租户隐藏
 * @param feature 功能标识
 * @returns true 表示隐藏，false 表示显示
 */
export const isFeatureHidden = (feature: HiddenFeature): boolean => {
  if (isAdminTenant()) {
    return false // 管理员租户显示所有功能
  }
  return HIDDEN_FEATURES_FOR_NORMAL_TENANT.includes(feature)
}

/**
 * 组合式函数：获取当前租户的功能可见性状态
 */
export const useFeatureFlags = () => {
  const authStore = useAuthStore()

  const isAdmin = computed(() => authStore.isAdminTenant)

  /**
   * 是否显示侧边栏"本空间"标签
   */
  const showWorkspaceTab = computed(() => isAdmin.value)

  /**
   * 是否显示知识库编辑器高级设置
   */
  const showKnowledgeBaseAdvancedSettings = computed(() => isAdmin.value)

  /**
   * 是否显示知识库编辑器全部设置项（模型配置、向量存储、解析引擎、存储引擎、分块、知识图谱、高级设置等）
   * 管理员租户（tenant_id=10000）显示全部；普通用户仅显示基本信息
   */
  const showKnowledgeBaseAllSettings = computed(() => isAdmin.value)

  return {
    isAdmin,
    showWorkspaceTab,
    showKnowledgeBaseAdvancedSettings,
    showKnowledgeBaseAllSettings,
  }
}