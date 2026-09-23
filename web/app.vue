<script setup lang="ts">
import { MODE_STORAGE_KEY, PRESET_STORAGE_KEY } from '~/composables/useTheme'
import { clearLegacySession, setReauthenticationHandler } from '~/src/api'

const noFlashTheme = `(()=>{try{const d=document.documentElement,m=localStorage.getItem('${MODE_STORAGE_KEY}'),p=localStorage.getItem('${PRESET_STORAGE_KEY}');d.dataset.theme=m==='dark'||m==='light'?m:'dark';d.dataset.preset=['default','cool','galaxy','deepseek'].includes(p)?p:'deepseek'}catch(e){document.documentElement.dataset.theme='dark';document.documentElement.dataset.preset='deepseek'}})()`
const { settings, loadSiteSettings } = useSiteSettings()
const { locale } = useI18n()
const { loadAccount } = useAccount()
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
    <NuxtPage />
  </NuxtLayout>
  <ClientOnly>
    <SessionReauthenticationDialog />
  </ClientOnly>
</template>
