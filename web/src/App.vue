<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { Activity, Bot, Check, ChevronRight, CircleAlert, Copy, KeyRound, LayoutDashboard, LogOut, Plus, RadioTower, RefreshCw, TerminalSquare, Users, WalletCards, ReceiptText, Tags } from 'lucide-vue-next'
import { api, clearToken, getToken, setToken } from './api'
import type { Account, ApiKey, Channel, LedgerEntry, Pricing, RequestLog, UsageRecord, User } from './api'

type View = 'overview' | 'users' | 'keys' | 'channels' | 'logs' | 'account' | 'usage' | 'ledger' | 'pricing' | 'audit'
const view = ref<View>('overview')
const authenticated = ref(Boolean(getToken()))
const mode = ref<'admin' | 'user'>('user')
const error = ref('')
const busy = ref(false)
const users = ref<User[]>([])
const keys = ref<ApiKey[]>([])
const channels = ref<Channel[]>([])
const logs = ref<RequestLog[]>([])
const account = ref<Account | null>(null)
const usageRecords = ref<UsageRecord[]>([])
const ledger = ref<LedgerEntry[]>([])
const pricing = ref<Pricing[]>([])
const auditLogs = ref<Record<string, unknown>[]>([])
const createdKey = ref('')
const showKey = ref(false)
const showChannel = ref(false)
const selectedUser = ref<User | null>(null)
const selectedPermissions = ref<string[]>([])
const keyForm = reactive({ user_id: '', name: '', expires_at: '' })
const channelForm = reactive({ name: '', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100 })

const nav = [
  ['overview', '概览', LayoutDashboard, 'logs.read'], ['users', '用户', Users, 'users.read'], ['keys', 'API 密钥', KeyRound, 'keys.manage'], ['channels', '渠道', RadioTower, 'channels.read'], ['logs', '请求日志', TerminalSquare, 'logs.read'],
] as const
const userNav = [['account', '我的账户', WalletCards], ['usage', '用量明细', Activity], ['ledger', '余额流水', ReceiptText]] as const
const sessionNav = [['account', '我的账户', WalletCards]] as const
const adminExtraNav = [['pricing', '模型定价', Tags, 'pricing.read'], ['audit', '操作审计', ReceiptText, 'audit.read']] as const
const permissions = ['users.read', 'users.manage', 'keys.manage', 'channels.read', 'channels.manage', 'logs.read', 'pricing.read', 'pricing.manage', 'audit.read', 'wallets.manage', 'routes.manage', 'quotas.manage', 'system.manage']
const pricingForm = reactive({ model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 })
const authPage = ref(window.location.pathname === '/auth')
const loginMode = ref<'token' | 'login' | 'register'>('token')
const accountForm = reactive({ name: '', email: '', password: '' })
const activeChannels = computed(() => channels.value.filter((channel) => channel.enabled).length)
const successRate = computed(() => logs.value.length ? Math.round(logs.value.filter((log) => log.status_code < 400).length / logs.value.length * 100) : 100)
const totalTokens = computed(() => logs.value.reduce((sum, log) => sum + (log.total_tokens ?? 0), 0))
const isAdmin = computed(() => account.value?.role === 'admin')
const can = (permission: string) => isAdmin.value || Boolean(account.value?.permissions.includes(permission))
const adminNav = computed(() => [...nav, ...adminExtraNav].filter((item) => can(item[3])))
const userName = (id: string | null) => users.value.find((user) => user.id === id)?.name ?? '已删除用户'
const formatDate = (value: string | null) => value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '从未'
const short = (value: string | null) => value ? `${value.slice(0, 8)}...` : '---'

async function load() {
  busy.value = true; error.value = ''
  try {
    account.value = await api<Account>('/account/me')
    mode.value = adminNav.value.length ? 'admin' : 'user'
    if (mode.value === 'user') return
    const requests: Promise<void>[] = []
    if (can('users.read')) requests.push(api<{ data: User[] }>('/admin/users').then((value) => { users.value = value.data }))
    if (can('keys.manage')) requests.push(api<{ data: ApiKey[] }>('/admin/keys').then((value) => { keys.value = value.data }))
    if (can('channels.read')) requests.push(api<{ data: Channel[] }>('/admin/channels').then((value) => { channels.value = value.data }))
    if (can('logs.read')) requests.push(api<{ data: RequestLog[] }>('/admin/request-logs').then((value) => { logs.value = value.data }))
    if (can('pricing.read')) requests.push(api<{ data: Pricing[] }>('/admin/pricing').then((value) => { pricing.value = value.data }))
    if (can('audit.read')) requests.push(api<{ data: Record<string, unknown>[] }>('/admin/audit-logs').then((value) => { auditLogs.value = value.data }))
    await Promise.all(requests)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '加载失败' } finally { busy.value = false }
}
async function accountSignIn(register: boolean) { await action(async () => { const result = await api<{ token: string }>(register ? '/auth/register' : '/auth/login', { method: 'POST', body: JSON.stringify(register ? accountForm : { email: accountForm.email, password: accountForm.password }) }); setToken(result.token); authenticated.value = true; view.value = 'account'; await load(); if (mode.value === 'admin') view.value = 'overview' }) }
async function signOut() { try { await api('/auth/logout', { method: 'POST' }) } catch { /* Local session removal is sufficient when the server is unreachable. */ } clearToken(); authenticated.value = false; error.value = '' }
async function createKey() { await action(async () => { const response = await api<{ key: string }>('/admin/keys', { method: 'POST', body: JSON.stringify({ ...keyForm, expires_at: keyForm.expires_at ? new Date(keyForm.expires_at).toISOString() : '' }) }); createdKey.value = response.key; showKey.value = false; Object.assign(keyForm, { user_id: '', name: '', expires_at: '' }); await load() }) }
async function createChannel() { await action(async () => { await api('/admin/channels', { method: 'POST', body: JSON.stringify({ ...channelForm, models: channelForm.models.split(',').map((value) => value.trim()).filter(Boolean) }) }); showChannel.value = false; Object.assign(channelForm, { name: '', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100 }); await load() }) }
async function toggleChannel(channel: Channel) { await action(async () => { await api(`/admin/channels/${channel.id}/status`, { method: 'POST', body: JSON.stringify({ enabled: !channel.enabled }) }); await load() }) }
async function revokeKey(key: ApiKey) { if (!confirm(`吊销 ${key.key_prefix} 的访问权限？`)) return; await action(async () => { await api(`/admin/keys/${key.id}/revoke`, { method: 'POST' }); await load() }) }
async function action(work: () => Promise<void>) { busy.value = true; error.value = ''; try { await work() } catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败' } finally { busy.value = false } }
async function copyKey() { await navigator.clipboard.writeText(createdKey.value) }
async function savePricing() { await action(async () => { await api('/admin/pricing', { method: 'POST', body: JSON.stringify(pricingForm) }); Object.assign(pricingForm, { model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 }); await load() }) }
function manageUser(user: User) { selectedUser.value = user; selectedPermissions.value = [...user.permissions] }
async function saveUserAccess() { if (!selectedUser.value) return; await action(async () => { await api(`/admin/users/${selectedUser.value?.id}/role`, { method: 'POST', body: JSON.stringify({ role: selectedUser.value?.role }) }); await api(`/admin/users/${selectedUser.value?.id}/permissions`, { method: 'PUT', body: JSON.stringify({ permissions: selectedPermissions.value }) }); selectedUser.value = null; await load() }) }
function openAuth() { window.history.pushState({}, '', '/auth'); authPage.value = true }
function closeAuth() { window.history.pushState({}, '', '/'); authPage.value = false; error.value = '' }
function syncPage() { authPage.value = window.location.pathname === '/auth' }
onMounted(() => { window.addEventListener('popstate', syncPage); if (authenticated.value) load() })
onUnmounted(() => window.removeEventListener('popstate', syncPage))
</script>

<template>
  <main v-if="!authenticated && !authPage" class="landing-shell">
    <nav class="landing-nav">
      <a class="landing-logo" href="/"><span class="brand-mark small"><Bot :size="19" /></span><span>Xinghai</span><i>Router</i></a>
      <div class="landing-links"><a href="#features">能力</a><a href="#quickstart">快速开始</a><button class="button ghost" @click="openAuth">进入控制台 <ChevronRight :size="15" /></button></div>
    </nav>
    <section class="hero-section">
      <div class="hero-copy"><p class="eyebrow">OPENAI-COMPATIBLE MODEL GATEWAY</p><h1>让每一次模型请求，<em>走向正确的地方。</em></h1><p class="hero-description">Xinghai Router 是一个轻量、可观测的模型流量网关。统一 API、智能路由上游渠道，并把用量与成本留在你的控制台。</p><div class="hero-actions"><button class="button primary hero-button" @click="openAuth">打开控制台 <ChevronRight :size="16" /></button><a class="text-link" href="#quickstart">查看请求示例 <span>↓</span></a></div></div>
      <div class="hero-visual"><div class="visual-glow"></div><div class="route-card"><div class="route-card-top"><span><i class="live-dot"></i>ROUTER ONLINE</span><code>POST /v1/chat/completions</code></div><div class="route-model"><Bot :size="18" /><strong>gpt-4o</strong><span>智能路由中</span></div><div class="route-line"><span class="route-node active"></span><div><b>OpenAI 主线路</b><small>优先级 P10 · 42ms</small></div><span class="route-check">✓</span></div><div class="route-line muted-route"><span class="route-node"></span><div><b>备用渠道</b><small>等待流量切换</small></div></div><div class="route-footer"><span>成功率</span><strong>99.98%</strong><span class="route-divider"></span><span>平均延迟</span><strong>186ms</strong></div></div></div>
    </section>
    <section id="features" class="feature-section"><div class="section-intro"><p class="eyebrow">BUILT FOR CONTROL</p><h2>把复杂的上游，<br><em>变成一个简单的入口。</em></h2></div><div class="feature-grid"><article><span class="feature-number">01</span><RadioTower :size="21" /><h3>智能路由</h3><p>按模型、优先级和渠道状态分发请求，在上游波动时自动切换。</p></article><article><span class="feature-number">02</span><Activity :size="21" /><h3>全链路可观测</h3><p>请求日志、状态码、耗时和 Token 用量，都在一个清晰的控制台里。</p></article><article><span class="feature-number">03</span><WalletCards :size="21" /><h3>用量与成本</h3><p>为模型设置价格规则，记录每次调用费用，让团队用得明白。</p></article></div></section>
    <section id="quickstart" class="quickstart-section"><div><p class="eyebrow">ONE ENDPOINT</p><h2>接入只需要<br><em>改一个地址。</em></h2><p>使用熟悉的 OpenAI SDK，将 Base URL 指向 Xinghai Router，即刻获得统一的模型入口。</p></div><pre><span class="code-comment">// 使用 OpenAI SDK</span><span><b>const</b> client = <b>new</b> OpenAI({</span><span>  apiKey: <i>'sk-xh-your-key'</i>,</span><span>  baseURL: <i>'http://localhost:8080/v1'</i></span><span>})</span><span class="code-gap"></span><span><b>await</b> client.chat.completions.create({</span><span>  model: <i>'gpt-4o'</i>,</span><span>  messages: [{ role: <i>'user'</i>, content: <i>'你好'</i> }]</span><span>})</span></pre></section>
    <footer class="landing-footer"><span>© 2026 Xinghai Router</span><span>轻量、透明、为模型流量而生。</span></footer>
  </main>

  <main v-else-if="!authenticated" class="login-shell">
    <section class="login-card"><button class="login-close" aria-label="返回首页" @click="closeAuth">×</button><div class="brand-mark"><Bot :size="29" /></div><p class="eyebrow">XINGHAI ROUTER</p><h1>控制模型流量。</h1><div class="auth-tabs"><button :class="{ active: loginMode === 'login' }" @click="loginMode = 'login'">账户登录</button><button :class="{ active: loginMode === 'register' }" @click="loginMode = 'register'">创建账户</button></div><form @submit.prevent="accountSignIn(loginMode === 'register')"><label v-if="loginMode === 'register'">姓名<input v-model="accountForm.name" autocomplete="name" required maxlength="100" placeholder="例如：李雷" /></label><label>邮箱<input v-model="accountForm.email" type="email" autocomplete="email" required placeholder="name@example.com" /></label><label>密码<input v-model="accountForm.password" type="password" :autocomplete="loginMode === 'register' ? 'new-password' : 'current-password'" required minlength="8" placeholder="至少 8 个字符" /></label><button class="button primary full" :disabled="busy">{{ loginMode === 'register' ? '创建并进入控制台' : '登录控制台' }} <ChevronRight :size="16" /></button></form><p v-if="error" class="error"><CircleAlert :size="16" />{{ error }}</p></section>
  </main>

  <main v-else class="app-shell">
    <aside class="sidebar">
       <div class="logo"><span class="brand-mark small"><Bot :size="19" /></span><span>Xinghai</span><i>Router</i></div>
       <nav><template v-if="mode === 'admin'"><button v-for="[id, label, Icon] in adminNav" :key="id" :class="{ active: view === id }" @click="view = id"><component :is="Icon" :size="18" />{{ label }}</button></template><template v-else><button v-for="[id, label, Icon] in sessionNav" :key="id" :class="{ active: view === id }" @click="view = id"><component :is="Icon" :size="18" />{{ label }}</button></template></nav>
      <div class="sidebar-footer"><span><span class="live-dot"></span>网关在线</span><button @click="signOut"><LogOut :size="16" />退出</button></div>
    </aside>
    <section class="content">
       <header><div><p class="eyebrow">{{ mode === 'admin' ? '运营控制台' : '个人控制台' }}</p><h1>{{ [...nav, ...userNav, ...adminExtraNav].find((item) => item[0] === view)?.[1] }}</h1></div><button class="button ghost" @click="load" :disabled="busy"><RefreshCw :size="16" :class="{ spinning: busy }" />刷新</button></header>
      <p v-if="error" class="error banner"><CircleAlert :size="16" />{{ error }}</p>

      <template v-if="view === 'overview'">
        <div class="metrics"><article><span>活跃渠道</span><strong>{{ activeChannels }}<em>/{{ channels.length }}</em></strong><p><RadioTower :size="15" />可用于路由</p></article><article><span>近 100 请求</span><strong>{{ logs.length }}</strong><p><Activity :size="15" />最近记录</p></article><article><span>成功率</span><strong>{{ successRate }}<em>%</em></strong><p><Check :size="15" />HTTP 2xx / 3xx</p></article><article><span>计量 Token</span><strong>{{ totalTokens.toLocaleString() }}</strong><p><KeyRound :size="15" />非流式请求</p></article></div>
        <div class="grid-two"><section class="panel"><div class="panel-title"><div><h2>渠道状态</h2><p>按优先级选取首个可用渠道</p></div><button class="text-button" @click="view = 'channels'">管理</button></div><div v-if="channels.length" class="channel-list"><div v-for="channel in channels" :key="channel.id"><span :class="['status-dot', { off: !channel.enabled }]"></span><div><b>{{ channel.name }}</b><small>{{ channel.models.join(', ') }}</small></div><span class="priority">P{{ channel.priority }}</span></div></div><Empty v-else text="尚未配置上游渠道" /></section><section class="panel"><div class="panel-title"><div><h2>最近请求</h2><p>最近 100 条网关请求</p></div><button class="text-button" @click="view = 'logs'">查看全部</button></div><div v-if="logs.length" class="compact-list"><div v-for="log in logs.slice(0, 5)" :key="log.request_id"><code>{{ log.model }}</code><span>{{ log.duration_ms }} ms</span><b :class="log.status_code < 400 ? 'success' : 'danger'">{{ log.status_code }}</b></div></div><Empty v-else text="等待第一条模型请求" /></section></div>
      </template>

       <template v-if="view === 'users'"><section class="toolbar"><div><h2>用户与权限</h2><p>提升账户为管理员，或为运营用户授予具体权限。</p></div></section><section class="panel table-panel"><table><thead><tr><th>用户</th><th>角色</th><th>权限</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="user in users" :key="user.id"><td><b>{{ user.name }}</b><small>{{ user.email }}</small></td><td><span class="pill">{{ user.role }}</span></td><td>{{ user.role === 'admin' ? '全部权限' : user.permissions.join(', ') || '无' }}</td><td><span :class="['state', user.enabled ? 'good' : 'bad']">{{ user.enabled ? '已启用' : '已停用' }}</span></td><td><button v-if="can('system.manage')" class="text-button" @click="manageUser(user)">管理权限</button></td></tr></tbody></table><Empty v-if="!users.length" text="还没有用户" /></section></template>
      <template v-if="view === 'keys'"><section class="toolbar"><div><h2>API 密钥</h2><p>仅在创建时显示一次完整密钥。</p></div><button class="button primary" :disabled="!users.length" @click="showKey = true"><Plus :size="16" />创建密钥</button></section><section class="panel table-panel"><table><thead><tr><th>名称</th><th>所属用户</th><th>前缀</th><th>最后使用</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="key in keys" :key="key.id"><td><b>{{ key.name }}</b></td><td>{{ userName(key.user_id) }}</td><td><code>{{ key.key_prefix }}...</code></td><td>{{ formatDate(key.last_used_at) }}</td><td><span :class="['state', key.revoked_at ? 'bad' : 'good']">{{ key.revoked_at ? '已吊销' : '有效' }}</span></td><td><button v-if="!key.revoked_at" class="text-button danger" @click="revokeKey(key)">吊销</button></td></tr></tbody></table><Empty v-if="!keys.length" text="创建用户后，即可签发 API 密钥" /></section></template>
      <template v-if="view === 'channels'"><section class="toolbar"><div><h2>上游渠道</h2><p>模型请求按渠道优先级进行选择。</p></div><button class="button primary" @click="showChannel = true"><Plus :size="16" />添加渠道</button></section><div class="channel-cards"><article v-for="channel in channels" :key="channel.id" class="panel channel-card"><div class="card-top"><span :class="['status-dot', { off: !channel.enabled }]"></span><span>优先级 {{ channel.priority }}</span><button class="toggle" :class="{ on: channel.enabled }" @click="toggleChannel(channel)"><i></i></button></div><h3>{{ channel.name }}</h3><p class="url">{{ channel.base_url }}</p><div class="model-tags"><span v-for="model in channel.models" :key="model">{{ model }}</span></div></article><Empty v-if="!channels.length" text="添加 OpenAI-compatible 上游开始路由" /></div></template>
       <template v-if="view === 'logs'"><section class="toolbar"><div><h2>请求日志</h2><p>最多显示最新 100 条记录。</p></div></section><section class="panel table-panel"><table><thead><tr><th>时间</th><th>模型</th><th>状态</th><th>耗时</th><th>Token</th><th>请求 ID</th></tr></thead><tbody><tr v-for="log in logs" :key="log.request_id"><td>{{ formatDate(log.created_at) }}</td><td><code>{{ log.model }}</code></td><td><span :class="['state', log.status_code < 400 ? 'good' : 'bad']">{{ log.status_code }}</span></td><td>{{ log.duration_ms }} ms</td><td>{{ log.total_tokens ?? 0 }}</td><td><code>{{ short(log.request_id) }}</code></td></tr></tbody></table><Empty v-if="!logs.length" text="暂无请求日志" /></section></template>
         <template v-if="view === 'account'"><div class="metrics"><article><span>当前余额</span><strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong><p><WalletCards :size="15" />可用余额</p></article><article><span>预扣金额</span><strong>{{ Number(account?.reserved ?? 0).toFixed(4) }}</strong><p>并发请求保护</p></article></div><section class="panel account-card"><h2>{{ account?.name }}</h2><p>{{ account?.email }} · {{ account?.role }}</p><code>{{ account?.id }}</code></section></template>
       <template v-if="view === 'usage'"><section class="panel table-panel"><table><thead><tr><th>时间</th><th>模型</th><th>输入</th><th>输出</th><th>费用</th><th>状态</th></tr></thead><tbody><tr v-for="item in usageRecords" :key="item.request_id"><td>{{ formatDate(item.created_at) }}</td><td><code>{{ item.model }}</code></td><td>{{ item.prompt_tokens }}</td><td>{{ item.completion_tokens }}</td><td>{{ Number(item.cost).toFixed(6) }}</td><td>{{ item.status }}</td></tr></tbody></table><Empty v-if="!usageRecords.length" text="暂无用量记录" /></section></template>
       <template v-if="view === 'ledger'"><section class="panel table-panel"><table><thead><tr><th>时间</th><th>类型</th><th>变动</th><th>余额</th><th>说明</th></tr></thead><tbody><tr v-for="item in ledger" :key="item.id"><td>{{ formatDate(item.created_at) }}</td><td>{{ item.kind }}</td><td :class="item.amount < 0 ? 'danger' : 'success'">{{ item.amount }}</td><td>{{ item.balance_after }}</td><td>{{ item.note || item.request_id }}</td></tr></tbody></table><Empty v-if="!ledger.length" text="暂无余额流水" /></section></template>
       <template v-if="view === 'pricing'"><section class="toolbar"><div><h2>模型定价</h2><p>按百万 token 配置输入、缓存输入和输出价格。</p></div></section><form class="panel pricing-form" @submit.prevent="savePricing"><input v-model="pricingForm.model" required placeholder="模型名，例如 gpt-4o" /><input v-model.number="pricingForm.input_per_million" type="number" min="0" step="any" placeholder="输入价格" /><input v-model.number="pricingForm.cached_input_per_million" type="number" min="0" step="any" placeholder="缓存输入价格" /><input v-model.number="pricingForm.output_per_million" type="number" min="0" step="any" placeholder="输出价格" /><input v-model.number="pricingForm.multiplier" type="number" min="0.01" step="any" placeholder="倍率" /><button class="button primary">保存规则</button></form><section class="panel table-panel"><table><thead><tr><th>模型</th><th>输入</th><th>缓存输入</th><th>输出</th><th>倍率</th></tr></thead><tbody><tr v-for="item in pricing" :key="item.id"><td><code>{{ item.model }}</code></td><td>{{ item.input_per_million }}</td><td>{{ item.cached_input_per_million }}</td><td>{{ item.output_per_million }}</td><td>{{ item.multiplier }}</td></tr></tbody></table></section></template>
       <template v-if="view === 'audit'"><section class="panel table-panel"><table><thead><tr><th>时间</th><th>动作</th><th>对象</th><th>详情</th></tr></thead><tbody><tr v-for="item in auditLogs" :key="String(item.id)"><td>{{ formatDate(String(item.created_at)) }}</td><td>{{ item.action }}</td><td>{{ item.entity_type }} / {{ item.entity_id }}</td><td><code>{{ JSON.stringify(item.details) }}</code></td></tr></tbody></table><Empty v-if="!auditLogs.length" text="暂无操作审计" /></section></template>
    </section>

    <div v-if="selectedUser || showKey || showChannel || createdKey" class="modal-backdrop" @click.self="selectedUser = null; showKey = showChannel = false">
      <form v-if="selectedUser" class="modal" @submit.prevent="saveUserAccess"><div class="modal-title"><h2>管理用户权限</h2><button type="button" @click="selectedUser = null">×</button></div><p class="muted">{{ selectedUser.name }} · {{ selectedUser.email }}</p><label>角色<select v-model="selectedUser.role"><option value="user">用户</option><option value="operator">运营</option><option value="admin">管理员（全部权限）</option></select></label><label v-if="selectedUser.role !== 'admin'">细粒度权限<select v-model="selectedPermissions" multiple size="8"><option v-for="permission in permissions" :key="permission" :value="permission">{{ permission }}</option></select></label><button class="button primary full" :disabled="busy">保存访问权限</button></form>
      <form v-if="showKey" class="modal" @submit.prevent="createKey"><div class="modal-title"><h2>创建 API 密钥</h2><button type="button" @click="showKey = false">×</button></div><label>用户<select v-model="keyForm.user_id" required><option disabled value="">选择用户</option><option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option></select></label><label>密钥名称<input v-model="keyForm.name" required placeholder="例如：生产环境" /></label><label>过期时间 <small>可选</small><input v-model="keyForm.expires_at" type="datetime-local" /></label><button class="button primary full" :disabled="busy">签发密钥</button></form>
      <form v-if="showChannel" class="modal" @submit.prevent="createChannel"><div class="modal-title"><h2>添加上游渠道</h2><button type="button" @click="showChannel = false">×</button></div><label>名称<input v-model="channelForm.name" required placeholder="例如：OpenAI 主线路" /></label><label>Base URL<input v-model="channelForm.base_url" required type="url" /></label><label>上游 API Key<input v-model="channelForm.api_key" required type="password" /></label><label>模型 <small>逗号分隔</small><input v-model="channelForm.models" required placeholder="gpt-4o-mini, gpt-4o" /></label><label>优先级 <input v-model.number="channelForm.priority" type="number" min="0" /></label><button class="button primary full" :disabled="busy">保存渠道</button></form>
      <section v-if="createdKey" class="modal secret"><div class="modal-title"><h2>保存 API 密钥</h2><button @click="createdKey = ''">×</button></div><p>完整密钥只显示这一次，请立即保存。</p><code>{{ createdKey }}</code><button class="button primary full" @click="copyKey"><Copy :size="16" />复制密钥</button><button class="button ghost full" @click="createdKey = ''">我已保存</button></section>
    </div>
  </main>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
export default defineComponent({ components: { Empty: { props: { text: { type: String, required: true } }, template: '<div class="empty">{{ text }}</div>' } } })
</script>
