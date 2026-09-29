import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import { renderToString } from 'vue/server-renderer'

const root = new URL('../', import.meta.url)
const source = path => readFile(new URL(path, root), 'utf8')
const resourceSource = await source('composables/useResource.ts')
const rewardSource = await source('composables/useRewardStatus.ts')
const collectorSource = await source('src/risk-context.ts')
const pages = {}
for (const name of ['console/admin-reward-risk', 'console/checkin', 'console/invitations', 'auth']) {
  const { descriptor } = parse(await source(`pages/${name}.vue`))
  const script = compileScript(descriptor, { id: name })
  const template = compileTemplate({ id: name, filename: `${name}.vue`, source: descriptor.template.content, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  pages[name] = { script: script.content, template: template.code }
}
const claim = { id: 'claim-1', user_id: 'user-1', user_name: 'Example User', email: 'user@example.test', origin_user_id: 'user-2', source: 'checkin', source_id: 'source-1', amount: '0.5', status: 'pending', reasons: ['username_cluster', 'unverified_context'], created_at: '', updated_at: '' }
const ban = { id: 'case-1', user_id: 'user-1', user_name: 'Example User', email: claim.email, rule_version: 2, details: { count: 3 }, observation_id: 'event-1', banned_at: '', released_at: null, released_by: null, release_reason: '' }
const event = { id: 'event-1', user_id: 'user-1', user_name: 'Example User', email: claim.email, action: 'register', decision: 'review', reasons: ['username_cluster'], details: { count: 3 }, created_at: '' }
const config = { enabled: true, auto_ban_enabled: true, webrtc_enabled: true, stun_urls: ['stun:stun.cloudflare.com:3478'], window_hours: 24, username_similarity: 0.85, similar_accounts: 3, burst_minutes: 10, burst_accounts: 5, registrations_per_ip: 3, checkins_per_ip: 3, checkins_per_browser: 1, invitations_per_inviter: 10, version: 7 }
const pageOf = data => ({ data, total: data.length, page: 1, page_size: 50 })
const plain = value => JSON.parse(JSON.stringify(value))
function evaluate(source, context) {
  context.exports = {}
  const code = ts.transpileModule(source.replaceAll('import.meta.client', 'true'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  vm.runInContext(`{${code}}`, context)
  return context.exports
}
function fixture(permissions = [], name = 'console/admin-reward-risk') {
  const calls = []
  const messages = []
  const mounted = []
  let contextFailure = false
  const results = {
    getRewardRiskClaims: pageOf([claim]), getRewardRiskEvents: pageOf([event]), getRewardRiskBans: pageOf([ban]), getRewardRiskSettings: config,
    getCheckinStatus: { checked_in: false, data: [] }, checkin: { checked_in: true, already_checked_in: false, reward_status: 'pending', reward: 0, scheduled_reward: 0.5 },
    getAccountInvitations: { enabled: true, code: 'invite-1', inviter_reward: '1', invitee_reward: '1', data: [] },
  }
  let respond = (method) => results[method] ?? { ok: true }
  const endpoints = new Proxy({}, { get: (_, method) => async (...args) => { calls.push({ method, args: plain(args) }); return respond(method, args) } })
  class ApiError extends Error { constructor(message, code) { super(message); this.code = code } }
  const context = vm.createContext({
    ...vue, URL, URLSearchParams, Error,
    onMounted: callback => mounted.push(callback), onBeforeUnmount: () => {},
    definePageMeta: () => {}, useHead: () => {}, usePageSeo: () => {},
    useI18n: () => ({ t: key => key }),
    useAccount: () => ({ can: permission => permissions.includes(permission), authenticated: vue.ref(false), loadAccount: async () => {}, signIn: async () => {} }),
    useSiteSettings: () => ({ settings: vue.ref({ name: 'Test', risk_probe: { enabled: true }, oauth_providers: ['github'] }) }),
    useToast: () => ({ toast: Object.fromEntries(['info', 'success', 'warn', 'error'].map(tone => [tone, message => messages.push([tone, message])])) }),
    useRiskContext: () => ({ prepare: async purpose => { calls.push({ method: 'riskContext', args: [purpose] }); if (contextFailure) throw new Error('Evidence could not be saved'); return 'risk-1' } }),
    useGeetest: () => ({ challenge: async () => ({}) }), useCorptcha: () => ({ challenge: async () => ({}) }),
    useRoute: () => ({ query: {}, path: '/auth' }), useRouter: () => ({ replace: () => {} }), navigateTo: async target => calls.push({ method: 'navigate', args: [target] }),
    window: { location: { origin: 'https://example.test', assign: target => calls.push({ method: 'oauth', args: [target] }) } },
  })
  const collector = evaluate(collectorSource, context)
  Object.assign(context, evaluate(resourceSource, context), evaluate(rewardSource, context))
  context.require = path => {
    if (path === 'vue') return vue
    if (path === '~/src/api') return { endpoints, ApiError }
    if (path === '~/src/risk-context') return collector
    if (path === '~/src/format') return { formatDateTime: value => value, formatDate: value => value, formatMoney: value => `$${value}` }
    if (path === 'lucide-vue-next') return { ShieldAlert: {}, CalendarCheck: {}, Copy: {}, UserPlus: {} }
    if (path === '@vueuse/core') return { useClipboard: () => ({ copy: () => {} }) }
    if (path.startsWith('~/composables/use')) return {}
    throw new Error(`Unexpected module ${path}`)
  }
  const component = evaluate(pages[name].script, context).default
  const render = evaluate(pages[name].template, context).render
  const state = component.setup({}, { expose: () => {} })
  async function html() {
    const app = vue.createSSRApp({ ...component, setup: () => state, render })
    const container = { setup: (_, { slots }) => () => vue.h('div', slots.default?.()) }
    for (const tag of ['UiCard', 'UiTable', 'UiButton', 'UiField', 'UiBadge', 'ConsoleUserDataState']) app.component(tag, container)
    app.component('UiAlert', { props: ['title'], setup: (props, { slots }) => () => vue.h('div', { role: 'alert' }, [props.title, slots.default?.()]) })
    app.component('UiEmptyState', { props: ['title'], setup: props => () => vue.h('div', { 'data-empty': '' }, props.title) })
    app.component('UiSkeleton', { render: () => vue.h('div', { 'data-loading': '' }) })
    for (const tag of ['UiTabs', 'UiSelect', 'UiSwitch']) app.component(tag, { render: () => vue.h('div') })
    for (const tag of ['UiInput', 'UiTextarea']) app.component(tag, { render: () => vue.h('input') })
    app.component('NuxtLink', container)
    app.component('UiDialog', { props: ['open'], setup: (props, { slots }) => () => props.open ? vue.h('div', slots.default?.()) : null })
    return renderToString(app)
  }
  return { state, calls, messages, results, html, mount: () => Promise.all(mounted.map(fn => fn())), respond: fn => { respond = fn }, failContext: () => { contextFailure = true }, ApiError }
}

test('permissions independently gate initial tabs and all read endpoints', async () => {
  for (const [permissions, tabs, method] of [
    [[], [], null], [['users.read'], ['claims', 'bans'], 'getRewardRiskClaims'], [['audit.read'], ['events'], 'getRewardRiskEvents'], [['system.manage'], ['settings'], 'getRewardRiskSettings'],
  ]) {
    const f = fixture(permissions)
    await f.mount()
    assert.deepEqual(plain(f.state.tabs.value.map(tab => tab.value)), tabs)
    assert.deepEqual(f.calls.map(call => call.method), method ? [method] : [])
    if (!permissions.length) assert.match(await f.html(), /admin.noAccessTitle/)
    for (const tab of ['claims', 'events', 'bans', 'settings']) {
      if (tabs.includes(tab)) continue
      const before = f.calls.length
      f.state.tab.value = tab
      await vue.nextTick()
      await f.state.refresh()
      assert.equal(f.calls.length, before)
    }
  }
})

test('claim list renders loading, error, empty and loaded states with a retry', async () => {
  const f = fixture(['users.read'])
  let resolve
  f.respond(() => new Promise(done => { resolve = done }))
  const mounting = f.mount()
  assert.match(await f.html(), /data-loading/)
  resolve(pageOf([claim]))
  await mounting
  assert.match(await f.html(), /Example User/)
  assert.match(await f.html(), /console.rewardPending/)
  assert.match(await f.html(), /admin.rewardRiskReason_unverified_context/)
  assert.doesNotMatch(await f.html(), /\[object Object\]|data-loading/)
  f.respond(() => { throw new Error('Review list unavailable') })
  await f.state.refresh()
  assert.match(await f.html(), /Review list unavailable/)
  f.respond(() => pageOf([]))
  await f.state.refresh()
  assert.match(await f.html(), /admin.rewardRiskEmpty/)
})

test('readers cannot approve reject or unban; wallet managers can review pending claims', async () => {
  const reader = fixture(['users.read'])
  await reader.mount()
  reader.state.openClaim(claim, 'approve')
  reader.state.openUnban(ban)
  assert.equal(reader.state.target.value, null)
  assert.doesNotMatch(await reader.html(), /admin.rewardRiskApprove|admin.rewardRiskReject/)
  const f = fixture(['users.read', 'wallets.manage'])
  await f.mount()
  f.state.openClaim({ ...claim, status: 'credited' }, 'approve')
  assert.equal(f.state.target.value, null)
  f.state.openClaim(claim, 'approve')
  f.state.reason.value = ' Reviewed '
  await f.state.submitDecision()
  assert.deepEqual(f.calls.find(call => call.method === 'approveRewardRiskClaim').args, ['claim-1', 'Reviewed'])
  assert.equal(f.state.target.value, null)
  f.state.openClaim(claim, 'reject')
  f.respond(() => { throw new Error('Already processed') })
  await f.state.submitDecision()
  assert.equal(f.state.target.value.kind, 'reject')
  assert.equal(f.state.mutationError.value, 'Already processed')
})

test('releasing a restriction requires a reason and submits the exact case', async () => {
  const f = fixture(['users.read', 'users.manage'])
  await f.mount()
  f.state.openUnban(ban)
  assert.equal(f.state.actionHint.value, 'admin.rewardRiskUnbanHint')
  await f.state.submitDecision()
  assert.equal(f.state.mutationError.value, 'admin.rewardRiskReasonRequired')
  assert.equal(f.calls.some(call => call.method === 'unbanRewardRiskUser'), false)
  f.state.reason.value = ' Manually verified '
  await f.state.submitDecision()
  assert.deepEqual(f.calls.find(call => call.method === 'unbanRewardRiskUser').args, ['user-1', 'case-1', 'Manually verified'])
})

test('settings exclude the readonly version and validate backend bounds and STUN URLs', async () => {
  const f = fixture(['system.manage'])
  await f.mount()
  await vue.nextTick()
  assert.equal(f.state.settingsLoaded.value, true)
  f.state.form.value.username_similarity = 0.5
  await f.state.saveSettings()
  assert.equal(f.state.mutationError.value, 'system.rewardRiskInvalid')
  assert.equal(f.calls.some(call => call.method === 'updateRewardRiskSettings'), false)
  f.state.form.value.username_similarity = 0.85
  f.state.stunUrls.value = 'turn:example.test:3478'
  await f.state.saveSettings()
  assert.equal(f.state.mutationError.value, 'system.rewardRiskStunInvalid')
  f.state.stunUrls.value = ''
  await f.state.saveSettings()
  const body = f.calls.find(call => call.method === 'updateRewardRiskSettings').args[0]
  assert.equal('version' in body, false)
  assert.deepEqual(body.stun_urls, [])
})

test('pending check-ins submit context and never show a credited-money toast', async () => {
  const f = fixture([], 'console/checkin')
  await f.mount()
  await f.state.submit()
  assert.deepEqual(f.calls.find(call => call.method === 'checkin').args, [{ risk_context_id: 'risk-1' }])
  assert.deepEqual(f.messages, [['info', 'console.checkinPending']])
  const failing = fixture([], 'console/checkin')
  failing.failContext()
  await failing.state.submit()
  assert.equal(failing.calls.some(call => call.method === 'checkin'), false)
  assert.equal(failing.state.formError.value, 'Evidence could not be saved')
})

test('invitation totals count credited and legacy entries only', async () => {
  const f = fixture([], 'console/invitations')
  f.results.getAccountInvitations.data = [
    { reward: '2', inviter_reward_status: 'pending' }, { reward: '3', inviter_reward_status: 'credited' },
    { reward: '5', inviter_reward_status: 'rejected' }, { reward: '7', inviter_reward_status: 'withdrawn' }, { reward: '11' },
  ]
  await f.mount()
  assert.equal(f.state.totalReward.value, 14)
})

test('normal login skips collection while registration and OAuth prepare context first', async () => {
  const login = fixture([], 'auth')
  Object.assign(login.state.form, { email: 'test@example.test', password: 'fake-password-123' })
  await login.state.submit()
  assert.equal(login.calls.some(call => call.method === 'riskContext'), false)
  const register = fixture([], 'auth')
  register.state.mode.value = 'register'
  Object.assign(register.state.form, { name: 'Test', email: 'test@example.test', password: 'fake-password-123' })
  await register.state.submit()
  assert.deepEqual(register.calls.slice(0, 2).map(call => call.method), ['riskContext', 'register'])
  assert.equal(register.calls.find(call => call.method === 'register').args[0].risk_context_id, 'risk-1')
  const oauth = fixture([], 'auth')
  await oauth.state.startOAuth('github')
  assert.deepEqual(oauth.calls, [{ method: 'riskContext', args: ['oauth'] }, { method: 'oauth', args: ['/api/auth/oauth/github?risk_context_id=risk-1'] }])
  const failure = fixture([], 'auth')
  failure.failContext()
  await failure.state.startOAuth('github')
  assert.equal(failure.calls.some(call => call.method === 'oauth'), false)
  assert.equal(failure.state.formError.value, 'Evidence could not be saved')
  assert.equal(failure.state.busy.value, false)
})

test('settings PUT retries after recent authentication and cancellation preserves the server failure', async () => {
  const apiSource = await source('src/api.ts')
  for (const accept of [true, false]) {
    const calls = []
    let reauth = 0
    const context = vm.createContext({
      Headers, Response, Error,
      fetch: async (url, init) => {
        calls.push({ url, method: init.method, body: JSON.parse(init.body) })
        return calls.length === 1
          ? new Response(JSON.stringify({ error: { code: 'reauthentication_required', message: 'Verify your session' } }), { status: 403 })
          : new Response(JSON.stringify(config), { status: 200 })
      },
    })
    const api = evaluate(apiSource, context)
    api.setReauthenticationHandler(async () => { reauth++; return accept })
    const body = { ...config }
    delete body.version
    const save = api.endpoints.updateRewardRiskSettings(body)
    if (accept) assert.deepEqual(plain(await save), config)
    else await assert.rejects(save, error => error.code === 'reauthentication_required')
    assert.equal(reauth, 1)
    assert.equal(calls.length, accept ? 2 : 1)
    for (const call of calls) {
      assert.equal(call.url, '/api/admin/reward-risk/settings')
      assert.equal(call.method, 'PUT')
      assert.deepEqual(call.body, body)
      assert.equal('version' in call.body, false)
    }
  }
})

test('new reward-risk translations exist in all three locales with matching placeholders', async () => {
  for (const ns of ['admin', 'system', 'nav', 'console', 'auth', 'site']) {
    const dictionaries = []
    for (const locale of ['en', 'zh', 'zh-Hant']) dictionaries.push(evaluate(await source(`src/locales/${locale}/${ns}.ts`), vm.createContext({})).default)
    const relevant = key => /^(reward|riskProbe|adminRewardRisk|accountRestricted|oauthContinue|checkinPending|checkinRecorded|checkinProbeNotice|inviterRewardStatus|inviteeRewardStatus|invitationPotentialReward)/.test(key)
    const keys = Object.keys(dictionaries[0]).filter(relevant).sort()
    for (const dictionary of dictionaries.slice(1)) {
      assert.deepEqual(Object.keys(dictionary).filter(relevant).sort(), keys)
      for (const key of keys) assert.deepEqual((dictionary[key].match(/\{\w+\}/g) ?? []).sort(), (dictionaries[0][key].match(/\{\w+\}/g) ?? []).sort(), `${ns}.${key}`)
    }
  }
})
