export type ThemeMode = 'light' | 'dark'
export type ThemePreset = 'default' | 'cool' | 'galaxy' | 'deepseek' | 'newapi' | 'shadcn'

export const DEFAULT_THEME_MODE: ThemeMode = 'light'
export const DEFAULT_THEME_PRESET: ThemePreset = 'newapi'

export const MODE_STORAGE_KEY = 'xinghai.theme'
export const PRESET_STORAGE_KEY = 'xinghai.preset'

/**
 * Labels live in the `theme` i18n namespace, keyed as `theme.<value>Label`.
 *
 * The swatches are the only hard-coded colours in the app: they preview a theme
 * that is not currently applied, so they cannot come from CSS variables. Keep
 * them in sync with `assets/css/themes/<value>.css`.
 */
export const THEME_PRESETS: { value: ThemePreset; swatch: string[] }[] = [
  { value: 'newapi', swatch: ['#f7f8fa', '#3563e9', '#1c2434'] },
  { value: 'shadcn', swatch: ['#ffffff', '#18181b', '#09090b'] },
  { value: 'default', swatch: ['#faf9f5', '#c96442', '#141413'] },
  { value: 'cool', swatch: ['#08090f', '#7c7cff', '#2fd3a5'] },
  { value: 'galaxy', swatch: ['#0a0a18', '#a78bfa', '#4f82f6'] },
  { value: 'deepseek', swatch: ['#0a0a0a', '#6799fe', '#f9f8f8'] },
]

const PRESET_VALUES = THEME_PRESETS.map(preset => preset.value)

const mode = ref<ThemeMode>(DEFAULT_THEME_MODE)
const preset = ref<ThemePreset>(DEFAULT_THEME_PRESET)

function apply() {
  if (!import.meta.client) return
  document.documentElement.dataset.theme = mode.value
  document.documentElement.dataset.preset = preset.value
}

export function useTheme() {
  function setMode(next: ThemeMode) {
    mode.value = next
    apply()
    if (import.meta.client) {
      try { localStorage.setItem(MODE_STORAGE_KEY, next) } catch { return }
    }
  }

  function toggleMode() {
    setMode(mode.value === 'dark' ? 'light' : 'dark')
  }

  function setPreset(next: ThemePreset) {
    preset.value = next
    apply()
    if (import.meta.client) {
      try { localStorage.setItem(PRESET_STORAGE_KEY, next) } catch { return }
    }
  }

  function initializeTheme() {
    if (!import.meta.client) return
    try {
      const storedMode = localStorage.getItem(MODE_STORAGE_KEY)
      mode.value = storedMode === 'light' || storedMode === 'dark' ? storedMode : DEFAULT_THEME_MODE
      const storedPreset = localStorage.getItem(PRESET_STORAGE_KEY) as ThemePreset | null
      preset.value = storedPreset && PRESET_VALUES.includes(storedPreset) ? storedPreset : DEFAULT_THEME_PRESET
    } catch {
      mode.value = DEFAULT_THEME_MODE
      preset.value = DEFAULT_THEME_PRESET
    }
    apply()
  }

  return {
    mode: readonly(mode),
    preset: readonly(preset),
    presets: THEME_PRESETS,
    setMode,
    toggleMode,
    setPreset,
    initializeTheme,
  }
}
