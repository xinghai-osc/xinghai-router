import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const collectorSource = await readFile(new URL('../src/risk-context.ts', import.meta.url), 'utf8')
const composableSource = await readFile(new URL('../composables/useRiskContext.ts', import.meta.url), 'utf8')
const settings = { enabled: true, stun_urls: [], timeout_ms: 2000, notice_version: '1' }
const plain = value => JSON.parse(JSON.stringify(value))

function evaluate(source, globals = {}) {
  const context = vm.createContext({ exports: {}, URL, ...globals })
  const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  vm.runInContext(code, context)
  return context.exports
}
function fixture(behavior = {}) {
  const timers = new Map()
  const peers = []
  let timeoutMs
  class Peer {
    constructor(config) {
      if (behavior.constructorError) throw new Error('constructor failed')
      this.config = config
      this.closed = 0
      this.localDescriptions = 0
      peers.push(this)
    }
    createDataChannel(label) {
      this.label = label
      if (behavior.channelError) throw new Error('channel failed')
    }
    createOffer() {
      if (behavior.offerError) return Promise.reject(new Error('offer failed'))
      return behavior.offer ?? Promise.resolve({ type: 'offer', sdp: 'must never be submitted' })
    }
    async setLocalDescription() {
      this.localDescriptions++
      if (behavior.descriptionError) throw new Error('description failed')
      for (const candidate of behavior.candidates ?? []) this.onicecandidate?.({ candidate })
      if (behavior.complete) this.onicecandidate?.({ candidate: null })
      if (behavior.gatheringComplete) {
        this.iceGatheringState = 'complete'
        this.onicegatheringstatechange?.()
      }
    }
    close() { this.closed++ }
  }
  const collector = evaluate(collectorSource, {
    window: { RTCPeerConnection: Peer },
    setTimeout: (fn, delay) => { timeoutMs = delay; timers.set(1, fn); return 1 },
    clearTimeout: id => timers.delete(id),
  })
  return { ...collector, peers, timers, timeout: () => timers.get(1)?.(), delay: () => timeoutMs }
}
const candidate = (address, type = 'srflx') => ({ address, type })

test('only public server-reflexive addresses are retained and normalized', () => {
  const { publicSrflxAddress } = fixture()
  for (const value of ['0.1.2.3', '10.1.2.3', '100.64.1.2', '127.0.0.1', '169.254.1.2', '172.16.1.2', '172.31.1.2', '192.168.1.2', '192.0.2.1', '198.18.0.1', '198.51.100.1', '203.0.113.1', '224.0.0.1', '255.255.255.255', '256.1.1.1', '01.2.3.4', 'host.local', '8.8.8.8:3478', '8.8.8.8\n', '::1', 'fc00::1', 'fd00::1', 'fe80::1', 'ff02::1', '::ffff:8.8.8.8', '2001:db8::1', '2002:a00:1::1', '3fff::1', '2001:4860:::1', '[2606:4700::1]']) {
    assert.equal(publicSrflxAddress(candidate(value)), null, value)
  }
  for (const type of ['host', 'relay', 'prflx', null]) assert.equal(publicSrflxAddress(candidate('8.8.8.8', type)), null)
  assert.equal(publicSrflxAddress(candidate('8.8.8.8')), '8.8.8.8')
  assert.equal(publicSrflxAddress(candidate('2606:4700:0000:0000:0000:0000:0000:1111')), '2606:4700::1111')
})

test('disabled and unsupported probes do not instantiate browser connections', async () => {
  const f = fixture()
  assert.deepEqual(plain(await f.collectRiskProbe()), { status: 'disabled', addresses: [] })
  assert.equal(f.peers.length, 0)
  const server = evaluate(collectorSource)
  assert.deepEqual(plain(await server.collectRiskProbe(settings)), { status: 'unsupported', addresses: [] })
})

test('collection uses a data channel, deduplicates, caps eight addresses and always closes', async () => {
  const f = fixture({ complete: true, candidates: [candidate('10.1.2.3'), candidate('1.1.1.1', 'relay'), ...Array.from({ length: 12 }, (_, i) => candidate(`8.8.8.${i + 1}`)), candidate('8.8.8.1')] })
  const result = await f.collectRiskProbe(settings)
  assert.deepEqual(plain(result), { status: 'completed', addresses: Array.from({ length: 8 }, (_, i) => `8.8.8.${i + 1}`) })
  assert.deepEqual(plain(f.peers[0].config), { iceServers: [{ urls: ['stun:stun.cloudflare.com:3478'] }], iceCandidatePoolSize: 0 })
  assert.equal(f.peers[0].label, 'risk-context')
  assert.equal(f.peers[0].closed, 1)
  assert.equal(f.peers[0].onicecandidate, null)
  assert.equal(f.timers.size, 0)
})

test('ICE gathering completion handles empty evidence and canonical IPv6 duplicates', async () => {
  const empty = fixture({ gatheringComplete: true })
  assert.deepEqual(plain(await empty.collectRiskProbe(settings)), { status: 'completed', addresses: [] })
  const f = fixture({ complete: true, candidates: [candidate('2606:4700::1111'), candidate('2606:4700:0:0:0:0:0:1111')] })
  assert.deepEqual(plain(await f.collectRiskProbe(settings)), { status: 'completed', addresses: ['2606:4700::1111'] })
})

test('the two-second deadline includes a hanging offer and prevents late browser work', async () => {
  let resolveOffer
  const f = fixture({ offer: new Promise(resolve => { resolveOffer = resolve }) })
  const task = f.collectRiskProbe({ ...settings, timeout_ms: 999999 })
  assert.equal(f.delay(), 2000)
  f.timeout()
  assert.deepEqual(plain(await task), { status: 'timeout', addresses: [] })
  assert.equal(f.peers[0].closed, 1)
  resolveOffer({ type: 'offer' })
  await Promise.resolve()
  assert.equal(f.peers[0].localDescriptions, 0)
  assert.equal(f.timers.size, 0)
})

test('timeouts retain only already observed public addresses', async () => {
  const f = fixture({ candidates: [candidate('1.1.1.1'), candidate('host.local')] })
  const task = f.collectRiskProbe({ ...settings, timeout_ms: 25 })
  await Promise.resolve()
  assert.equal(f.delay(), 25)
  f.timeout()
  assert.deepEqual(plain(await task), { status: 'timeout', addresses: ['1.1.1.1'] })
  assert.equal(f.peers[0].closed, 1)
})

test('browser failures report error and close every constructed connection', async () => {
  for (const key of ['constructorError', 'channelError', 'offerError', 'descriptionError']) {
    const f = fixture({ [key]: true })
    assert.deepEqual(plain(await f.collectRiskProbe(settings)), { status: 'error', addresses: [] }, key)
    assert.equal(f.peers[0]?.closed ?? 1, 1)
    assert.equal(f.timers.size, 0)
  }
})

test('only STUN URLs enter browser configuration', async () => {
  const f = fixture({ complete: true })
  const result = await f.collectRiskProbe({ ...settings, stun_urls: ['turn:relay.example:3478', 'https://example.com'] })
  assert.equal(result.status, 'error')
  assert.equal(f.peers.length, 0)
  assert.equal(f.validStunUrl('stun:stun.cloudflare.com:3478'), true)
  assert.equal(f.validStunUrl('stuns:stun.example.com:5349'), false)
  for (const value of ['stun:example.com', 'stun:example.com:0', 'stun:example.com:65536', 'stun:user@example.com:3478']) assert.equal(f.validStunUrl(value), false)
})

function contextFixture(client = true, fail = '') {
  const calls = []
  const endpoints = {
    getSiteSettings: async () => { calls.push('settings'); if (fail === 'settings') throw new Error('settings failed'); return { risk_probe: settings } },
    createRiskContext: async (body) => { calls.push(plain(body)); if (fail === 'submit') throw new Error('evidence failed'); return { id: 'context-1', expires_at: '2030-01-01' } },
  }
  const module = evaluate(composableSource.replaceAll('import.meta.client', String(client)), {
    require: path => path === '~/src/api' ? { endpoints } : { collectRiskProbe: async () => ({ status: 'unsupported', addresses: [] }) },
  })
  return { calls, ...module.useRiskContext() }
}

test('the composable never collects on creation or on SSR', async () => {
  const f = contextFixture(false)
  assert.equal(f.calls.length, 0)
  assert.equal(await f.prepare('register'), undefined)
  assert.equal(f.calls.length, 0)
})

test('unsupported browsers still submit truthful context for the deliberate purpose', async () => {
  const f = contextFixture()
  assert.equal(f.calls.length, 0)
  assert.equal(await f.prepare('oauth'), 'context-1')
  assert.deepEqual(f.calls, ['settings', { purpose: 'oauth', status: 'unsupported', addresses: [] }])
})

test('server submission and settings failures propagate without pretending evidence was accepted', async () => {
  for (const failure of ['settings', 'submit']) {
    const f = contextFixture(true, failure)
    await assert.rejects(f.prepare('checkin'), /failed/)
    assert.equal(f.calls.length, failure === 'settings' ? 1 : 2)
  }
})
