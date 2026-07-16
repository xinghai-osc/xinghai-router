<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { Activity, Bot, Check, ChevronRight, CircleAlert, Copy, KeyRound, LayoutDashboard, LogOut, Moon, Plus, RadioTower, RefreshCw, Search, Sparkles, Sun, TerminalSquare, UserRound, Users, WalletCards, ReceiptText, Tags } from 'lucide-vue-next'
import { api, clearToken, getToken, setToken } from '~/src/api'
import type { Account, ApiKey, CatalogGroup, CatalogModel, Channel, Group, LedgerEntry, Pricing, RequestLog, UsageRecord, User } from '~/src/api'

type View = 'overview' | 'users' | 'keys' | 'channels' | 'logs' | 'account' | 'profile' | 'wallet' | 'usage' | 'ledger' | 'pricing' | 'audit'
const props = withDefaults(defineProps<{ activeView?: View }>(), { activeView: 'overview' })
const route = useRoute()
const router = useRouter()
const views: View[] = ['overview', 'users', 'keys', 'channels', 'logs', 'account', 'profile', 'wallet', 'usage', 'ledger', 'pricing', 'audit']
const view = computed<View>(() => {
  if (views.includes(route.query.view as View)) return route.query.view as View
  if (props.activeView && views.includes(props.activeView)) return props.activeView
  return views.includes(route.params.view as View) ? route.params.view as View : 'overview'
})
const authenticated = ref(false)
const error = ref('')
const busy = ref(false)
const users = ref<User[]>([])
const groups = ref<Group[]>([])
const ownGroups = ref<string[]>([])
const keys = ref<ApiKey[]>([])
const accountKeys = ref<ApiKey[]>([])
const channels = ref<Channel[]>([])
const logs = ref<RequestLog[]>([])
const account = ref<Account | null>(null)
const usageRecords = ref<UsageRecord[]>([])
const ledger = ref<LedgerEntry[]>([])
const pricing = ref<Pricing[]>([])
const catalog = ref<CatalogModel[]>([])
const catalogGroups = ref<CatalogGroup[]>([])
const catalogGroup = ref('all')
const catalogSearch = ref('')
const auditLogs = ref<Record<string, unknown>[]>([])
const createdKey = ref('')
const showKey = ref(false)
const showAccountKey = ref(false)
const showChannel = ref(false)
const selectedUser = ref<User | null>(null)
const selectedPermissions = ref<string[]>([])
const selectedGroups = ref<string[]>([])
const keyForm = reactive({ user_id: '', name: '', expires_at: '', group_id: '' })
const accountKeyForm = reactive({ name: '', expires_at: '', group_id: '' })
const channelForm = reactive({ name: '', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] as string[] })

const generalNav = [['overview', '概览', LayoutDashboard], ['account', 'API 密钥', KeyRound], ['usage', '用量日志', Activity]] as const
const billingNav = [['wallet', '钱包', WalletCards], ['ledger', '余额流水', ReceiptText]] as const
const personalNav = [['profile', '个人资料', UserRound]] as const
const managementNavItems = [
  ['users', '用户', Users, 'users.read'], ['keys', 'API 密钥', KeyRound, 'keys.manage'], ['channels', '渠道', RadioTower, 'channels.read'], ['logs', '请求日志', TerminalSquare, 'logs.read'],
] as const
const adminExtraNav = [['pricing', '模型定价', Tags, 'pricing.read'], ['audit', '操作审计', ReceiptText, 'audit.read']] as const
const permissions = ['users.read', 'users.manage', 'keys.manage', 'channels.read', 'channels.manage', 'logs.read', 'pricing.read', 'pricing.manage', 'audit.read', 'wallets.manage', 'routes.manage', 'quotas.manage', 'system.manage']
const pricingForm = reactive({ model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 })
const loginMode = ref<'token' | 'login' | 'register'>('token')
const theme = ref<'light' | 'dark'>('light')
const accountForm = reactive({ name: '', email: '', password: '' })
const isLanding = computed(() => route.path === '/')
const isAuthPage = computed(() => route.path === '/auth')
const isMarketplacePage = computed(() => route.path === '/models')
const activeChannels = computed(() => channels.value.filter((channel) => channel.enabled).length)
const successRate = computed(() => logs.value.length ? Math.round(logs.value.filter((log) => log.status_code < 400).length / logs.value.length * 100) : 100)
const totalTokens = computed(() => logs.value.reduce((sum, log) => sum + (log.total_tokens ?? 0), 0))
const isAdmin = computed(() => account.value?.role === 'admin')
const can = (permission: string) => isAdmin.value || Boolean(account.value?.permissions.includes(permission))
const managementNav = computed(() => [...managementNavItems, ...adminExtraNav].filter((item) => can(item[3])))
const personalRequests = computed(() => usageRecords.value.length)
const personalTokens = computed(() => usageRecords.value.reduce((sum, item) => sum + item.prompt_tokens + item.completion_tokens, 0))
const personalCost = computed(() => usageRecords.value.reduce((sum, item) => sum + Number(item.cost), 0))
const setupProgress = computed(() => [accountKeys.value.some((item) => !item.revoked_at), Number(account.value?.balance ?? 0) > 0, personalRequests.value > 0].filter(Boolean).length)
const filteredCatalog = computed(() => {
  const search = catalogSearch.value.trim().toLowerCase()
  return catalog.value.filter((item) => (!search || item.model.toLowerCase().includes(search)) && (catalogGroup.value === 'all' || item.groups.some((group) => group.id === catalogGroup.value)))
})
const apiEndpoint = computed(() => import.meta.client ? `${window.location.origin}/v1/chat/completions` : '/v1/chat/completions')
const usageChart = computed(() => {
  const days = Array.from({ length: 7 }, (_, index) => {
    const date = new Date()
    date.setHours(0, 0, 0, 0)
    date.setDate(date.getDate() - 6 + index)
    return { key: date.toISOString().slice(0, 10), label: new Intl.DateTimeFormat('zh-CN', { weekday: 'short' }).format(date), tokens: 0, cost: 0 }
  })
  const byDay = new Map(days.map((day) => [day.key, day]))
  for (const item of usageRecords.value) {
    const day = byDay.get(item.created_at.slice(0, 10))
    if (day) { day.tokens += item.prompt_tokens + item.completion_tokens; day.cost += Number(item.cost) }
  }
  const maxTokens = Math.max(...days.map((day) => day.tokens), 1)
  const maxCost = Math.max(...days.map((day) => day.cost), 1)
  return days.map((day) => ({ ...day, tokenHeight: Math.max(day.tokens ? 8 : 2, Math.round(day.tokens / maxTokens * 100)), costHeight: Math.max(day.cost ? 8 : 2, Math.round(day.cost / maxCost * 100)) }))
})
const usageLinePoints = computed(() => usageChart.value.map((day, index) => `${index * 100 / 6},${100 - day.tokenHeight}`).join(' '))
const userName = (id: string | null) => users.value.find((user) => user.id === id)?.name ?? '已删除用户'
const formatDate = (value: string | null) => value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '从未'
const short = (value: string | null) => value ? `${value.slice(0, 8)}...` : '---'
const formatPrice = (value: number | null, multiplier = 1) => value == null ? '待配置' : `¥${(Number(value) * multiplier).toFixed(Number(value) * multiplier < 0.01 ? 4 : 2)}`
const modelProvider = (model: string) => {
  const name = model.toLowerCase()
  if (name.startsWith('gpt-') || name.startsWith('o1') || name.startsWith('o3')) return 'OpenAI'
  if (name.startsWith('claude')) return 'Anthropic'
  if (name.startsWith('gemini')) return 'Google'
  if (name.startsWith('deepseek')) return 'DeepSeek'
  if (name.startsWith('qwen')) return 'Qwen'
  if (name.startsWith('glm')) return 'Zhipu'
  return '通用模型'
}
const selectedCatalogGroup = (item: CatalogModel) => item.groups.find((group) => group.id === catalogGroup.value) ?? item.groups[0]
const actualMultiplier = (item: CatalogModel) => Number(item.multiplier ?? 1) * Number(selectedCatalogGroup(item)?.multiplier ?? 1)
const Empty = (props: { text: string }) => h('div', { class: 'empty' }, props.text)
Empty.props = { text: { type: String, required: true } }

async function load() {
  busy.value = true; error.value = ''
  try {
    const me = await api<Account>('/account/me')
    account.value = me
    const [ownKeys, ownUsage, ownLedger, ownGroupValue] = await Promise.all([
      api<{ data: ApiKey[] }>('/account/keys').catch(() => ({ data: [] })),
      api<{ data: UsageRecord[] }>('/account/usage').catch(() => ({ data: [] })),
      api<{ data: LedgerEntry[] }>('/account/ledger').catch(() => ({ data: [] })),
      api<{ data: string[]; groups: Group[] }>('/account/groups').catch(() => ({ data: [], groups: [] })),
    ])
    accountKeys.value = ownKeys.data; usageRecords.value = ownUsage.data; ledger.value = ownLedger.data; ownGroups.value = ownGroupValue.data
    if (!can('users.read')) groups.value = ownGroupValue.groups
    const requests: Promise<void>[] = []
    if (can('users.read')) requests.push(Promise.all([api<{ data: User[] }>('/admin/users'), api<{ data: Group[] }>('/admin/groups')]).then(([userValue, groupValue]) => { users.value = userValue.data; groups.value = groupValue.data }))
    if (can('keys.manage')) requests.push(api<{ data: ApiKey[] }>('/admin/keys').then((value) => { keys.value = value.data }))
    if (can('channels.read')) requests.push(api<{ data: Channel[] }>('/admin/channels').then((value) => { channels.value = value.data }))
    if (can('logs.read')) requests.push(api<{ data: RequestLog[] }>('/admin/request-logs').then((value) => { logs.value = value.data }))
    if (can('pricing.read')) requests.push(api<{ data: Pricing[] }>('/admin/pricing').then((value) => { pricing.value = value.data }))
    if (can('audit.read')) requests.push(api<{ data: Record<string, unknown>[] }>('/admin/audit-logs').then((value) => { auditLogs.value = value.data }))
    await Promise.all(requests)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '加载失败' } finally { busy.value = false }
}
async function loadCatalog() { const value = await api<{ data: CatalogModel[]; groups: CatalogGroup[] }>('/model-catalog'); catalog.value = value.data; catalogGroups.value = value.groups }
async function accountSignIn(register: boolean) { await action(async () => { const result = await api<{ token: string }>(register ? '/auth/register' : '/auth/login', { method: 'POST', body: JSON.stringify(register ? accountForm : { email: accountForm.email, password: accountForm.password }) }); setToken(result.token); authenticated.value = true; await load(); await router.replace({ path: '/console', query: { view: managementNav.value.length ? 'overview' : 'account' } }) }) }
async function signOut() { try { await api('/auth/logout', { method: 'POST' }) } catch { /* Local session removal is sufficient when the server is unreachable. */ } clearToken(); authenticated.value = false; error.value = ''; await router.replace('/') }
async function createKey() { await action(async () => { const response = await api<{ key: string }>('/admin/keys', { method: 'POST', body: JSON.stringify({ ...keyForm, expires_at: keyForm.expires_at ? new Date(keyForm.expires_at).toISOString() : '' }) }); createdKey.value = response.key; showKey.value = false; Object.assign(keyForm, { user_id: '', name: '', expires_at: '', group_id: '' }); await load() }) }
async function createAccountKey() { await action(async () => { const response = await api<{ key: string }>('/account/keys', { method: 'POST', body: JSON.stringify({ ...accountKeyForm, expires_at: accountKeyForm.expires_at ? new Date(accountKeyForm.expires_at).toISOString() : '' }) }); createdKey.value = response.key; showAccountKey.value = false; Object.assign(accountKeyForm, { name: '', expires_at: '', group_id: '' }); await load() }) }
async function createChannel() { await action(async () => { await api('/admin/channels', { method: 'POST', body: JSON.stringify({ ...channelForm, models: channelForm.models.split(',').map((value) => value.trim()).filter(Boolean) }) }); showChannel.value = false; Object.assign(channelForm, { name: '', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] }); await load() }) }
async function createGroup() { const name = window.prompt('分组名称'); if (!name?.trim()) return; await action(async () => { await api('/admin/groups', { method: 'POST', body: JSON.stringify({ name: name.trim() }) }); await load() }) }
async function editGroupMultiplier(group: Group) { const value = window.prompt(`${group.name} 的倍率`, String(group.multiplier)); if (value == null) return; const multiplier = Number(value); if (!Number.isFinite(multiplier) || multiplier <= 0) { error.value = '倍率必须大于 0'; return } await action(async () => { await api(`/admin/groups/${group.id}`, { method: 'PUT', body: JSON.stringify({ multiplier }) }); await load() }) }
async function toggleChannel(channel: Channel) { await action(async () => { await api(`/admin/channels/${channel.id}/status`, { method: 'POST', body: JSON.stringify({ enabled: !channel.enabled }) }); await load() }) }
async function revokeKey(key: ApiKey) { if (!confirm(`吊销 ${key.key_prefix} 的访问权限？`)) return; await action(async () => { await api(`/admin/keys/${key.id}/revoke`, { method: 'POST' }); await load() }) }
async function action(work: () => Promise<void>) { busy.value = true; error.value = ''; try { await work() } catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败' } finally { busy.value = false } }
async function copyKey() { await navigator.clipboard.writeText(createdKey.value) }
async function savePricing() { await action(async () => { await api('/admin/pricing', { method: 'POST', body: JSON.stringify(pricingForm) }); Object.assign(pricingForm, { model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 }); await load() }) }
function manageUser(user: User) { selectedUser.value = user; selectedPermissions.value = [...user.permissions]; selectedGroups.value = [...(user.groups ?? [])] }
async function saveUserAccess() { if (!selectedUser.value) return; await action(async () => { await api(`/admin/users/${selectedUser.value?.id}/role`, { method: 'POST', body: JSON.stringify({ role: selectedUser.value?.role }) }); await api(`/admin/users/${selectedUser.value?.id}/permissions`, { method: 'PUT', body: JSON.stringify({ permissions: selectedPermissions.value }) }); await api(`/admin/users/${selectedUser.value?.id}/groups`, { method: 'PUT', body: JSON.stringify({ groups: selectedGroups.value }) }); selectedUser.value = null; await load() }) }
function openAuth() { router.push('/auth') }
function openConsoleOrAuth() { router.push(authenticated.value ? '/console/overview' : '/auth') }
function closeAuth() { router.push('/') }
function openConsole(nextView: string) { if (views.includes(nextView as View)) router.push({ path: '/console', query: { view: nextView } }) }
function setTheme(nextTheme: 'light' | 'dark') {
  theme.value = nextTheme
  document.documentElement.dataset.theme = nextTheme
  localStorage.setItem('xinghai-router-theme', nextTheme)
}
function toggleTheme() { setTheme(theme.value === 'dark' ? 'light' : 'dark') }
onMounted(async () => {
  const savedTheme = localStorage.getItem('xinghai-router-theme')
  setTheme(savedTheme === 'dark' || savedTheme === 'light' ? savedTheme : window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  authenticated.value = Boolean(getToken())
  if (isMarketplacePage.value) { await loadCatalog().catch((cause) => { error.value = cause instanceof Error ? cause.message : '加载失败' }); return }
  if (!authenticated.value) return
  await load()
  if (route.meta.requiresAuth && error.value) {
    clearToken()
    authenticated.value = false
    await router.replace('/auth')
  }
})
</script>

<template>
  <main v-if="isLanding" class="landing-shell">
    <nav class="landing-nav">
      <a class="landing-logo" href="/"><span class="brand-mark small"><Bot :size="19" /></span><span>Xinghai</span><i>Router</i></a>
      <div class="landing-links"><a href="#features">能力</a><a href="#quickstart">快速开始</a><a href="/models">模型广场</a><button class="theme-toggle" :aria-label="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" :title="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" @click="toggleTheme"><Sun v-if="theme === 'dark'" :size="16" /><Moon v-else :size="16" /></button><button class="button ghost" @click="openConsoleOrAuth">进入控制台 <ChevronRight :size="15" /></button></div>
    </nav>
    <section class="hero-section">
      <div class="hero-copy"><p class="eyebrow">OPENAI-COMPATIBLE MODEL GATEWAY</p><h1>让每一次模型请求，<em>走向正确的地方。</em></h1><p class="hero-description">Xinghai Router 是一个轻量、可观测的模型流量网关。统一 API、智能路由上游渠道，并把用量与成本留在你的控制台。</p><div class="hero-actions"><button class="button primary hero-button" @click="openConsoleOrAuth">打开控制台 <ChevronRight :size="16" /></button><a class="text-link" href="#quickstart">查看请求示例 <span>↓</span></a></div></div>
      <div class="hero-visual"><div class="visual-glow"></div><div class="route-card"><div class="route-card-top"><span><i class="live-dot"></i>ROUTER ONLINE</span><code>POST /v1/chat/completions</code></div><div class="route-model"><Bot :size="18" /><strong>gpt-4o</strong><span>智能路由中</span></div><div class="route-line"><span class="route-node active"></span><div><b>OpenAI 主线路</b><small>优先级 P10 · 42ms</small></div><span class="route-check">✓</span></div><div class="route-line muted-route"><span class="route-node"></span><div><b>备用渠道</b><small>等待流量切换</small></div></div><div class="route-footer"><span>成功率</span><strong>99.98%</strong><span class="route-divider"></span><span>平均延迟</span><strong>186ms</strong></div></div></div>
    </section>
    <section id="features" class="feature-section"><div class="section-intro"><p class="eyebrow">BUILT FOR CONTROL</p><h2>把复杂的上游，<br><em>变成一个简单的入口。</em></h2></div><div class="feature-grid"><article><span class="feature-number">01</span><RadioTower :size="21" /><h3>智能路由</h3><p>按模型、优先级和渠道状态分发请求，在上游波动时自动切换。</p></article><article><span class="feature-number">02</span><Activity :size="21" /><h3>全链路可观测</h3><p>请求日志、状态码、耗时和 Token 用量，都在一个清晰的控制台里。</p></article><article><span class="feature-number">03</span><WalletCards :size="21" /><h3>用量与成本</h3><p>为模型设置价格规则，记录每次调用费用，让团队用得明白。</p></article></div></section>
    <section id="quickstart" class="quickstart-section"><div><p class="eyebrow">ONE ENDPOINT</p><h2>接入只需要<br><em>改一个地址。</em></h2><p>使用熟悉的 OpenAI SDK，将 Base URL 指向 Xinghai Router，即刻获得统一的模型入口。</p></div><pre><span class="code-comment">// 使用 OpenAI SDK</span><span><b>const</b> client = <b>new</b> OpenAI({</span><span>  apiKey: <i>'sk-xh-your-key'</i>,</span><span>  baseURL: <i>'http://localhost:8080/v1'</i></span><span>})</span><span class="code-gap"></span><span><b>await</b> client.chat.completions.create({</span><span>  model: <i>'gpt-4o'</i>,</span><span>  messages: [{ role: <i>'user'</i>, content: <i>'你好'</i> }]</span><span>})</span></pre></section>
    <footer class="landing-footer"><span>© 2026 Xinghai Router</span><span>轻量、透明、为模型流量而生。</span></footer>
  </main>

  <main v-else-if="isMarketplacePage" class="public-marketplace">
    <nav class="marketplace-nav"><a class="landing-logo" href="/"><span class="brand-mark small"><Bot :size="19" /></span><span>Xinghai</span><i>Router</i></a><div><button class="theme-toggle" :aria-label="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" @click="toggleTheme"><Sun v-if="theme === 'dark'" :size="16" /><Moon v-else :size="16" /></button><button class="button ghost" @click="openConsoleOrAuth">{{ authenticated ? '进入控制台' : '登录' }} <ChevronRight :size="15" /></button></div></nav>
    <section class="marketplace-page-content">
      <section class="marketplace-hero"><div><span class="marketplace-kicker"><Sparkles :size="13" /> MODEL CATALOG</span><h1>找到适合你的模型</h1><p>汇集当前已配置渠道的全部可用模型，价格按每百万 Token 展示。</p></div><div class="marketplace-count"><strong>{{ catalog.length }}</strong><span>个模型可用</span></div></section>
      <p v-if="error" class="error banner"><CircleAlert :size="16" />{{ error }}</p>
      <section class="marketplace-tools"><div class="marketplace-search"><Search :size="16" /><input v-model="catalogSearch" aria-label="搜索模型" placeholder="搜索模型名称" /></div><div class="group-filters"><button :class="{ active: catalogGroup === 'all' }" @click="catalogGroup = 'all'">全部分组</button><button v-for="group in catalogGroups" :key="group.id" :class="{ active: catalogGroup === group.id }" @click="catalogGroup = group.id">{{ group.name }} <small>{{ Number(group.multiplier).toFixed(2) }}x</small></button></div></section>
      <div class="model-market-grid"><article v-for="item in filteredCatalog" :key="item.model" class="model-market-card"><div class="model-card-heading"><span class="model-avatar">{{ item.model.slice(0, 1).toUpperCase() }}</span><div><h3>{{ item.model }}</h3><p>{{ modelProvider(item.model) }}</p></div><span :class="['pricing-state', { missing: item.input_per_million == null }]">{{ item.input_per_million == null ? '待定价' : '可用' }}</span></div><div class="model-price-grid"><div><span>输入</span><strong>{{ formatPrice(item.input_per_million, actualMultiplier(item)) }}</strong><small>/ 1M tokens</small></div><div><span>缓存输入</span><strong>{{ formatPrice(item.cached_input_per_million, actualMultiplier(item)) }}</strong><small>/ 1M tokens</small></div><div><span>输出</span><strong>{{ formatPrice(item.output_per_million, actualMultiplier(item)) }}</strong><small>/ 1M tokens</small></div></div><footer><div class="model-groups"><span v-for="group in item.groups" :key="group.id" :class="{ selected: catalogGroup === group.id }">{{ group.name }}</span></div><span class="actual-rate">实际倍率 <b>{{ actualMultiplier(item).toFixed(2) }}x</b></span></footer></article><Empty v-if="!filteredCatalog.length" :text="catalog.length ? '没有符合筛选条件的模型' : error ? '模型目录暂时不可用' : '启用渠道并配置模型后将在这里展示'" /></div>
      <p class="pricing-note">展示价 = 模型基础价格 × 模型倍率 × 当前筛选分组倍率。选择“全部分组”时，每张卡片采用其首个可用分组。</p>
    </section>
  </main>

  <main v-else-if="isAuthPage" class="login-shell">
    <section class="login-card"><div class="login-card-actions"><button class="theme-toggle" :aria-label="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" @click="toggleTheme"><Sun v-if="theme === 'dark'" :size="16" /><Moon v-else :size="16" /></button><button class="login-close" aria-label="返回首页" @click="closeAuth">×</button></div><div class="brand-mark"><Bot :size="29" /></div><p class="eyebrow">XINGHAI ROUTER</p><h1>控制模型流量。</h1><div class="auth-tabs"><button :class="{ active: loginMode === 'login' }" @click="loginMode = 'login'">账户登录</button><button :class="{ active: loginMode === 'register' }" @click="loginMode = 'register'">创建账户</button></div><form @submit.prevent="accountSignIn(loginMode === 'register')"><label v-if="loginMode === 'register'">姓名<input v-model="accountForm.name" autocomplete="name" required maxlength="100" placeholder="例如：李雷" /></label><label>邮箱<input v-model="accountForm.email" type="email" autocomplete="email" required placeholder="name@example.com" /></label><label>密码<input v-model="accountForm.password" type="password" :autocomplete="loginMode === 'register' ? 'new-password' : 'current-password'" required minlength="8" placeholder="至少 8 个字符" /></label><button class="button primary full" :disabled="busy">{{ loginMode === 'register' ? '创建并进入控制台' : '登录控制台' }} <ChevronRight :size="16" /></button></form><p v-if="error" class="error"><CircleAlert :size="16" />{{ error }}</p></section>
  </main>

  <main v-else class="app-shell">
    <aside class="sidebar">
       <div class="logo"><span class="brand-mark small"><Bot :size="19" /></span><span>Xinghai</span><i>Router</i></div>
        <nav>
          <div class="nav-group"><p class="nav-label">常规</p><button v-for="[id, label, Icon] in generalNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div class="nav-group"><p class="nav-label">账户</p><button v-for="[id, label, Icon] in billingNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div class="nav-group"><p class="nav-label">个人</p><button v-for="[id, label, Icon] in personalNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div v-if="managementNav.length" class="nav-group management-group"><p class="nav-label">管理</p><button v-for="[id, label, Icon] in managementNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
        </nav>
      <div class="sidebar-footer"><div class="gateway-status"><span class="live-dot"></span><span><b>网关在线</b><small>服务运行正常</small></span></div><div class="sidebar-account"><i>{{ account?.name?.slice(0, 1) || '?' }}</i><span><b>{{ account?.name || '正在加载' }}</b><small>{{ account?.role || '账户' }}</small></span><button aria-label="退出登录" title="退出登录" @click="signOut"><LogOut :size="16" /></button></div></div>
    </aside>
     <section class="content">
       <section v-if="view === 'users' && can('system.manage')" class="panel group-management" style="margin-bottom: 1rem"><div class="panel-title"><div><h2>访问分组</h2><p>渠道设置分组后，仅属于相同分组的用户可访问；倍率参与实际结算。</p></div><button class="button ghost" @click="createGroup"><Plus :size="16" />新建分组</button></div><div class="group-rate-list"><button v-for="group in groups" :key="group.id" @click="editGroupMultiplier(group)"><span>{{ group.name }}</span><b>{{ Number(group.multiplier).toFixed(2) }}x</b><small>点击修改</small></button></div></section>
         <header class="console-header"><div><p class="eyebrow">{{ managementNav.some((item) => item[0] === view) ? '管理' : personalNav.some((item) => item[0] === view) ? '个人' : billingNav.some((item) => item[0] === view) ? '账户' : '常规' }}</p><h1>{{ [...managementNavItems, ...generalNav, ...billingNav, ...personalNav, ...adminExtraNav].find((item) => item[0] === view)?.[1] }}</h1></div><div class="header-actions"><a class="button ghost marketplace-link" href="/models"><Sparkles :size="15" />模型广场</a><span class="account-chip"><i>{{ account?.name?.slice(0, 1) || '?' }}</i>{{ account?.name || '正在加载' }}</span><button class="theme-toggle" :aria-label="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" :title="theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'" @click="toggleTheme"><Sun v-if="theme === 'dark'" :size="16" /><Moon v-else :size="16" /></button><button class="button ghost" @click="load" :disabled="busy"><RefreshCw :size="16" :class="{ spinning: busy }" />刷新</button></div></header>
      <p v-if="error" class="error banner"><CircleAlert :size="16" />{{ error }}</p>

      <template v-if="view === 'overview'">
        <section class="setup-workspace">
          <div class="setup-guide">
            <div class="setup-heading"><div><span class="overview-kicker">快速开始</span><h2>{{ account?.name || '我的账户' }}，开始接入模型网关</h2><p>完成下面三步，即可发送第一条模型请求。</p></div><span class="setup-progress">{{ setupProgress }} / 3</span></div>
            <div class="setup-steps">
              <button :class="{ complete: accountKeys.some((item) => !item.revoked_at) }" @click="openConsole('account')"><i><Check v-if="accountKeys.some((item) => !item.revoked_at)" :size="14" /><span v-else>1</span></i><span><b>创建 API 密钥</b><small>为应用签发访问凭证</small></span><ChevronRight :size="16" /></button>
              <button :class="{ complete: Number(account?.balance ?? 0) > 0 }" @click="openConsole('wallet')"><i><Check v-if="Number(account?.balance ?? 0) > 0" :size="14" /><span v-else>2</span></i><span><b>确认账户余额</b><small>为模型调用准备可用额度</small></span><ChevronRight :size="16" /></button>
              <button :class="{ complete: personalRequests > 0 }" @click="openConsole('usage')"><i><Check v-if="personalRequests > 0" :size="14" /><span v-else>3</span></i><span><b>发送模型请求</b><small>在用量明细中确认调用结果</small></span><ChevronRight :size="16" /></button>
            </div>
          </div>
          <div class="request-preview"><div class="request-preview-title"><span><TerminalSquare :size="16" />第一条 API 请求</span><code>curl</code></div><pre><span>curl {{ apiEndpoint }} \</span><span>  -H <i>"Authorization: Bearer sk-xh-..."</i> \</span><span>  -H <i>"Content-Type: application/json"</i> \</span><span>  -d <i>'{"model":"gpt-4o-mini",</i></span><span><i>       "messages":[{"role":"user","content":"你好"}]}'</i></span></pre><div class="request-signals"><span><i></i>网关在线</span><button @click="openConsole('account')">查看密钥 <ChevronRight :size="14" /></button></div></div>
        </section>
        <div class="metrics"><article><span>可用余额</span><strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong><p><WalletCards :size="15" />不含预扣金额</p></article><article><span>有效 API 密钥</span><strong>{{ accountKeys.filter((item) => !item.revoked_at).length }}</strong><p><KeyRound :size="15" />当前账户密钥</p></article><article><span>近期调用</span><strong>{{ personalRequests }}</strong><p><Activity :size="15" />最近 100 条用量</p></article><article><span>计量 Token</span><strong>{{ personalTokens.toLocaleString() }}</strong><p><ReceiptText :size="15" />累计费用 {{ personalCost.toFixed(6) }}</p></article></div>
        <div class="grid-two"><section class="panel usage-line-chart"><div class="panel-title"><div><h2>用量趋势</h2><p>近 7 日 Token 消耗</p></div><button class="text-button" @click="openConsole('usage')">查看全部</button></div><div class="line-plot"><svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-label="近 7 日 Token 趋势"><defs><linearGradient id="usageFill" x1="0" x2="0" y1="0" y2="1"><stop offset="0%" stop-color="#65a986" stop-opacity=".34" /><stop offset="100%" stop-color="#65a986" stop-opacity="0" /></linearGradient></defs><path :d="`M 0,100 L ${usageLinePoints} L 100,100 Z`" fill="url(#usageFill)" /><polyline :points="usageLinePoints" fill="none" stroke="#2d7657" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" vector-effect="non-scaling-stroke" /></svg><div class="line-labels"><span v-for="day in usageChart" :key="day.key">{{ day.label }}<b>{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</b></span></div></div></section><section class="panel"><div class="panel-title"><div><h2>访问密钥</h2><p>当前账户可用的 API 密钥</p></div><button class="text-button" @click="openConsole('account')">我的账户</button></div><div v-if="accountKeys.length" class="compact-list"><div v-for="key in accountKeys.slice(0, 5)" :key="key.id"><code>{{ key.key_prefix }}...</code><span>{{ key.name }}</span><b :class="key.revoked_at ? 'danger' : 'success'">{{ key.revoked_at ? '已吊销' : '有效' }}</b></div></div><Empty v-else text="尚未创建 API 密钥" /></section></div>
      </template>

       <template v-if="view === 'users'"><section class="toolbar"><div><h2>用户与权限</h2><p>提升账户为管理员，或为运营用户授予具体权限。</p></div></section><section class="panel table-panel"><table><thead><tr><th>用户</th><th>角色</th><th>权限</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="user in users" :key="user.id"><td><b>{{ user.name }}</b><small>{{ user.email }}</small></td><td><span class="pill">{{ user.role }}</span></td><td>{{ user.role === 'admin' ? '全部权限' : user.permissions.join(', ') || '无' }}</td><td><span :class="['state', user.enabled ? 'good' : 'bad']">{{ user.enabled ? '已启用' : '已停用' }}</span></td><td><button v-if="can('system.manage')" class="text-button" @click="manageUser(user)">管理权限</button></td></tr></tbody></table><Empty v-if="!users.length" text="还没有用户" /></section></template>
      <template v-if="view === 'keys'"><section class="toolbar"><div><h2>API 密钥</h2><p>仅在创建时显示一次完整密钥。</p></div><button class="button primary" :disabled="!users.length" @click="showKey = true"><Plus :size="16" />创建密钥</button></section><section class="panel table-panel"><table><thead><tr><th>名称</th><th>所属用户</th><th>前缀</th><th>最后使用</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="key in keys" :key="key.id"><td><b>{{ key.name }}</b></td><td>{{ userName(key.user_id) }}</td><td><code>{{ key.key_prefix }}...</code></td><td>{{ formatDate(key.last_used_at) }}</td><td><span :class="['state', key.revoked_at ? 'bad' : 'good']">{{ key.revoked_at ? '已吊销' : '有效' }}</span></td><td><button v-if="!key.revoked_at" class="text-button danger" @click="revokeKey(key)">吊销</button></td></tr></tbody></table><Empty v-if="!keys.length" text="创建用户后，即可签发 API 密钥" /></section></template>
      <template v-if="view === 'channels'"><section class="toolbar"><div><h2>上游渠道</h2><p>模型请求按渠道优先级进行选择。</p></div><button v-if="can('channels.manage')" class="button primary" @click="showChannel = true"><Plus :size="16" />添加渠道</button></section><div class="channel-cards"><article v-for="channel in channels" :key="channel.id" class="panel channel-card"><div class="card-top"><span :class="['status-dot', { off: !channel.enabled }]"></span><span>优先级 {{ channel.priority }}</span><button v-if="can('channels.manage')" class="toggle" :class="{ on: channel.enabled }" :aria-label="channel.enabled ? '停用渠道' : '启用渠道'" @click="toggleChannel(channel)"><i></i></button></div><h3>{{ channel.name }}</h3><p class="url">{{ channel.base_url }}</p><div class="model-tags"><span v-for="model in channel.models" :key="model">{{ model }}</span></div></article><Empty v-if="!channels.length" text="添加 OpenAI-compatible 上游开始路由" /></div></template>
       <template v-if="view === 'logs'"><section class="toolbar"><div><h2>请求日志</h2><p>最多显示最新 100 条记录。</p></div></section><section class="panel table-panel"><table><thead><tr><th>时间</th><th>模型</th><th>状态</th><th>耗时</th><th>Token</th><th>请求 ID</th></tr></thead><tbody><tr v-for="log in logs" :key="log.request_id"><td>{{ formatDate(log.created_at) }}</td><td><code>{{ log.model }}</code></td><td><span :class="['state', log.status_code < 400 ? 'good' : 'bad']">{{ log.status_code }}</span></td><td>{{ log.duration_ms }} ms</td><td>{{ log.total_tokens ?? 0 }}</td><td><code>{{ short(log.request_id) }}</code></td></tr></tbody></table><Empty v-if="!logs.length" text="暂无请求日志" /></section></template>
           <template v-if="view === 'account'"><section class="toolbar"><div><h2>API 密钥</h2><p>用于访问 OpenAI-compatible 模型接口。</p></div><button class="button primary" @click="showAccountKey = true"><Plus :size="16" />创建密钥</button></section><section class="panel table-panel"><table><thead><tr><th>名称</th><th>密钥前缀</th><th>创建时间</th><th>最后使用</th><th>状态</th></tr></thead><tbody><tr v-for="key in accountKeys" :key="key.id"><td><b>{{ key.name }}</b></td><td><code>{{ key.key_prefix }}...</code></td><td>{{ formatDate(key.created_at) }}</td><td>{{ formatDate(key.last_used_at) }}</td><td><span :class="['state', key.revoked_at ? 'bad' : 'good']">{{ key.revoked_at ? '已吊销' : '有效' }}</span></td></tr></tbody></table><Empty v-if="!accountKeys.length" text="尚未创建 API 密钥" /></section></template>
           <template v-if="view === 'profile'"><section class="profile-layout"><section class="panel account-card"><div class="profile-avatar">{{ account?.name?.slice(0, 1) || '?' }}</div><div><span class="overview-kicker">账户资料</span><h2>{{ account?.name }}</h2><p>{{ account?.email }}</p></div></section><section class="panel profile-details"><div><span>账户角色</span><b>{{ account?.role }}</b></div><div><span>账户 ID</span><code>{{ account?.id }}</code></div><div><span>权限范围</span><b>{{ account?.role === 'admin' ? '全部权限' : account?.permissions.join(', ') || '普通账户' }}</b></div></section></section></template>
        <template v-if="view === 'wallet'"><section class="wallet-hero"><div><span>可用余额</span><strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong><p>余额可用于后续模型调用费用结算。</p></div><WalletCards :size="64" /></section><div class="metrics wallet-metrics"><article><span>当前余额</span><strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong><p><WalletCards :size="15" />账户可用额度</p></article><article><span>预扣金额</span><strong>{{ Number(account?.reserved ?? 0).toFixed(4) }}</strong><p>并发请求中的预留费用</p></article><article><span>累计消费</span><strong>{{ personalCost.toFixed(6) }}</strong><p><ReceiptText :size="15" />最近 100 条用量</p></article></div><section class="panel table-panel"><div class="panel-title"><div><h2>余额流水</h2><p>充值、扣费及退款记录</p></div><button class="text-button" @click="openConsole('ledger')">查看全部</button></div><table><thead><tr><th>时间</th><th>类型</th><th>变动</th><th>余额</th><th>说明</th></tr></thead><tbody><tr v-for="item in ledger.slice(0, 10)" :key="item.id"><td>{{ formatDate(item.created_at) }}</td><td>{{ item.kind }}</td><td :class="item.amount < 0 ? 'danger' : 'success'">{{ item.amount }}</td><td>{{ item.balance_after }}</td><td>{{ item.note || item.request_id }}</td></tr></tbody></table><Empty v-if="!ledger.length" text="暂无余额流水" /></section></template>
        <template v-if="view === 'usage'"><section class="usage-summary"><article><span>近 7 日 Token</span><strong>{{ personalTokens.toLocaleString() }}</strong><small>输入与输出 Token 总和</small></article><article><span>近 7 日费用</span><strong>{{ personalCost.toFixed(6) }}</strong><small>按当前价格规则结算</small></article><article><span>调用次数</span><strong>{{ personalRequests }}</strong><small>最近 100 条用量记录</small></article></section><section class="panel usage-chart"><div class="panel-title"><div><h2>用量趋势</h2><p>近 7 天 Token 消耗与费用变化</p></div><div class="chart-legend"><span><i class="token-dot"></i>Token</span><span><i class="cost-dot"></i>费用</span></div></div><div class="chart-bars"><div v-for="day in usageChart" :key="day.key" class="chart-day"><div class="chart-values"><span :style="{ height: `${day.tokenHeight}%` }" :title="`${day.tokens.toLocaleString()} tokens`"></span><i :style="{ height: `${day.costHeight}%` }" :title="`费用 ${day.cost.toFixed(6)}`"></i></div><b>{{ day.label }}</b><small>{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</small></div></div></section><section class="panel table-panel"><div class="panel-title"><div><h2>用量明细</h2><p>最近 100 条模型调用记录</p></div></div><table><thead><tr><th>时间</th><th>模型</th><th>输入</th><th>输出</th><th>费用</th><th>状态</th></tr></thead><tbody><tr v-for="item in usageRecords" :key="item.request_id"><td>{{ formatDate(item.created_at) }}</td><td><code>{{ item.model }}</code></td><td>{{ item.prompt_tokens }}</td><td>{{ item.completion_tokens }}</td><td>{{ Number(item.cost).toFixed(6) }}</td><td>{{ item.status }}</td></tr></tbody></table><Empty v-if="!usageRecords.length" text="暂无用量记录" /></section></template>
        <template v-if="view === 'ledger'"><section class="toolbar"><div><h2>余额流水</h2><p>查看账户余额的每一笔变动记录。</p></div></section><section class="panel table-panel"><table><thead><tr><th>时间</th><th>类型</th><th>变动</th><th>余额</th><th>说明</th></tr></thead><tbody><tr v-for="item in ledger" :key="item.id"><td>{{ formatDate(item.created_at) }}</td><td><span class="pill">{{ item.kind }}</span></td><td :class="item.amount < 0 ? 'danger' : 'success'">{{ item.amount }}</td><td>{{ item.balance_after }}</td><td>{{ item.note || item.request_id }}</td></tr></tbody></table><Empty v-if="!ledger.length" text="暂无余额流水" /></section></template>
        <template v-if="view === 'pricing'"><section class="toolbar"><div><h2>模型定价</h2><p>按百万 token 配置输入、缓存输入和输出价格。</p></div></section><form v-if="can('pricing.manage')" class="panel pricing-form" @submit.prevent="savePricing"><label>模型<input v-model="pricingForm.model" required placeholder="例如 gpt-4o" /></label><label>输入价格<input v-model.number="pricingForm.input_per_million" type="number" min="0" step="any" placeholder="0" /></label><label>缓存输入<input v-model.number="pricingForm.cached_input_per_million" type="number" min="0" step="any" placeholder="0" /></label><label>输出价格<input v-model.number="pricingForm.output_per_million" type="number" min="0" step="any" placeholder="0" /></label><label>倍率<input v-model.number="pricingForm.multiplier" type="number" min="0.01" step="any" placeholder="1" /></label><button class="button primary">保存规则</button></form><section class="panel table-panel"><table><thead><tr><th>模型</th><th>输入</th><th>缓存输入</th><th>输出</th><th>倍率</th></tr></thead><tbody><tr v-for="item in pricing" :key="item.id"><td><code>{{ item.model }}</code></td><td>{{ item.input_per_million }}</td><td>{{ item.cached_input_per_million }}</td><td>{{ item.output_per_million }}</td><td>{{ item.multiplier }}</td></tr></tbody></table><Empty v-if="!pricing.length" text="暂无模型定价规则" /></section></template>
        <template v-if="view === 'audit'"><section class="toolbar"><div><h2>操作审计</h2><p>追踪管理操作及其影响对象。</p></div></section><section class="panel table-panel"><table><thead><tr><th>时间</th><th>动作</th><th>对象</th><th>详情</th></tr></thead><tbody><tr v-for="item in auditLogs" :key="String(item.id)"><td>{{ formatDate(String(item.created_at)) }}</td><td><span class="pill">{{ item.action }}</span></td><td>{{ item.entity_type }} / {{ item.entity_id }}</td><td><code>{{ JSON.stringify(item.details) }}</code></td></tr></tbody></table><Empty v-if="!auditLogs.length" text="暂无操作审计" /></section></template>
    </section>

    <div v-if="selectedUser || showKey || showAccountKey || showChannel || createdKey" class="modal-backdrop" @click.self="selectedUser = null; showKey = showAccountKey = showChannel = false">
      <form v-if="selectedUser" class="modal" @submit.prevent="saveUserAccess"><div class="modal-title"><h2>管理用户权限</h2><button type="button" @click="selectedUser = null">×</button></div><p class="muted">{{ selectedUser.name }} · {{ selectedUser.email }}</p><label>角色<select v-model="selectedUser.role"><option value="user">用户</option><option value="operator">运营</option><option value="admin">管理员（全部权限）</option></select></label><label>用户分组<select v-model="selectedGroups" multiple size="5"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option></select></label><label v-if="selectedUser.role !== 'admin'">细粒度权限<select v-model="selectedPermissions" multiple size="8"><option v-for="permission in permissions" :key="permission" :value="permission">{{ permission }}</option></select></label><button class="button primary full" :disabled="busy">保存访问权限</button></form>
      <form v-if="showKey" class="modal" @submit.prevent="createKey"><div class="modal-title"><h2>创建 API 密钥</h2><button type="button" @click="showKey = false">×</button></div><label>用户<select v-model="keyForm.user_id" required><option disabled value="">选择用户</option><option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option></select></label><label>使用分组<select v-model="keyForm.group_id"><option value="">自动匹配（1.00x）</option><option v-for="group in groups.filter((item) => users.find((user) => user.id === keyForm.user_id)?.groups.includes(item.id))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option></select></label><label>密钥名称<input v-model="keyForm.name" required placeholder="例如：生产环境" /></label><label>过期时间 <small>可选</small><input v-model="keyForm.expires_at" type="datetime-local" /></label><button class="button primary full" :disabled="busy">签发密钥</button></form>
      <form v-if="showAccountKey" class="modal" @submit.prevent="createAccountKey"><div class="modal-title"><h2>创建 API 密钥</h2><button type="button" @click="showAccountKey = false">×</button></div><p class="muted">密钥将归属当前账户，仅在创建后显示一次。</p><label>使用分组<select v-model="accountKeyForm.group_id"><option value="">自动匹配（1.00x）</option><option v-for="group in groups.filter((item) => ownGroups.includes(item.name))" :key="group.id" :value="group.id">{{ group.name }} · {{ Number(group.multiplier).toFixed(2) }}x</option></select></label><label>密钥名称<input v-model="accountKeyForm.name" required maxlength="100" placeholder="例如：本地开发" /></label><label>过期时间 <small>可选</small><input v-model="accountKeyForm.expires_at" type="datetime-local" /></label><button class="button primary full" :disabled="busy">创建密钥</button></form>
      <form v-if="showChannel" class="modal" @submit.prevent="createChannel"><div class="modal-title"><h2>添加上游渠道</h2><button type="button" @click="showChannel = false">×</button></div><label>名称<input v-model="channelForm.name" required placeholder="例如：OpenAI 主线路" /></label><label>Base URL<input v-model="channelForm.base_url" required type="url" /></label><label>上游 API Key<input v-model="channelForm.api_key" required type="password" /></label><label>模型 <small>逗号分隔</small><input v-model="channelForm.models" required placeholder="gpt-4o-mini, gpt-4o" /></label><label>访问分组<select v-model="channelForm.groups" multiple size="5"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select><small>不选择表示所有用户可访问</small></label><label>优先级 <input v-model.number="channelForm.priority" type="number" min="0" /></label><button class="button primary full" :disabled="busy">保存渠道</button></form>
      <section v-if="createdKey" class="modal secret"><div class="modal-title"><h2>保存 API 密钥</h2><button @click="createdKey = ''">×</button></div><p>完整密钥只显示这一次，请立即保存。</p><code>{{ createdKey }}</code><button class="button primary full" @click="copyKey"><Copy :size="16" />复制密钥</button><button class="button ghost full" @click="createdKey = ''">我已保存</button></section>
    </div>
  </main>
</template>
