<script setup lang="ts">
import { MODE_STORAGE_KEY, PRESET_STORAGE_KEY } from '~/composables/useTheme'

const noFlashTheme = `(()=>{try{const d=document.documentElement,m=localStorage.getItem('${MODE_STORAGE_KEY}'),p=localStorage.getItem('${PRESET_STORAGE_KEY}');d.dataset.theme=m==='dark'||m==='light'?m:(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');d.dataset.preset=['default','cool','galaxy'].includes(p)?p:'default'}catch(e){document.documentElement.dataset.theme='light';document.documentElement.dataset.preset='default'}})()`

useHead({
  script: [{ innerHTML: noFlashTheme, tagPosition: 'head' }],
  meta: [{ name: 'color-scheme', content: 'light dark' }],
})

const { initializeTheme } = useTheme()
const { initializeLocale } = useI18n()

onMounted(() => {
  initializeTheme()
  initializeLocale()
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
