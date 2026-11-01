import { ref, watch } from 'vue'

/**
 * Card color presets available for KB / Agent cards.
 * Each preset defines a gradient background and an accent color for icons.
 */
export interface CardColorPreset {
  key: string
  label: string
  /** CSS gradient for card background */
  bg: string
  /** Accent color for decorative pseudo-element */
  accent: string
  /** Hover state gradient */
  hoverBg: string
}

export const CARD_COLOR_PRESETS: CardColorPreset[] = [
  {
    key: 'theme-purple',
    label: '主题紫色',
    bg: 'linear-gradient(135deg, rgba(102, 126, 234, 0.10) 0%, rgba(118, 75, 162, 0.08) 50%, rgba(245, 192, 240, 0.10) 100%)',
    accent: 'rgba(118, 75, 162, 0.18)',
    hoverBg: 'linear-gradient(135deg, rgba(102, 126, 234, 0.18) 0%, rgba(118, 75, 162, 0.14) 50%, rgba(245, 192, 240, 0.16) 100%)',
  },
  {
    key: 'theme-blue',
    label: '主题蓝色',
    bg: 'linear-gradient(135deg, rgba(59, 130, 246, 0.06) 0%, rgba(99, 102, 241, 0.10) 50%, rgba(102, 126, 234, 0.08) 100%)',
    accent: 'rgba(99, 102, 241, 0.18)',
    hoverBg: 'linear-gradient(135deg, rgba(59, 130, 246, 0.10) 0%, rgba(99, 102, 241, 0.16) 50%, rgba(102, 126, 234, 0.12) 100%)',
  },
  {
    key: 'goose-yellow',
    label: '鹅黄色',
    bg: 'linear-gradient(135deg, rgba(255, 215, 100, 0.10) 0%, rgba(255, 193, 7, 0.08) 50%, rgba(255, 235, 150, 0.10) 100%)',
    accent: 'rgba(255, 193, 7, 0.18)',
    hoverBg: 'linear-gradient(135deg, rgba(255, 215, 100, 0.18) 0%, rgba(255, 193, 7, 0.14) 50%, rgba(255, 235, 150, 0.16) 100%)',
  },
  {
    key: 'mint-green',
    label: '薄荷绿色',
    bg: 'linear-gradient(135deg, rgba(7, 192, 95, 0.08) 0%, rgba(0, 150, 136, 0.08) 50%, rgba(129, 199, 132, 0.10) 100%)',
    accent: 'rgba(7, 192, 95, 0.18)',
    hoverBg: 'linear-gradient(135deg, rgba(7, 192, 95, 0.14) 0%, rgba(0, 150, 136, 0.14) 50%, rgba(129, 199, 132, 0.16) 100%)',
  },
]

const STORAGE_KEY_KB = 'weknora-card-colors-kb'
const STORAGE_KEY_AGENT = 'weknora-card-colors-agent'

function loadMap(key: string): Record<string, string> {
  try {
    const raw = localStorage.getItem(key)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

function saveMap(key: string, map: Record<string, string>) {
  localStorage.setItem(key, JSON.stringify(map))
}

/**
 * Composable for managing per-card color labels.
 * Colors are persisted in localStorage, keyed by card ID.
 */
export function useCardColors(type: 'kb' | 'agent' = 'kb') {
  const storageKey = type === 'kb' ? STORAGE_KEY_KB : STORAGE_KEY_AGENT
  const colorMap = ref<Record<string, string>>(loadMap(storageKey))

  watch(colorMap, (val) => {
    saveMap(storageKey, val)
  }, { deep: true })

  function getColor(id: string): string | undefined {
    return colorMap.value[id]
  }

  function setColor(id: string, colorKey: string) {
    colorMap.value[id] = colorKey
  }

  function clearColor(id: string) {
    delete colorMap.value[id]
  }

  function getPreset(colorKey: string): CardColorPreset | undefined {
    return CARD_COLOR_PRESETS.find((p) => p.key === colorKey)
  }

  return { colorMap, getColor, setColor, clearColor, getPreset }
}