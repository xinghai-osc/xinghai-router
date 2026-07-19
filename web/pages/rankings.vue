<script setup lang="ts">
import { ArrowDown, ArrowUp, RefreshCw, Trophy } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import type { Rankings, SiteSettings } from '~/src/api'
import { Button } from '@/components/ui/button'

type Period = 'today' | 'week' | 'month' | 'year'
const period = ref<Period>('week')
const rankings = ref<Rankings | null>(null)
const siteSettings = ref<SiteSettings>({ name: 'Xinghai Router', icon_url: '', auto_disable_failed_channels: false })
const loading = ref(true)
const error = ref('')
const { locale, t, initializeLocale } = useI18n()
const periods: { value: Period; label: string }[] = [{ value: 'today', label: t('today') }, { value: 'week', label: t('thisWeek') }, { value: 'month', label: t('thisMonth') }, { value: 'year', label: t('thisYear') }]

const compactNumber = (value: number) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
const change = (value: number) => `${value >= 0 ? '+' : ''}${value.toFixed(1)}%`
const share = (value: number) => value > 0 && value < .001 ? '<0.1%' : `${(value * 100).toFixed(1)}%`

async function load(next = period.value) {
  period.value = next
  loading.value = true
  error.value = ''
  try {
    const response = await fetch(`/api/rankings?period=${next}`)
    if (!response.ok) throw new Error(t('rankingsUnavailable'))
    rankings.value = await response.json()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('rankingsUnavailable')
  } finally {
    loading.value = false
  }
}
async function loadSiteSettings() {
  const response = await fetch('/api/site-settings')
  if (!response.ok) return
  siteSettings.value = await response.json() as SiteSettings
  document.title = `${siteSettings.value.name} · ${t('titleRankings')}`
  if (siteSettings.value.icon_url) {
    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]') ?? document.head.appendChild(Object.assign(document.createElement('link'), { rel: 'icon' }))
    link.href = siteSettings.value.icon_url
  }
}

function selectPeriod(next: Period) {
  history.replaceState({}, '', `/rankings?period=${next}`)
  load(next)
}

onMounted(() => {
  initializeLocale()
  const queryPeriod = new URLSearchParams(location.search).get('period') as Period | null
  if (queryPeriod && periods.some((item) => item.value === queryPeriod)) period.value = queryPeriod
  loadSiteSettings()
  load()
})
</script>

<template>
  <main class="rankings-page">
     <PublicTopbar />

    <section class="rankings-shell">
      <header class="rankings-hero">
        <div><span><Trophy :size="15" /> LIVE USAGE RANKINGS</span><h1>{{ t('modelRankingsTitle') }}</h1><p>{{ t('rankingsDesc') }}</p></div>
        <div v-if="rankings" class="ranking-total"><strong>{{ compactNumber(rankings.total_tokens) }}</strong><small>{{ t('periodTokens') }}</small></div>
      </header>
      <div class="rankings-controls"><div><span>{{ t('timeRange') }}</span><div class="flex gap-1 rounded-lg border border-border bg-card p-1"><button v-for="item in periods" :key="item.value" class="min-w-[58px] rounded-md px-3 py-1.5 text-[10px] font-bold transition-colors disabled:opacity-50" :class="period === item.value ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent'" :disabled="loading" @click="selectPeriod(item.value)">{{ item.label }}</button></div></div><Button variant="ghost" size="sm" class="ranking-refresh" :disabled="loading" @click="load()"><RefreshCw :size="15" :class="{ 'animate-spin': loading }" />{{ t('refreshData') }}</Button></div>

      <div v-if="loading && !rankings" class="rankings-loading"><div/><div/><div/></div>
      <section v-else-if="error && !rankings" class="rankings-error"><h2>{{ t('cannotLoadRankings') }}</h2><p>{{ error }}</p><Button variant="outline" @click="load()">{{ t('reload') }}</Button></section>
      <template v-else-if="rankings">
        <section class="ranking-panel">
          <div class="ranking-panel-title"><div><span>TOP MODELS</span><h2>{{ t('llmRankings') }}</h2><p>{{ t('rankingsSortByTokens') }}</p></div><b>{{ rankings.models.length }} {{ t('modelCount') }}</b></div>
          <div v-if="rankings.models.length" class="model-rankings"><article v-for="item in rankings.models" :key="item.model_name"><b :class="['rank-number', { podium: item.rank <= 3 }]">{{ String(item.rank).padStart(2, '0') }}</b><div class="rank-model"><i>{{ item.model_name.slice(0, 1).toUpperCase() }}</i><span><strong>{{ item.model_name }}</strong><small>{{ item.vendor }}</small></span></div><div class="rank-share"><i><span :style="{ width: `${Math.max(item.share * 100, 1)}%` }"/></i><small>{{ share(item.share) }}</small></div><div class="rank-tokens"><strong>{{ compactNumber(item.total_tokens) }}</strong><small>Token</small></div><em :class="item.growth_pct < 0 ? 'down' : 'up'"><ArrowDown v-if="item.growth_pct < 0" :size="11" /><ArrowUp v-else :size="11" />{{ change(item.growth_pct) }}</em></article></div>
          <div v-else class="ranking-empty">{{ t('noModelUsage') }}</div>
        </section>

        <section class="ranking-panel">
          <div class="ranking-panel-title"><div><span>MARKET SHARE</span><h2>{{ t('vendorShare') }}</h2><p>{{ t('vendorShareDesc') }}</p></div></div>
          <div v-if="rankings.vendors.length" class="vendor-list"><article v-for="item in rankings.vendors.slice(0, 12)" :key="item.vendor"><b>{{ String(item.rank).padStart(2, '0') }}</b><div><strong>{{ item.vendor }}</strong><small>{{ item.models_count }} {{ t('modelCount') }} · {{ t('topModelLabel') }} {{ item.top_model }}</small></div><i><span :style="{ width: `${item.share * 100}%` }"/></i><span>{{ share(item.share) }}</span><em :class="item.growth_pct < 0 ? 'down' : 'up'">{{ change(item.growth_pct) }}</em></article></div>
          <div v-else class="ranking-empty">{{ t('noVendorData') }}</div>
        </section>

        <div class="movers-grid"><section class="ranking-panel"><div class="ranking-panel-title compact"><div><span>TRENDING UP</span><h2>{{ t('trendingUp') }}</h2></div><ArrowUp :size="18" /></div><div class="mover-list"><article v-for="item in rankings.top_movers" :key="item.model_name"><div><strong>{{ item.model_name }}</strong><small>#{{ item.current_rank }} · {{ item.vendor }}</small></div><b class="up"><ArrowUp :size="13" />{{ item.rank_delta }}</b></article><div v-if="!rankings.top_movers.length" class="ranking-empty small">{{ t('noTrendingUpModels') }}</div></div></section><section class="ranking-panel"><div class="ranking-panel-title compact"><div><span>TRENDING DOWN</span><h2>{{ t('trendingDown') }}</h2></div><ArrowDown :size="18" /></div><div class="mover-list"><article v-for="item in rankings.top_droppers" :key="item.model_name"><div><strong>{{ item.model_name }}</strong><small>#{{ item.current_rank }} · {{ item.vendor }}</small></div><b class="down"><ArrowDown :size="13" />{{ Math.abs(item.rank_delta) }}</b></article><div v-if="!rankings.top_droppers.length" class="ranking-empty small">{{ t('noTrendingDownModels') }}</div></div></section></div>
        <p class="rankings-updated">{{ t('dataUpdatedAt') }} {{ new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(rankings.updated_at)) }}</p>
      </template>
    </section>
  </main>
</template>

<style scoped>
.rankings-page{min-height:100vh;color:#27272a;background:radial-gradient(circle at 50% -10%,#e9eaed 0,transparent 32%),#f8f8f7}.rankings-nav{display:flex;align-items:center;justify-content:space-between;width:min(1180px,calc(100% - 48px));height:78px;margin:auto}.rankings-nav>div{display:flex;align-items:center;gap:10px}.back-link{display:flex;align-items:center;gap:5px;padding:8px;color:#71717a;font-size:11px;font-weight:700;text-decoration:none}.rankings-nav .button{text-decoration:none}.rankings-shell{width:min(1180px,calc(100% - 48px));margin:auto;padding:58px 0 70px}.rankings-hero{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:34px}.rankings-hero>div:first-child>span,.ranking-panel-title>div>span{display:flex;align-items:center;gap:6px;color:#71717a;font:600 9px "DM Mono",monospace;letter-spacing:.14em}.rankings-hero h1{margin:13px 0 9px;color:#18181b;font-size:clamp(38px,6vw,66px);letter-spacing:-.075em}.rankings-hero p{max-width:620px;margin:0;color:#71717a;font-size:14px;line-height:1.8}.ranking-total{display:grid;text-align:right}.ranking-total strong{color:#18181b;font:500 38px "DM Mono",monospace;letter-spacing:-.07em}.ranking-total small{color:#a1a1aa;font-size:10px}.rankings-controls{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:16px;padding-bottom:16px;border-bottom:1px solid #e4e4e7}.rankings-controls>div>span{display:block;margin-bottom:8px;color:#a1a1aa;font-size:10px;font-weight:700}.period-tabs{display:flex;gap:4px;padding:4px;border:1px solid #e4e4e7;border-radius:8px;background:#fff}.period-tabs button{min-width:58px;height:30px;padding:0 11px;border:0;border-radius:5px;color:#71717a;background:transparent;font-size:10px;font-weight:700}.period-tabs button.active{color:#fff;background:#27272a}.ranking-refresh{display:flex;align-items:center;gap:6px;height:34px;padding:0;border:0;color:#71717a;background:transparent;font-size:10px;font-weight:700}.ranking-panel{overflow:hidden;margin-bottom:16px;border:1px solid #e4e4e7;border-radius:12px;background:#fff}.ranking-panel-title{display:flex;align-items:center;justify-content:space-between;min-height:92px;padding:20px 22px;border-bottom:1px solid #ececef}.ranking-panel-title h2{margin:6px 0 4px;color:#27272a;font-size:17px}.ranking-panel-title p{margin:0;color:#a1a1aa;font-size:10px}.ranking-panel-title>b{color:#a1a1aa;font:500 10px "DM Mono",monospace}.ranking-panel-title.compact{min-height:78px}.model-rankings{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}.model-rankings article{display:grid;grid-template-columns:28px minmax(130px,1fr) minmax(80px,.7fr) 65px 64px;gap:10px;align-items:center;min-height:74px;padding:10px 18px;border-bottom:1px solid #f0f0f2}.model-rankings article:nth-child(odd){border-right:1px solid #f0f0f2}.rank-number{color:#a1a1aa;font:500 10px "DM Mono",monospace}.rank-number.podium{color:#18181b}.rank-model{display:flex;min-width:0;align-items:center;gap:9px}.rank-model>i{display:grid;flex:none;width:30px;height:30px;place-items:center;border-radius:7px;color:#fff;background:#27272a;font:600 11px "DM Mono",monospace;font-style:normal}.rank-model>span{min-width:0}.rank-model strong,.rank-model small,.rank-tokens strong,.rank-tokens small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.rank-model strong{color:#27272a;font:600 10px "DM Mono",monospace}.rank-model small,.rank-tokens small{margin-top:3px;color:#a1a1aa;font-size:8px}.rank-share{display:flex;align-items:center;gap:6px}.rank-share>i,.vendor-list article>i{width:100%;height:4px;overflow:hidden;border-radius:99px;background:#f0f0f2}.rank-share>i span,.vendor-list article>i span{display:block;height:100%;border-radius:inherit;background:#52525b}.rank-share small{color:#a1a1aa;font:8px "DM Mono",monospace}.rank-tokens{text-align:right}.rank-tokens strong{color:#27272a;font:600 10px "DM Mono",monospace}.model-rankings em,.vendor-list em{display:flex;align-items:center;justify-content:flex-end;gap:2px;font:600 8px "DM Mono",monospace;font-style:normal}.up{color:#15803d}.down{color:#b91c1c}.vendor-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}.vendor-list article{display:grid;grid-template-columns:26px minmax(120px,1fr) minmax(70px,.6fr) 42px 56px;gap:10px;align-items:center;min-height:68px;padding:10px 20px;border-bottom:1px solid #f0f0f2}.vendor-list article:nth-child(odd){border-right:1px solid #f0f0f2}.vendor-list article>b{color:#a1a1aa;font:500 10px "DM Mono",monospace}.vendor-list article div{min-width:0}.vendor-list strong,.vendor-list small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.vendor-list strong{color:#27272a;font-size:11px}.vendor-list small{margin-top:3px;color:#a1a1aa;font-size:8px}.vendor-list article>span{color:#52525b;text-align:right;font:600 9px "DM Mono",monospace}.movers-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.mover-list article{display:flex;align-items:center;justify-content:space-between;min-height:62px;padding:10px 20px;border-bottom:1px solid #f0f0f2}.mover-list strong,.mover-list small{display:block}.mover-list strong{color:#27272a;font:600 10px "DM Mono",monospace}.mover-list small{margin-top:4px;color:#a1a1aa;font-size:9px}.mover-list b{display:flex;align-items:center;gap:3px;font:600 11px "DM Mono",monospace}.ranking-empty{display:grid;min-height:220px;place-items:center;color:#a1a1aa;font-size:11px}.ranking-empty.small{min-height:140px}.rankings-updated{margin:24px 0 0;color:#a1a1aa;text-align:center;font-size:9px}.rankings-loading{display:grid;gap:16px}.rankings-loading div{height:280px;border:1px solid #e4e4e7;border-radius:12px;background:linear-gradient(100deg,#fff 30%,#f4f4f5 50%,#fff 70%);background-size:200% 100%;animation:pulse 1.5s infinite}.rankings-loading div:last-child{height:180px}.rankings-error{display:grid;min-height:360px;place-items:center;align-content:center;border:1px dashed #d4d4d8;border-radius:12px;background:#fff}.rankings-error h2{margin:0;font-size:16px}.rankings-error p{color:#71717a;font-size:11px}@keyframes pulse{to{background-position:-200% 0}}
:global(:root[data-theme="dark"]) .rankings-page{color:#d4d4d8;background:radial-gradient(circle at 50% -10%,#242429 0,transparent 32%),#111113}:global(:root[data-theme="dark"]) .rankings-hero h1,:global(:root[data-theme="dark"]) .ranking-total strong,:global(:root[data-theme="dark"]) .ranking-panel-title h2,:global(:root[data-theme="dark"]) .rank-model strong,:global(:root[data-theme="dark"]) .rank-tokens strong,:global(:root[data-theme="dark"]) .vendor-list strong,:global(:root[data-theme="dark"]) .mover-list strong{color:#f4f4f5}:global(:root[data-theme="dark"]) .rankings-controls,:global(:root[data-theme="dark"]) .ranking-panel-title,:global(:root[data-theme="dark"]) .model-rankings article,:global(:root[data-theme="dark"]) .vendor-list article,:global(:root[data-theme="dark"]) .mover-list article{border-color:#303036}:global(:root[data-theme="dark"]) .period-tabs,:global(:root[data-theme="dark"]) .ranking-panel,:global(:root[data-theme="dark"]) .rankings-error{border-color:#303036;background:#18181b}:global(:root[data-theme="dark"]) .period-tabs button.active{color:#18181b;background:#e4e4e7}:global(:root[data-theme="dark"]) .rank-model>i{color:#18181b;background:#e4e4e7}:global(:root[data-theme="dark"]) .rank-share>i,:global(:root[data-theme="dark"]) .vendor-list article>i{background:#303036}:global(:root[data-theme="dark"]) .rank-share>i span,:global(:root[data-theme="dark"]) .vendor-list article>i span{background:#d4d4d8}
@media(max-width:1050px){.model-rankings{grid-template-columns:1fr}.model-rankings article:nth-child(odd){border-right:0}}@media(max-width:700px){.rankings-nav,.rankings-shell{width:calc(100% - 28px)}.back-link{display:none}.rankings-shell{padding-top:35px}.rankings-hero{align-items:flex-start;flex-direction:column;gap:20px}.ranking-total{text-align:left}.ranking-refresh{font-size:0}.model-rankings article{grid-template-columns:24px minmax(110px,1fr) 60px 54px;padding:9px 12px}.rank-share{display:none}.vendor-list{grid-template-columns:1fr}.vendor-list article:nth-child(odd){border-right:0}.movers-grid{grid-template-columns:1fr}.rankings-hero p{font-size:12px}}@media(max-width:430px){.rankings-nav .landing-logo i{display:none}.period-tabs button{min-width:47px;padding:0 6px}.model-rankings article{grid-template-columns:20px minmax(100px,1fr) 54px}.model-rankings em{display:none}.rank-model>i{display:none}.vendor-list article{grid-template-columns:22px minmax(100px,1fr) 40px 50px}.vendor-list article>i{display:none}}
</style>
