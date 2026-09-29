import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'
import { renderToString } from 'vue/server-renderer'

const root = new URL('../', import.meta.url)
const pageSource = await readFile(new URL('pages/console/clusters.vue', root), 'utf8')
const apiSource = await readFile(new URL('src/api.ts', root), 'utf8')
const resourceSource = await readFile(new URL('composables/useResource.ts', root), 'utf8')
const { descriptor } = parse(pageSource)
const script = compileScript(descriptor, { id: 'clusters-test' })
const template = compileTemplate({
  id: 'clusters-test',
  filename: 'clusters.vue',
  source: descriptor.template.content,
  compilerOptions: { bindingMetadata: script.bindings },
})
assert.deepEqual(template.errors, [])

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
  enabled: false, created_at: '',
}

function fixture(allowed = true) {
  const calls = []
  const messages = []
  const mounted = []
  let respond = url => json(url === '/api/admin/clusters' ? { data: [cluster] }
    : url === '/api/admin/instances' ? { data: [instance] } : { id: 'created' })
  const context = vm.createContext({
    ...vue, Error, Headers, Response,
    onMounted: callback => mounted.push(callback),
    definePageMeta: () => {},
    useI18n: () => ({ t: key => key }),
    useAccount: () => ({ can: () => allowed }),
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
  Object.assign(context, evaluate(resourceSource, context))
  context.require = name => {
    if (name === 'vue') return vue
    if (name === '~/src/api') return api
    if (name === 'lucide-vue-next') return { Server: { render: () => vue.h('span') } }
    throw new Error(`Unexpected module ${name}`)
  }
  const component = evaluate(script.content, context).default
  const render = evaluate(template.code, context).render
  const state = component.setup({}, { expose: () => {} })

  async function html() {
    const app = vue.createSSRApp({ ...component, setup: () => state, render })
    const container = { setup: (_, { slots }) => () => vue.h('div', slots.default?.()) }
    for (const name of ['UiCard', 'UiTable', 'UiButton', 'UiField']) app.component(name, container)
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

test('instance creation validates its cluster and address and sends the nested API contract', async () => {
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
  assert.deepEqual(f.messages.at(-1), ['error', 'admin.instanceAddressRequired'])
  assert.equal(f.calls.filter(call => call.method !== 'GET').length, 0)
  f.state.form.endpoint = ' https://new.example.test '
  await f.state.save()
  assert.deepEqual(f.calls.find(call => call.method === 'POST'), {
    url: '/api/admin/clusters/cluster-a/instances', method: 'POST',
    body: { name: 'New instance', address: 'https://new.example.test' },
  })
  assert.equal(f.state.open.value, false)
  assert.equal(f.calls.filter(call => call.method === 'GET').length, 4)
})

test('instance editing and deletion retain the original cluster and use matching nested URLs', async () => {
  const f = fixture()
  await f.mount()
  f.state.edit(instance)
  f.state.form.name = 'Renamed instance'
  f.state.form.cluster_id = 'different-cluster'
  f.state.tab.value = 'clusters'
  await f.state.save()
  assert.deepEqual(f.calls.find(call => call.method === 'PUT'), {
    url: '/api/admin/clusters/cluster-a/instances/instance-a', method: 'PUT',
    body: { name: 'Renamed instance', address: instance.address },
  })
  await f.state.remove(instance)
  assert.equal(f.calls.find(call => call.method === 'DELETE').url, '/api/admin/clusters/cluster-a/instances/instance-a')
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
