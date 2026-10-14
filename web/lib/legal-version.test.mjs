import assert from 'node:assert/strict'
import { test } from 'node:test'
import { legalVersion } from './legal-version.mjs'

test('legal changes in any locale invalidate prior consent', () => {
  const page = "t('site.termsTitle') t('site.termsS1P1')"
  const messages = { zh: { termsTitle: '协议', termsS1P1: '原文' }, en: { termsTitle: 'Terms', termsS1P1: 'Original' } }
  const initial = legalVersion(page, messages)
  assert.match(initial, /^[a-f0-9]{64}$/)
  assert.notEqual(initial, legalVersion(page, { ...messages, en: { ...messages.en, termsS1P1: 'Updated' } }))
  assert.notEqual(initial, legalVersion(`${page} t('site.termsS2P1')`, messages))
})

test('unrelated copy, locale order and formatting do not invalidate consent', () => {
  const page = "t('site.privacyTitle')"
  const messages = { zh: { privacyTitle: '隐私', heroTitle: '旧' }, en: { privacyTitle: 'Privacy' } }
  const initial = legalVersion(page, messages)
  assert.equal(initial, legalVersion(`\n  ${page}\n`, { en: messages.en, zh: { ...messages.zh, heroTitle: '新' } }))
})

test('privacy disclosures referenced outside privacy-prefixed keys affect the version', () => {
  const page = "['site.privacyS1P1', 'site.riskProbePrivacy']"
  const messages = { zh: { privacyS1P1: 'Data', riskProbePrivacy: 'Original' } }
  assert.notEqual(legalVersion(page, messages), legalVersion(page, { zh: { ...messages.zh, riskProbePrivacy: 'Updated' } }))
})
