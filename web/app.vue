<script setup lang="ts">
import { DEFAULT_THEME_MODE, DEFAULT_THEME_PRESET, MODE_STORAGE_KEY, PRESET_STORAGE_KEY, THEME_PRESETS } from '~/composables/useTheme'
import { clearLegacySession, setReauthenticationHandler } from '~/src/api'

const noFlashTheme = `(()=>{const d=document.documentElement;try{const m=localStorage.getItem('${MODE_STORAGE_KEY}'),p=localStorage.getItem('${PRESET_STORAGE_KEY}');d.dataset.theme=m==='dark'||m==='light'?m:'${DEFAULT_THEME_MODE}';d.dataset.preset=${JSON.stringify(THEME_PRESETS.map(preset => preset.value))}.includes(p)?p:'${DEFAULT_THEME_PRESET}'}catch(e){d.dataset.theme='${DEFAULT_THEME_MODE}';d.dataset.preset='${DEFAULT_THEME_PRESET}'}})()`
const { settings, loadSiteSettings } = useSiteSettings()
const { locale } = useI18n()
const { loadAccount } = useAccount()
const { pageKey: workspacePageKey } = useWorkspace()
const { request: requestReauthentication, cancel: cancelReauthentication } = useReauthentication()

if (import.meta.client) {
  clearLegacySession()
  setReauthenticationHandler(requestReauthentication)
}

onBeforeUnmount(() => {
  setReauthenticationHandler(null)
  cancelReauthentication()
})

useHead({
  htmlAttrs: { lang: computed(() => locale.value === 'en' ? 'en' : locale.value === 'zh-Hant' ? 'zh-TW' : 'zh-CN') },
  link: [{ rel: 'icon', type: 'image/svg+xml', href: computed(() => settings.value.icon_url || '/favicon.svg') }],
  script: [{ innerHTML: noFlashTheme, tagPosition: 'head' }],
  meta: [{ name: 'color-scheme', content: 'light dark' }],
})

const { initializeTheme } = useTheme()
const { initializeLocale } = useI18n()

onMounted(() => {
  initializeTheme()
  initializeLocale()
  loadSiteSettings()
  loadAccount()
})
</script>

<template>
  <NuxtLoadingIndicator color="var(--clay)" error-color="var(--danger)" :height="3" :throttle="120" />
  <NuxtLayout>
    <NuxtPage :page-key="route => route.path.startsWith('/console') ? `${route.path}:${workspacePageKey}` : route.path" />
  </NuxtLayout>
  <ClientOnly>
    <SessionReauthenticationDialog />
  </ClientOnly>
</template>
