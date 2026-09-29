import assert from 'node:assert/strict'
import { test } from 'node:test'
import { parseWebTrustedProxies, sanitizeForwardingHeaders } from '../server/utils/proxy-headers.ts'

const cases = [
  { name: 'untrusted direct peer ignores claimed IP', remote: '198.51.100.2', trusted: 'loopback', xff: '203.0.113.1', real: '203.0.113.2', want: '198.51.100.2' },
  { name: 'empty trust ignores all forwarded headers', remote: '127.0.0.1', xff: '203.0.113.1', want: '127.0.0.1' },
  { name: 'spoofed left prefix stops at actual peer', remote: '127.0.0.1', trusted: 'loopback', xff: '203.0.113.1, 198.51.100.2', want: '198.51.100.2' },
  { name: 'untrusted intermediary is the boundary', remote: '127.0.0.1', trusted: 'loopback', xff: '203.0.113.1, 10.0.0.2', want: '10.0.0.2' },
  { name: 'walks through explicitly trusted intermediaries', remote: '127.0.0.1', trusted: 'loopback,10.0.0.2', xff: '203.0.113.1, 198.51.100.2, 10.0.0.2', want: '198.51.100.2' },
  { name: 'X-Real-IP cannot override XFF', remote: '127.0.0.1', trusted: 'loopback', xff: '198.51.100.2', real: '203.0.113.1', want: '198.51.100.2' },
  { name: 'malformed rightmost hop fails closed', remote: '127.0.0.1', trusted: 'loopback', xff: '198.51.100.2, bad', real: '203.0.113.1', want: '127.0.0.1' },
  { name: 'empty XFF prevents fallback', remote: '127.0.0.1', trusted: 'loopback', xff: '', real: '203.0.113.1', want: '127.0.0.1' },
  { name: 'invalid prefix beyond the boundary is irrelevant', remote: '127.0.0.1', trusted: 'loopback', xff: 'bad, 198.51.100.2', want: '198.51.100.2' },
  { name: 'repeated header fields preserve chain order', remote: '127.0.0.1', trusted: 'loopback,10.0.0.2', xff: ['203.0.113.1, 198.51.100.2', '10.0.0.2'], want: '198.51.100.2' },
  { name: 'X-Real-IP fallback requires trusted peer', remote: '127.0.0.1', trusted: 'loopback', real: '198.51.100.2', want: '198.51.100.2' },
  { name: 'duplicate real IP is rejected', remote: '127.0.0.1', trusted: 'loopback', real: ['198.51.100.2', '203.0.113.1'], want: '127.0.0.1' },
  { name: 'mapped IPv4 socket and candidates normalize', remote: '::ffff:127.0.0.1', trusted: 'loopback', xff: '::ffff:c633:6402', want: '198.51.100.2' },
  { name: 'mapped IPv4 trusted subnet normalizes', remote: '10.0.0.2', trusted: '::ffff:10.0.0.0/104', xff: '198.51.100.2', want: '198.51.100.2' },
  { name: 'mapped IPv4 trusted address normalizes', remote: '10.0.0.2', trusted: '::ffff:10.0.0.2', xff: '198.51.100.2', want: '198.51.100.2' },
  { name: 'IPv6 trust does not implicitly include IPv4 peers', remote: '198.51.100.2', trusted: '::/0', xff: '203.0.113.1', want: '198.51.100.2' },
  { name: 'mapped peers require IPv4 trust', remote: '::ffff:198.51.100.2', trusted: '::/0', xff: '203.0.113.1', want: '198.51.100.2' },
  { name: 'IPv6 chains support brackets and CIDRs', remote: '::1', trusted: '::1,2001:db8:1::/48', xff: '2001:db8:2::9, [2001:db8:1::2]', want: '2001:db8:2::9' },
  { name: 'IPv6 sockets normalize canonically', remote: '2001:0DB8:0002::9', want: '2001:db8:2::9' },
  { name: 'header scoped addresses are rejected', remote: '127.0.0.1', trusted: 'loopback', xff: 'fe80::1%eth0', want: '127.0.0.1' },
  { name: 'header addresses with ports are rejected', remote: '127.0.0.1', trusted: 'loopback', xff: '198.51.100.2:80', want: '127.0.0.1' },
  { name: 'oversized chain fails closed', remote: '127.0.0.1', trusted: 'loopback', xff: ['198.51.100.2', ...Array(32).fill('127.0.0.1')].join(','), want: '127.0.0.1' },
]

for (const entry of cases) {
  test(entry.name, () => {
    const headers = {}
    if (entry.xff !== undefined) headers['x-forwarded-for'] = entry.xff
    if (entry.real !== undefined) headers['x-real-ip'] = entry.real
    sanitizeForwardingHeaders(headers, entry.remote, false, parseWebTrustedProxies(entry.trusted))
    assert.equal(headers['x-forwarded-for'], entry.want)
    assert.equal(headers['x-real-ip'], entry.want)
  })
}

test('sanitization removes forwarding aliases while preserving host, cookies and CSRF headers', () => {
  const preserved = { host: 'router.example', cookie: 'xinghai.session=fake; xinghai.oauth.state=fake-state', origin: 'https://router.example', 'sec-fetch-site': 'same-origin', 'x-xinghai-request': '1', 'content-type': 'application/json' }
  const headers = {
    ...preserved,
    Forwarded: 'for=203.0.113.1;proto=https;host=attacker.example',
    'X-Forwarded-Host': 'attacker.example',
    'X-Forwarded-For': '203.0.113.1',
    'X-Forwarded-Port': '443',
    'x-forwarded-proto': 'https',
    'x-original-forwarded-for': '203.0.113.1',
    'x-real-ip': '203.0.113.1',
    'cf-connecting-ip': '203.0.113.1',
    'true-client-ip': '203.0.113.1',
    'x-envoy-external-address': '203.0.113.1',
    'front-end-https': 'on',
  }
  sanitizeForwardingHeaders(headers, '198.51.100.2', false, parseWebTrustedProxies())
  assert.deepEqual(headers, { ...preserved, 'x-forwarded-for': '198.51.100.2', 'x-real-ip': '198.51.100.2', 'x-forwarded-proto': 'http' })
})

test('forwarded protocol is trusted only from explicit peers and must be a single scheme', () => {
  for (const [remote, encrypted, value, want] of [
    ['198.51.100.2', false, 'https', 'http'],
    ['127.0.0.1', false, 'https', 'https'],
    ['127.0.0.1', false, 'http', 'http'],
    ['127.0.0.1', false, 'https,http', 'http'],
    ['127.0.0.1', false, ['https', 'http'], 'http'],
    ['127.0.0.1', false, 'javascript', 'http'],
    ['127.0.0.1', true, 'http', 'https'],
  ]) {
    const headers = { 'x-forwarded-proto': value }
    sanitizeForwardingHeaders(headers, remote, encrypted, parseWebTrustedProxies('loopback'))
    assert.equal(headers['x-forwarded-proto'], want)
  }
})

test('missing socket does not promote a header to a verified address', () => {
  const headers = { 'x-forwarded-for': '198.51.100.2', 'x-real-ip': '198.51.100.2', 'x-forwarded-proto': 'https' }
  sanitizeForwardingHeaders(headers, undefined, false, parseWebTrustedProxies('loopback'))
  assert.deepEqual(headers, { 'x-forwarded-proto': 'http' })
})

test('invalid proxy specifications fail configuration rather than expanding trust', () => {
  for (const value of ['not-an-ip', '10.0.0.0/33', '::1/129', '::ffff:10.0.0.0/80', 'fe80::1%eth0', '198.51.100.2:80', '10.0.0.0/', '10.0.0.0/8/1']) {
    assert.throws(() => parseWebTrustedProxies(value), /Invalid WEB_TRUSTED_PROXIES/)
  }
})
