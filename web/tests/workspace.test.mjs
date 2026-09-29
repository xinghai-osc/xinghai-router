import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import { computed, nextTick, ref } from 'vue'

const source = await readFile(new URL('../src/api.ts', import.meta.url), 'utf8')
const workspaceSource = await readFile(new URL('../composables/useWorkspace.ts', import.meta.url), 'utf8')

function workspaceFixture(loader) {
  const account = ref({ id: 'user-a' })
  const storage = new Map()
  const scopes = []
  const states = new Map([['account', account]])
  const context = vm.createContext({
    exports: {}, computed, nextTick,
    require: () => ({ endpoints: { getWorkspaces: loader }, setWorkspaceRequestScope: id => scopes.push(id) }),
    useI18n: () => ({ t: key => key }),
    useToast: () => ({ toast: { error: () => {} } }),
    useState: (key, create) => {
      if (!states.has(key)) states.set(key, ref(create()))
      return states.get(key)
    },
    localStorage: { getItem: key => storage.get(key), setItem: (key, value) => storage.set(key, value) },
  })
  vm.runInContext(ts.transpileModule(workspaceSource.replaceAll('import.meta.client', 'true'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, context)
  return { workspace: context.exports.useWorkspace(), account, storage, scopes }
}

test('removed stored memberships fall back to personal and another account cannot inherit selection', async () => {
  const f = workspaceFixture(async () => ({
    data: [{ id: `${f.account.value.id}-personal`, is_personal: true }],
    current_id: `${f.account.value.id}-personal`,
  }))
  f.storage.set('xinghai.workspace:user-a', 'removed-team')
  await f.workspace.loadWorkspaces()
  assert.equal(f.workspace.current.value.id, 'user-a-personal')
  f.account.value = { id: 'user-b' }
  assert.equal(f.workspace.ready.value, false)
  assert.equal(f.workspace.current.value, null)
  await f.workspace.loadWorkspaces()
  assert.equal(f.workspace.current.value.id, 'user-b-personal')
  assert.equal(f.scopes.at(-1), 'user-b-personal')
})

test('a late membership response after logout cannot restore workspace state', async () => {
  const list = deferred()
  const f = workspaceFixture(() => list.promise)
  const pending = f.workspace.loadWorkspaces()
  f.workspace.resetWorkspace()
  f.account.value = null
  list.resolve({ data: [{ id: 'old-personal', is_personal: true }], current_id: 'old-personal' })
  await pending
  assert.equal(f.workspace.ready.value, false)
  assert.equal(f.workspace.current.value, null)
  assert.equal(f.scopes.at(-1), null)
})

function fixture(fetch) {
  const context = vm.createContext({ exports: {}, fetch, Headers, AbortController, DOMException })
  const code = ts.transpileModule(source.replaceAll('import.meta.client', 'true'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText
  vm.runInContext(code, context)
  return context.exports
}

function deferred() {
  let resolve
  const promise = new Promise(done => { resolve = done })
  return { promise, resolve }
}

const response = (data = {}) => new Response(JSON.stringify(data), { status: 200 })

test('workspace requests require selection and send scope only to keys and usage', async () => {
  const calls = []
  const client = fixture(async (path, init) => {
    calls.push({ path, init })
    return response({ data: [] })
  })
  await assert.rejects(client.api('/account/keys'), { name: 'AbortError' })
  assert.equal(calls.length, 0)
  client.setWorkspaceRequestScope('space-a')
  for (const path of ['/account/keys', '/account/keys/key/quota', '/account/usage?days=30', '/account/usage/daily']) {
    await client.api(path)
    assert.equal(calls.at(-1).init.headers.get('X-Workspace-ID'), 'space-a')
  }
  for (const path of ['/account/me', '/account/workspaces', '/account/payments', '/admin/channels']) {
    await client.api(path, { headers: { 'X-Workspace-ID': 'injected' } })
    assert.equal(calls.at(-1).init.headers.has('X-Workspace-ID'), false)
  }
})

test('switching cancels pending calls and rejects late responses', async () => {
  const gate = deferred()
  let signal
  const client = fixture((_path, init) => { signal = init.signal; return gate.promise })
  client.setWorkspaceRequestScope('space-a')
  const pending = client.api('/account/keys')
  const rejected = assert.rejects(pending, { name: 'AbortError' })
  client.setWorkspaceRequestScope('space-b')
  assert.equal(signal.aborted, true)
  gate.resolve(response({ data: [{ id: 'old-space-secret' }] }))
  await rejected
})

test('a body arriving after switching is discarded even when headers arrived first', async () => {
  const body = deferred()
  const started = deferred()
  const client = fixture(async () => ({ ok: true, status: 200, json: () => { started.resolve(); return body.promise } }))
  client.setWorkspaceRequestScope('space-a')
  const pending = client.api('/account/keys')
  const rejected = assert.rejects(pending, { name: 'AbortError' })
  await started.promise
  client.setWorkspaceRequestScope('space-b')
  body.resolve({ data: [{ id: 'old-space-secret' }] })
  await rejected
})

test('a protected mutation cannot replay in another workspace after reauthentication', async () => {
  const auth = deferred()
  const started = deferred()
  let calls = 0
  const client = fixture(async () => {
    calls += 1
    return new Response(JSON.stringify({ error: { code: 'reauthentication_required' } }), { status: 403 })
  })
  client.setReauthenticationHandler(() => { started.resolve(); return auth.promise })
  client.setWorkspaceRequestScope('space-a')
  const pending = client.api('/account/keys/key/revoke', { method: 'POST' })
  const rejected = assert.rejects(pending, { name: 'AbortError' })
  await started.promise
  client.setWorkspaceRequestScope('space-b')
  auth.resolve(true)
  await rejected
  assert.equal(calls, 1)
})

test('signing out clears workspace scope and invalidates in-flight bodies', async () => {
  const body = deferred()
  const started = deferred()
  const client = fixture(async () => ({ ok: true, status: 200, json: () => { started.resolve(); return body.promise } }))
  client.setWorkspaceRequestScope('space-a')
  const pending = client.api('/account/usage')
  const rejected = assert.rejects(pending, { name: 'AbortError' })
  await started.promise
  client.invalidateSessionRequests()
  body.resolve({ data: [] })
  await rejected
  await assert.rejects(client.api('/account/keys'), { name: 'AbortError' })
})
