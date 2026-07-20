<script setup lang="ts">
import { computed } from 'vue'
import { TooltipProvider } from '@/components/ui/tooltip'

const route = useRoute()
const { t } = useI18n()
const { initializeTheme } = useTheme()
const consoleTitles: Record<string, string> = {
  overview: 'overview', users: 'users', groups: 'groups', keys: 'keys', channels: 'channels', providers: 'modelProviders', logs: 'logs', account: 'account', profile: 'profile', wallet: 'wallet', usage: 'usage', 'usage-overview': 'usageOverview', ledger: 'ledger', pricing: 'pricing', audit: 'audit', reliability: 'reliability',
}

const title = computed(() => {
  if (route.path === '/') return 'Xinghai Router'
  if (route.path === '/auth') return `${t('titleLogin')} | Xinghai Router`
  if (route.path === '/models') return `${t('titleMarketplace')} | Xinghai Router`
  if (route.path === '/rankings') return `${t('titleRankings')} | Xinghai Router`
  if (route.path === '/terms') return `${t('termsTitle')} | Xinghai Router`
  if (route.path === '/privacy') return `${t('privacyTitle')} | Xinghai Router`

  const queryView = route.query.view
  const view = route.path === '/console'
    ? typeof queryView === 'string' ? queryView : 'overview'
    : typeof route.params.view === 'string' ? route.params.view : route.path.slice('/console/'.length)
  return `${t((consoleTitles[view] ?? 'overview') as Parameters<typeof t>[0])} | Xinghai Router`
})

// 在绘制前同步恢复主题，避免从控制台跳转到外部页面时先闪一下浅色模式
const themeInitScript = `(function(){try{var d=document.documentElement;var s={};try{s=JSON.parse(localStorage.getItem('xinghai-router-theme-config')||'{}')}catch(e){}var m=s.mode;var l=localStorage.getItem('xinghai-router-theme');if(['light','dark','system'].indexOf(m)<0)m=(l==='light'||l==='dark')?l:'system';var r=m==='system'?(window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'):m;d.dataset.theme=r;d.dataset.themeMode=m;d.dataset.themeColor=['neutral','blue','green','orange','rose','violet'].indexOf(s.color)>=0?s.color:'neutral';d.dataset.themeRadius=['none','small','medium','large'].indexOf(s.radius)>=0?s.radius:'medium';d.dataset.themePreset=['custom','a-site'].indexOf(s.preset)>=0?s.preset:'custom';d.style.colorScheme=r}catch(e){}})();`

useHead({
  title,
  script: [{ innerHTML: themeInitScript, tagPriority: 'critical' }],
})
onMounted(initializeTheme)
</script>

<template>
  <TooltipProvider :delay-duration="200">
    <NuxtPage />
  </TooltipProvider>
</template>
