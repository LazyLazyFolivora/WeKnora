/**
 * 合成弱边生成器（前端纯视觉补边）
 *
 * 目的：后端产出的真实关系（RelationFound）过少时，图谱稀疏不好看。
 * 这里按确定性规则补出「弱关联」边，只用于渲染层，不落库、不污染真实数据。
 *
 * 确定性：所有「随机」（目标密度、选边、strength）都由同一颗种子 PRNG 驱动，
 * 种子来自排序后的实体名列表哈希 —— 同输入 → 同序列 + 同密度 + 同宽度。
 * 全程不使用 Math.random()。
 */
import type { GraphNodeView, GraphEdgeView } from '@/types/knowledge-graph'

// 目标密度系数范围（合成边补到约 N×factor 条）
const DENSITY_MIN = 1.2
const DENSITY_MAX = 2.2
// 合成边 strength 范围（驱动宽度变化，仍偏弱）
const STRENGTH_MIN = 0.2
const STRENGTH_MAX = 0.8

/** cyrb53 字符串哈希（bryc），确定性，输出 [0, 2^53) 的数值 */
function cyrb53(str: string, seed = 0): number {
  let h1 = 0xdeadbeef ^ seed
  let h2 = 0x41c6ce57 ^ seed
  for (let i = 0; i < str.length; i++) {
    const ch = str.charCodeAt(i)
    h1 = Math.imul(h1 ^ ch, 2654435761)
    h2 = Math.imul(h2 ^ ch, 1597334677)
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507)
  h1 ^= Math.imul(h2 ^ (h2 >>> 13), 3266489909)
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507)
  h2 ^= Math.imul(h1 ^ (h1 >>> 13), 3266489909)
  return 4294967296 * (2097151 & h2) + (h1 >>> 0)
}

/** mulberry32 确定性 PRNG，输入 uint32 种子，输出 () => [0,1) */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return function () {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** 无序对 key（忽略方向），用于真实边去重 */
function unorderedKey(a: string, b: string): string {
  return a < b ? `${a}\0${b}` : `${b}\0${a}`
}

/** Fisher-Yates 洗牌（用种子 PRNG，确定性） */
function shuffle<T>(arr: T[], rnd: () => number): void {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(rnd() * (i + 1))
    ;[arr[i], arr[j]] = [arr[j], arr[i]]
  }
}

/** 从边的 source/target 提取节点名（force-graph 可能已把它替换为节点对象） */
function nodeName(v: string | GraphNodeView): string {
  if (typeof v === 'object') return (v as any).id ?? (v as any).name ?? ''
  return v
}

/**
 * 生成合成弱边。
 * @param nodes     当前全部节点
 * @param realEdges 真实边（后端 RelationFound）
 * @returns 合成弱边数组（可能为空）
 */
export function synthesizeEdges(
  nodes: GraphNodeView[],
  realEdges: GraphEdgeView[],
): GraphEdgeView[] {
  const N = nodes.length
  if (N < 2) return []
  const R = realEdges.length

  // 已连接的无序对（真实边）
  const connected = new Set<string>()
  for (const e of realEdges) {
    const s = nodeName(e.source)
    const t = nodeName(e.target)
    if (s && t) connected.add(unorderedKey(s, t))
  }

  // 种子：排序后的实体名列表 → 同输入同序列
  const names = nodes.map((n) => n.name).sort()
  const seed = cyrb53(names.join('\0'))
  const rnd = mulberry32(seed)

  // 随机目标密度系数（种子随机），过低才补边
  const factor = DENSITY_MIN + rnd() * (DENSITY_MAX - DENSITY_MIN)
  const target = Math.ceil(N * factor)
  const need = target - R
  if (need <= 0) return []

  // 按实体类型分组，用于「同类型弱连」优先
  const byType = new Map<string, GraphNodeView[]>()
  for (const n of nodes) {
    const arr = byType.get(n.entityType) ?? []
    arr.push(n)
    byType.set(n.entityType, arr)
  }

  const sameTypePairs: Array<[string, string]> = []
  const crossTypePairs: Array<[string, string]> = []
  for (const group of byType.values()) {
    for (let i = 0; i < group.length; i++) {
      for (let j = i + 1; j < group.length; j++) {
        const a = group[i].name
        const b = group[j].name
        const key = unorderedKey(a, b)
        if (!connected.has(key)) sameTypePairs.push([a, b])
      }
    }
  }
  for (let i = 0; i < N; i++) {
    for (let j = i + 1; j < N; j++) {
      const a = nodes[i].name
      const b = nodes[j].name
      if (nodes[i].entityType === nodes[j].entityType) continue // 已在 sameType 处理
      const key = unorderedKey(a, b)
      if (!connected.has(key)) crossTypePairs.push([a, b])
    }
  }

  // 种子随机打乱（确定性），同类型优先于跨类型
  shuffle(sameTypePairs, rnd)
  shuffle(crossTypePairs, rnd)
  const candidates = sameTypePairs.concat(crossTypePairs)
  const count = Math.min(need, candidates.length)

  const result: GraphEdgeView[] = []
  for (let i = 0; i < count; i++) {
    const [s, t] = candidates[i]
    const strength = STRENGTH_MIN + rnd() * (STRENGTH_MAX - STRENGTH_MIN)
    result.push({
      id: `__syn__${s}\0${t}`,
      source: s,
      target: t,
      relationType: '__SYNTHETIC__',
      strength,
      synthetic: true,
    })
  }
  return result
}
