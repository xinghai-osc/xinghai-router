import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createServer, request as requestHTTP } from 'node:http'
import { createRequire } from 'node:module'
import { test } from 'node:test'
import { pathToFileURL } from 'node:url'
import vm from 'node:vm'
import ts from 'typescript'
import { computed, readonly, ref, watch } from 'vue'

const root = new URL('../', import.meta.url)

async function compile(file, client = true) {
  const source = (await readFile(new URL(file, root), 'utf8')).replaceAll('import.meta.client', String(client))
  const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  return `{\n${output}\n}`
}

async function fixture(client = true) {
  const calls = []
  const redirects = []
  const deleted = []
  const state = new Map()
  const context = vm.createContext({
    exports: {}, Headers, Response, AbortController, computed, readonly, ref, watch,
    useState: (key, initial) => {
      if (!state.has(key)) state.set(key, ref(initial()))
      return state.get(key)
    },
    document: { cookie: 'xinghai.admin-token=fake-legacy-token' },
    localStorage: { removeItem: key => deleted.push(key) },
    sessionStorage: { removeItem: key => deleted.push(key) },
    window: { location: { pathname: '/console/keys', search: '', assign: url => redirects.push(url) } },
    fetch: async (url, init) => {
      calls.push({ url, init })
      return context.respond(url, init)
    },
  })
  vm.runInContext(await compile('src/api.ts', client), context)
  const api = context.exports
  context.exports = {}
  vm.runInContext(await compile('composables/useReauthentication.ts', client), context)
  const challenge = context.exports.useReauthentication()
  api.setReauthenticationHandler(challenge.request)
  return { api, challenge, context, state, calls, redirects, deleted }
}

const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const required = () => json({ error: { code: 'reauthentication_required', message: 'Verify your password' } }, 403)

async function waitUntil(condition) {
  for (let attempt = 0; attempt < 50 && !condition(); attempt++) await new Promise(resolve => setImmediate(resolve))
  assert.ok(condition())
}

test('all requests and binary downloads use cookie credentials and the CSRF header, never Bearer', async () => {
  const f = await fixture()
  f.context.respond = () => json({ expires_at: '2099-01-01T00:00:00Z' })
  await f.api.api('/account/me', { headers: { Authorization: 'Bearer fake-old-token' } })
  await f.api.endpoints.login({ email: 'fake@example.test', password: 'fake-password' })
  await f.api.endpoints.downloadInvoicePDF('fake-invoice')
  for (const { init } of f.calls) {
    assert.equal(init.credentials, 'same-origin')
    assert.equal(init.headers.get('X-Xinghai-Request'), '1')
    assert.equal(init.headers.has('Authorization'), false)
  }
  f.api.clearLegacySession()
  assert.match(f.context.document.cookie, /Max-Age=0/)
  assert.equal(f.deleted.length, 2)
})

test('concurrent sensitive requests share one dialog and retry each original request exactly once', async () => {
  const f = await fixture()
  const counts = new Map()
  f.context.respond = (url) => {
    if (url === '/api/auth/reauthenticate') return json({ expires_at: '2099-01-01T00:00:00Z' })
    counts.set(url, (counts.get(url) ?? 0) + 1)
    return counts.get(url) === 1 ? required() : json({ key: 'fake-revealed-key' })
  }
  const a = f.api.endpoints.revealAccountKey('a')
  const b = f.api.endpoints.revealAccountKey('b')
  await waitUntil(() => f.challenge.open.value)
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(f.challenge.challengeId.value, 1)
  await f.api.endpoints.reauthenticate('fake-password')
  f.challenge.finish(true)
  assert.equal((await a).key, 'fake-revealed-key')
  assert.equal((await b).key, 'fake-revealed-key')
  assert.equal(f.calls.length, 5)
  assert.equal(f.challenge.open.value, false)
})

test('cancel never retries, failed password does not expire the session or close the dialog', async () => {
  const f = await fixture()
  let expired = 0
  f.api.setSessionExpiredHandler(() => expired++)
  f.context.respond = url => url === '/api/auth/reauthenticate'
    ? json({ error: { code: 'invalid_credentials', message: 'Wrong password' } }, 401)
    : required()
  const original = f.api.endpoints.revealAccountKey('a')
  const rejected = assert.rejects(original, error => error.code === 'reauthentication_required')
  await waitUntil(() => f.challenge.open.value)
  await assert.rejects(f.api.endpoints.reauthenticate('fake-wrong-password'), error => error.status === 401)
  assert.equal(expired, 0)
  assert.equal(f.redirects.length, 0)
  assert.equal(f.challenge.open.value, true)
  f.challenge.cancel()
  await rejected
  assert.equal(f.calls.length, 2)
})

test('a second challenge response is not retried recursively', async () => {
  const f = await fixture()
  f.context.respond = required
  f.api.setReauthenticationHandler(() => Promise.resolve(true))
  await assert.rejects(f.api.endpoints.revealAccountKey('a'), error => error.code === 'reauthentication_required')
  assert.equal(f.calls.length, 2)
})

test('session changes and aborted requests prevent sensitive replay', async () => {
  for (const mode of ['session', 'abort']) {
    const f = await fixture()
    const controller = new AbortController()
    f.context.respond = required
    const original = f.api.api('/admin/sensitive', { method: 'POST', body: '{"fake":true}', signal: controller.signal })
    const rejected = assert.rejects(original, error => error.status === 403)
    await waitUntil(() => f.challenge.open.value)
    if (mode === 'session') f.api.invalidateSessionRequests()
    else controller.abort()
    f.challenge.finish(true)
    await rejected
    assert.equal(f.calls.length, 1)
  }
})

test('anonymous me probes and public pages never redirect, protected failures do', async () => {
  const f = await fixture()
  let expired = 0
  f.api.setSessionExpiredHandler(() => expired++)
  f.context.respond = () => json({ error: { code: 'unauthorized', message: 'Expired session' } }, 401)
  await assert.rejects(f.api.endpoints.getAccount())
  assert.equal(f.redirects.length, 0)
  f.context.window.location.pathname = '/activity'
  await assert.rejects(f.api.endpoints.getPublicActivity())
  assert.equal(f.redirects.length, 0)
  f.context.window.location.pathname = '/console/keys'
  await assert.rejects(f.api.endpoints.getAccountKeys())
  assert.equal(f.redirects.length, 1)
  assert.equal(expired, 2)
})

test('rotated cookies do not turn in-flight 401s into logout; only safe reads replay', async () => {
  const f = await fixture()
  let expired = 0
  let completeVerification
  const oldRequests = []
  f.api.setSessionExpiredHandler(() => expired++)
  f.context.respond = url => url === '/api/auth/reauthenticate'
    ? new Promise(resolve => { completeVerification = resolve })
    : new Promise(resolve => oldRequests.push(resolve))
  const read = f.api.endpoints.getAccount()
  const write = f.api.api('/admin/unsafe', { method: 'POST', body: '{}' })
  const rejected = assert.rejects(write, error => error.status === 401)
  const verification = f.api.endpoints.reauthenticate('fake-password')
  for (const resolve of oldRequests) resolve(json({ error: { code: 'unauthorized', message: 'Rotated session' } }, 401))
  await new Promise(resolve => setImmediate(resolve))
  f.context.respond = () => json({ id: 'same-account' })
  completeVerification(json({ expires_at: '2099-01-01T00:00:00Z' }))
  await verification
  assert.equal((await read).id, 'same-account')
  await rejected
  assert.equal(f.calls.filter(call => call.url === '/api/admin/unsafe').length, 1)
  assert.equal(f.calls.filter(call => call.url === '/api/account/me').length, 2)
  assert.equal(expired, 0)
  assert.equal(f.redirects.length, 0)
})

test('SSR reauthentication is inert and allocates no password or session secret state', async () => {
  const f = await fixture(false)
  assert.equal(await f.challenge.request(), false)
  f.api.clearLegacySession()
  assert.deepEqual([...f.state.keys()].sort(), ['reauthentication-id', 'reauthentication-open'])
  assert.equal(f.calls.length, 0)
  assert.equal(f.deleted.length, 0)
})

async function accountFixture(client = true) {
  const f = await fixture(client)
  f.context.require = name => {
    if (name === '~/src/api') return f.api
    if (name === '~/src/marketplace') return { dedupeSquareModels: models => models, toSquareModel: model => model }
    throw new Error(`Unexpected import: ${name}`)
  }
  f.context.useI18n = () => ({ t: key => key })
  f.context.useToast = () => ({ toast: { error: () => {} } })
  f.context.useReauthentication = () => f.challenge
  f.context.navigateTo = async () => {}
  f.context.exports = {}
  vm.runInContext(await compile('composables/useAccount.ts', client), f.context)
  f.context.useAccount = f.context.exports.useAccount
  return { ...f, account: f.context.useAccount() }
}

test('account verification ignores stale me failures and never trusts legacy cookie presence', async () => {
  const f = await accountFixture()
  let failOld
  f.context.respond = () => new Promise(resolve => { failOld = resolve })
  const old = f.account.loadAccount()
  f.context.respond = () => json({ id: 'new-account', permissions: [], role: 'user' })
  await f.account.loadAccount(true)
  failOld(json({ error: { code: 'unauthorized', message: 'Old request' } }, 401))
  await old
  assert.equal(f.account.account.value.id, 'new-account')
  assert.equal(f.account.authenticated.value, true)
  f.context.respond = () => json({ error: { code: 'unauthorized', message: 'Expired' } }, 401)
  await f.account.loadAccount(true)
  assert.equal(f.account.authenticated.value, false)
  assert.equal(f.account.loaded.value, true)
  assert.equal(f.account.error.value, '')
  assert.equal(f.redirects.length, 0)
})

test('SSR account state never loads cookies or session secrets', async () => {
  const f = await accountFixture(false)
  await f.account.loadAccount(true)
  assert.equal(f.calls.length, 0)
  assert.equal(f.account.account.value, null)
  assert.equal(f.account.authenticated.value, false)
  assert.equal(JSON.stringify([...f.state.values()].map(value => value.value)).includes('fake-legacy-token'), false)
})

test('catalog state is separated by verified account identity and rejects stale responses', async () => {
  const f = await accountFixture()
  f.context.exports = {}
  vm.runInContext(await compile('composables/useCatalog.ts'), f.context)
  const catalog = f.context.exports.useCatalog()
  f.context.respond = () => json({ id: 'account-a', permissions: [], role: 'user' })
  await f.account.loadAccount()
  let oldCatalog
  f.context.respond = () => new Promise(resolve => { oldCatalog = resolve })
  const old = catalog.loadCatalog()
  await waitUntil(() => Boolean(oldCatalog))
  f.context.respond = () => json({ id: 'account-b', permissions: [], role: 'user' })
  await f.account.signIn()
  oldCatalog(json({ data: [{ id: 'private-account-a' }], groups: [{ id: 'a' }] }))
  await old
  assert.equal(catalog.models.value.length, 0)
  f.context.respond = () => json({ data: [{ id: 'private-account-b' }], groups: [{ id: 'b' }] })
  await catalog.loadCatalog()
  assert.equal(catalog.models.value[0].id, 'private-account-b')
  assert.equal(catalog.groups.value[0].id, 'b')
})

test('Nuxt proxy preserves original Host, Cookie, CSRF headers and all Set-Cookie headers on redirects', async () => {
  const require = createRequire(import.meta.resolve('nuxt'))
  const h3 = await import(pathToFileURL(require.resolve('h3')).href)
  let seen
  let followed = false
  let streamedBytes = 0
  let uploadClosed = false
  let uploadStarted = false
  const upstream = createServer((req, res) => {
    if (req.url === '/upload') {
      req.on('data', (chunk) => {
        streamedBytes += chunk.length
        if (!res.headersSent) res.writeHead(413, { 'Content-Type': 'application/json' }).end('{"error":"too_large"}')
      })
      return
    }
    if (req.url === '/cancel') {
      uploadStarted = true
      req.on('data', () => {})
      req.once('close', () => { uploadClosed = true })
      return
    }
    if (req.url === '/followed') followed = true
    seen = req.headers
    res.writeHead(302, {
      Location: '/followed',
      'Set-Cookie': [
        'xinghai.session=fake-cookie-secret; Path=/; HttpOnly; Secure; SameSite=Lax',
        'xinghai.admin-token=; Path=/; Max-Age=0; SameSite=Strict',
      ],
    })
    res.end()
  })
  await new Promise(resolve => upstream.listen(0, '127.0.0.1', resolve))
  const address = upstream.address()
  const context = vm.createContext({ ...h3, exports: {}, require, Request, Response, Headers, URL, Buffer, AbortController, process: { env: { API_INTERNAL_URL: `http://127.0.0.1:${address.port}` } } })
  vm.runInContext(await compile('server/api/[...path].ts'), context)
  const proxy = createServer(h3.toNodeListener(h3.createApp().use(context.exports.default)))
  await new Promise(resolve => proxy.listen(0, '127.0.0.1', resolve))
  try {
    const origin = `http://127.0.0.1:${proxy.address().port}`
    const response = await fetch(`${origin}/api/auth/callback`, {
      method: 'POST', redirect: 'manual',
      headers: { Origin: origin, Cookie: 'xinghai.session=fake-cookie-secret', 'X-Xinghai-Request': '1', 'X-Forwarded-Host': 'spoofed.example.test' },
    })
    assert.equal(seen.host, new URL(origin).host)
    assert.equal(seen.origin, origin)
    assert.equal(seen.cookie, 'xinghai.session=fake-cookie-secret')
    assert.equal(seen['x-xinghai-request'], '1')
    assert.equal(response.status, 302)
    assert.equal(response.headers.get('location'), '/followed')
    assert.equal(response.headers.getSetCookie().length, 2)
    assert.match(response.headers.getSetCookie()[0], /HttpOnly; Secure; SameSite=Lax/)
    assert.equal(followed, false)

    await new Promise((resolve, reject) => {
      const upload = requestHTTP(`${origin}/api/upload`, { method: 'POST' }, (response) => {
        assert.equal(response.statusCode, 413)
        response.resume()
        response.once('end', () => { upload.destroy(); resolve() })
      })
      upload.once('error', reject)
      upload.setTimeout(2000, () => upload.destroy(new Error('Proxy buffered the unfinished upload')))
      upload.write(Buffer.alloc(4 * 1024 * 1024))
    })
    assert.ok(streamedBytes > 0)

    const upload = requestHTTP(`${origin}/api/cancel`, { method: 'POST' })
    upload.on('error', () => {})
    upload.write('fake-incomplete-body')
    await waitUntil(() => uploadStarted)
    upload.destroy()
    await waitUntil(() => uploadClosed)
  } finally {
    proxy.closeAllConnections()
    upstream.closeAllConnections()
    await Promise.all([new Promise(resolve => proxy.close(resolve)), new Promise(resolve => upstream.close(resolve))])
  }
})
