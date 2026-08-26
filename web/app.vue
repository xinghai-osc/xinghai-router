<script setup lang="ts">
import { MODE_STORAGE_KEY, PRESET_STORAGE_KEY } from '~/composables/useTheme'

const noFlashTheme = `(()=>{try{const d=document.documentElement,m=localStorage.getItem('${MODE_STORAGE_KEY}'),p=localStorage.getItem('${PRESET_STORAGE_KEY}');d.dataset.theme=m==='dark'||m==='light'?m:'dark';d.dataset.preset=['default','cool','galaxy','deepseek'].includes(p)?p:'deepseek'}catch(e){document.documentElement.dataset.theme='dark';document.documentElement.dataset.preset='deepseek'}})()`
const { settings, loadSiteSettings } = useSiteSettings()

useHead({
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
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
