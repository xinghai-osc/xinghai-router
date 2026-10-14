import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { computed, ref } from 'vue'

const versions = { terms_version: 'a'.repeat(64), privacy_version: 'b'.repeat(64) }
const source = ts.transpileModule(readFileSync(new URL('../composables/useLegalConsent.ts', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText

function setup({ user = null, stored = null, endpoints = {} } = {}) {
  const account = { account: ref(user), loaded: ref(true), error: ref(''), sessionScope: ref(user ? '1:user' : '0:anonymous'), loadAccount: async () => {} }
  const states = new Map()
  const storage = new Map(stored ? [['xinghai.legal-consent', JSON.stringify(stored)]] : [])
  const exports = {}
  runInNewContext(source, {
    exports,
    require: () => ({ endpoints }),
    computed,
    useState: (key, initial) => {
      if (!states.has(key)) states.set(key, ref(initial()))
      return states.get(key)
    },
    useAccount: () => account,
    useRuntimeConfig: () => ({ public: { legalVersions: versions } }),
    localStorage: { getItem: key => storage.get(key), setItem: (key, value) => storage.set(key, value) },
  })
  return { consent: exports.useLegalConsent(), account, storage }
}

test('visitors must accept and matching stored consent allows the next visit', async () => {
  const { consent, storage } = setup()
  await consent.check()
  assert.equal(consent.accepted.value, false)
  await consent.accept()
  assert.equal(consent.accepted.value, true)
  const next = setup({ stored: JSON.parse(storage.get('xinghai.legal-consent')) })
  await next.consent.check()
  assert.equal(next.consent.accepted.value, true)
})

test('changing either legal version requires new consent', async () => {
  for (const field of ['terms_version', 'privacy_version']) {
    const { consent } = setup({ stored: { ...versions, [field]: 'c'.repeat(64) } })
    await consent.check()
    assert.equal(consent.ready.value, true)
    assert.equal(consent.accepted.value, false)
  }
})

test('signed-in users use account consent instead of visitor storage', async () => {
  const { consent } = setup({ user: { id: 'user' }, stored: versions, endpoints: { getLegalConsent: async () => ({ terms_version: '', privacy_version: '' }) } })
  await consent.check()
  assert.equal(consent.accepted.value, false)
})

test('failed saves keep the website blocked and permit a retry', async () => {
  let fail = true
  const { consent } = setup({ user: { id: 'user' }, endpoints: {
    getLegalConsent: async () => ({}),
    acceptLegalConsent: async () => { if (fail) throw new Error('offline') },
  } })
  await consent.check()
  await consent.accept()
  assert.equal(consent.failed.value, true)
  assert.equal(consent.accepted.value, false)
  fail = false
  await consent.accept()
  assert.equal(consent.accepted.value, true)
})

test('a save for an old session cannot consent for a different account', async () => {
  let finish
  const { consent, account } = setup({ user: { id: 'user' }, endpoints: {
    getLegalConsent: async () => ({}),
    acceptLegalConsent: () => new Promise(resolve => { finish = resolve }),
  } })
  await consent.check()
  const pending = consent.accept()
  account.sessionScope.value = '2:other'
  finish()
  await pending
  assert.equal(consent.accepted.value, false)
})

test('failed account consent reads never fall back to visitor acceptance', async () => {
  const { consent } = setup({ user: { id: 'user' }, stored: versions, endpoints: { getLegalConsent: async () => { throw new Error('offline') } } })
  await consent.check()
  assert.equal(consent.failed.value, true)
  assert.equal(consent.ready.value, false)
  await consent.accept()
  assert.equal(consent.accepted.value, false)
})
