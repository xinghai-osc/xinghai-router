import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import { renderToString } from 'vue/server-renderer'

const root = new URL('../', import.meta.url)

const apiSource = await readFile(new URL('src/api.ts', root), 'utf8')
const resourceSource = await readFile(new URL('composables/useResource.ts', root), 'utf8')
async function compilePage(name) {
  const filename = `${name}.vue`
  const { descriptor } = parse(await readFile(new URL(`pages/console/${filename}`, root), 'utf8'))
  const script = compileScript(descriptor, { id: name })
  const template = compileTemplate({
    id: name, filename, source: descriptor.template.content,
    compilerOptions: { bindingMetadata: script.bindings },
  })
  assert.deepEqual(template.errors, [])
  return { script, template }
}
const clusterPage = await compilePage('clusters')
const sitePage = await compilePage('site-settings')

function evaluate(source, context) {
  context.exports = {}
  const code = ts.transpileModule(source.replaceAll('import.meta.client', 'true'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText
  vm.runInContext(`{${code}}`, context)
  return context.exports
}

const json = (body, status = 200) => new Response(JSON.stringify(body), {
  status,
  headers: { 'Content-Type': 'application/json' },
})
const cluster = {
  id: 'cluster-a', name: 'Cluster A', description: 'Keep this description',
  endpoint: 'https://cluster.example.test', enabled: false, manager_id: null,
  instance_count: 1, created_at: '', updated_at: '',
}
const instance = {
  id: 'instance-a', cluster_id: cluster.id, name: 'Instance A',
  address: 'https://instance.example.test', endpoint: 'https://instance.example.test',
  enabled: false, created_at: '', deploy_status: 'idle', ssh_host: 'example.test',
  ssh_port: 22, ssh_user: 'router', ssh_host_key: 'SHA256:fixture', ssh_auth_method: 'password',
  deploy_path: '/srv/router', router_image: 'router:test',
}

function fixture(allowed = true, admin = true, { script, template } = clusterPage) {
  const calls = []
  const messages = []
  const mounted = []
  let respond = url => json(url === '/api/admin/clusters' ? { data: [cluster] }
    : url === '/api/admin/nodes' ? { data: [instance] } : { id: 'created' })
  const context = vm.createContext({
    ...vue, Error, Headers, Response,
    onMounted: callback => mounted.push(callback),
    definePageMeta: () => {},
    useI18n: () => ({ t: key => key }),
    useAccount: () => ({ can: () => allowed, isAdmin: vue.ref(admin) }),
    useSiteSettings: () => ({ settings: vue.ref({ name: 'Test' }) }),
    useHead: () => {},
    useToast: () => ({ toast: {
      success: message => messages.push(['success', message]),
      error: message => messages.push(['error', message]),
    } }),
    confirm: () => true,
    fetch: async (url, init) => {
      calls.push({ url, method: init.method ?? 'GET', body: init.body ? JSON.parse(init.body) : undefined })
      return respond(url, init)
    },
  })
  const api = evaluate(apiSource, context)
  context.require = name => {
    if (name === 'vue') return vue
    if (name === '~/src/api' || name === '../src/api') return api
    if (name === 'lucide-vue-next') return { Server: { render: () => vue.h('span') } }
    throw new Error(`Unexpected module ${name}`)
  }
  Object.assign(context, evaluate(resourceSource, context))
  const component = evaluate(script.content, context).default
  const render = evaluate(template.code, context).render
  const state = component.setup({}, { expose: () => {} })

  async function html() {
    const app = vue.createSSRApp({ ...component, setup: () => state, render })
    const container = { setup: (_, { slots }) => () => vue.h('div', slots.default?.()) }
    for (const name of ['UiCard', 'UiTable', 'UiButton', 'UiField', 'UiBadge', 'UiTextarea', 'UiSwitch', 'ConsoleSystemGate']) app.component(name, container)
    app.component('UiAlert', { props: ['title'], setup: props => () => vue.h('div', { role: 'alert' }, props.title) })
    app.component('UiEmptyState', { props: ['title'], setup: props => () => vue.h('div', { 'data-empty': '' }, props.title) })
    app.component('UiSkeleton', { render: () => vue.h('div', { 'data-loading': '' }) })
    app.component('UiTabs', { render: () => vue.h('div') })
    app.component('UiSelect', { render: () => vue.h('div') })
    app.component('UiInput', { render: () => vue.h('input') })
    app.component('UiDialog', { props: ['open'], setup: (props, { slots }) => () => props.open ? vue.h('div', slots.default?.()) : null })
    return renderToString(app)
  }

  return { state, calls, messages, html, api, mount: () => Promise.all(mounted.map(callback => callback())), respond: callback => { respond = callback } }
}

test('cluster and instance loading, loaded and empty states render values instead of Ref objects', async () => {
  const f = fixture()
  const loading = []
  f.respond(() => new Promise(resolve => loading.push(resolve)))
  const mounted = f.mount()
  assert.match(await f.html(), /data-loading/)
  assert.doesNotMatch(await f.html(), /\[object Object\]|role="alert"/)
  loading[0](json({ data: [cluster] }))
  loading[1](json({ data: [instance] }))
  await mounted
  assert.match(await f.html(), /Cluster A/)
  assert.doesNotMatch(await f.html(), /data-loading|data-empty|role="alert"/)
  f.state.tab.value = 'instances'
  const instances = await f.html()
  assert.match(instances, /Instance A/)
  assert.match(instances, /Cluster A/)
  assert.match(instances, /https:\/\/instance.example.test/)
  f.respond(() => json({ data: [] }))
  await f.state.refresh()
  for (const tab of ['clusters', 'instances']) {
    f.state.tab.value = tab
    assert.match(await f.html(), /data-empty/)
    assert.doesNotMatch(await f.html(), /data-loading|role="alert"|\[object Object\]/)
  }
})

test('list failures display the server message only for the affected tab and recover on retry', async () => {
  const f = fixture()
  f.respond(url => url.endsWith('/clusters')
    ? json({ error: { code: 'internal_error', message: 'Cluster query failed' } }, 500)
    : json({ data: [] }))
  await f.mount()
  assert.match(await f.html(), /role="alert"[^>]*>Cluster query failed/)
  assert.doesNotMatch(await f.html(), /data-empty|data-loading|\[object Object\]/)
  f.state.tab.value = 'instances'
  assert.match(await f.html(), /data-empty/)
  assert.doesNotMatch(await f.html(), /role="alert"/)
  f.respond(() => json({ data: [] }))
  await f.state.refresh()
  f.state.tab.value = 'clusters'
  assert.match(await f.html(), /data-empty/)
  assert.doesNotMatch(await f.html(), /role="alert"/)
})

test('accounts without system.manage do not fetch cluster resources', async () => {
  const f = fixture(false)
  await f.mount()
  assert.equal(f.calls.length, 0)
  assert.match(await f.html(), /admin.noAccessTitle/)
})

test('node creation validates its cluster and SSH configuration and sends the deployment request', async () => {
  const f = fixture()
  await f.mount()
  f.state.tab.value = 'instances'
  f.state.create()
  f.state.form.name = ' New instance '
  f.state.form.cluster_id = ''
  await f.state.save()
  assert.deepEqual(f.messages.at(-1), ['error', 'admin.clusterRequired'])
  f.state.form.cluster_id = cluster.id
  await f.state.save()
  assert.deepEqual(f.messages.at(-1), ['error', 'admin.nodeSSHRequired'])
  assert.equal(f.calls.filter(call => call.method !== 'GET').length, 0)
  Object.assign(f.state.form, { endpoint: ' https://new.example.test ', ssh_host: 'new.example.test', ssh_user: 'router', ssh_host_key: 'SHA256:fixture', ssh_password: 'fixture' })
  await f.state.save()
  const created = f.calls.find(call => call.method === 'POST')
  assert.equal(created.url, '/api/admin/clusters/cluster-a/nodes')
  assert.equal(created.body.name, 'New instance')
  assert.equal(created.body.ssh_host, 'new.example.test')
  assert.equal(created.body.address, 'https://new.example.test')
  assert.equal(created.body.deploy, true)
  assert.equal(f.state.open.value, false)
  assert.equal(f.calls.filter(call => call.method === 'GET').length, 4)
})

test('node edits, deployment and deletion use the dedicated node APIs', async () => {
  const f = fixture()
  await f.mount()
  f.state.edit(instance)
  f.state.form.name = 'Renamed instance'
  f.state.form.cluster_id = 'different-cluster'
  f.state.tab.value = 'clusters'
  await f.state.save()
  const updated = f.calls.find(call => call.method === 'PUT')
  assert.equal(updated.url, '/api/admin/nodes/instance-a')
  assert.equal(updated.body.name, 'Renamed instance')
  assert.equal(updated.body.deploy, false)
  await f.state.deploy(instance)
  assert.equal(f.calls.find(call => call.method === 'POST').url, '/api/admin/nodes/instance-a/deploy')
  await f.state.remove(instance)
  assert.equal(f.calls.find(call => call.method === 'DELETE').url, '/api/admin/nodes/instance-a')
})

test('cluster edits preserve unexposed fields and failed mutations remain visible', async () => {
  const f = fixture()
  await f.mount()
  f.state.edit(cluster)
  f.state.form.name = 'Renamed cluster'
  f.respond(() => json({ error: { code: 'conflict', message: 'Name already exists' } }, 409))
  await f.state.save()
  assert.deepEqual(f.calls.find(call => call.method === 'PUT').body, {
    name: 'Renamed cluster', endpoint: cluster.endpoint, description: cluster.description, enabled: false,
  })
  assert.equal(f.state.open.value, true)
  assert.deepEqual(f.messages.at(-1), ['error', 'Name already exists'])
  assert.match(await f.html(), /role="alert"[^>]*>Name already exists/)
  await f.state.remove(cluster)
  assert.deepEqual(f.messages.at(-1), ['error', 'Name already exists'])
  await f.state.sync(cluster)
  assert.deepEqual(f.messages.at(-1), ['error', 'Name already exists'])
})

test('instance path parameters are URL-encoded and sync reports a queued task', async () => {
  const f = fixture()
  await f.api.endpoints.createAdminInstance('cluster/a', { name: 'test', address: 'host:8080' })
  await f.api.endpoints.updateAdminInstance('cluster/a', 'instance/b', { name: 'test', address: 'host:8080' })
  await f.api.endpoints.deleteAdminInstance('cluster/a', 'instance/b')
  assert.equal(f.calls[0].url, '/api/admin/clusters/cluster%2Fa/instances')
  assert.equal(f.calls[1].url, '/api/admin/clusters/cluster%2Fa/instances/instance%2Fb')
  assert.equal(f.calls[2].url, f.calls[1].url)
  await f.state.sync(cluster)
  assert.deepEqual(f.messages.at(-1), ['success', 'admin.clusterSyncQueued'])
})


test('system operators can inspect nodes but cannot create, edit, deploy or delete them', async () => {
  const f = fixture(true, false)
  await f.mount()
  assert.match(await f.html(), /admin.create/)
  f.state.tab.value = 'instances'
  const html = await f.html()
  assert.match(html, /Instance A/)
  assert.doesNotMatch(html, /admin\.(create|edit|nodeDeploy<|delete)/)
  f.state.create()
  f.state.edit(instance)
  assert.equal(f.state.open.value, false)
  f.state.formTab.value = 'instances'
  Object.assign(f.state.form, instance)
  await f.state.save()
  await f.state.remove(instance)
  await f.state.deploy(instance)
  assert.equal(f.calls.filter(call => call.method !== 'GET').length, 0)
})

for (const admin of [false, true]) {
  test(`${admin ? 'admin' : 'operator'} ordinary site updates omit every recovery field`, async () => {
    const f = fixture(true, admin, sitePage)
    const saved = { ...f.state.data.value, smtp_host: 'mail.example.test', public_base_url: 'https://example.test' }
    f.respond(() => json(saved))
    await f.mount()
    const html = await f.html()
    if (admin) assert.match(html, /system.smtpPassword/)
    else assert.doesNotMatch(html, /system.smtpPassword|system.publicBaseUrl/)
    if (!admin) {
      f.state.recovery.smtp_host = 'other.example.test'
      f.state.smtpPassword.value = 'fixture'
    }
    f.state.form.name = 'Updated'
    await f.state.save()
    const body = f.calls.find(call => call.method === 'PUT').body
    assert.equal(body.name, 'Updated')
    assert.deepEqual(Object.keys(body).filter(key => key.startsWith('smtp_') || key === 'public_base_url'), [])
  })
}

test('admin recovery changes transmit only modified fields, including deliberate clears', async () => {
  const f = fixture(true, true, sitePage)
  f.respond(() => json({ ...f.state.data.value, smtp_host: 'mail.example.test', public_base_url: 'https://example.test' }))
  await f.mount()
  f.state.recovery.public_base_url = ''
  f.state.smtpPassword.value = 'fixture'
  await f.state.save()
  const body = f.calls.find(call => call.method === 'PUT').body
  assert.deepEqual(Object.keys(body).filter(key => key.startsWith('smtp_') || key === 'public_base_url').sort(), ['public_base_url', 'smtp_password'])
  assert.equal(body.public_base_url, '')
  assert.equal(body.smtp_password, 'fixture')
})
