import { BlockList, isIP } from 'node:net'
import type { IncomingHttpHeaders } from 'node:http'

function normalizeAddress(value: string): string | null {
  let address = value.trim()
  if (address.startsWith('[') && address.endsWith(']')) address = address.slice(1, -1)
  if (address.includes('%')) return null
  const family = isIP(address)
  if (family === 4) return address
  if (family !== 6) return null
  address = new URL(`http://[${address}]/`).hostname.slice(1, -1)
  const mapped = /^::ffff:([\da-f]+):([\da-f]+)$/.exec(address)
  if (!mapped) return address
  const upper = Number.parseInt(mapped[1]!, 16)
  const lower = Number.parseInt(mapped[2]!, 16)
  return [upper >> 8, upper & 255, lower >> 8, lower & 255].join('.')
}

interface TrustedProxies {
  ipv4: BlockList
  ipv6: BlockList
}

export function parseWebTrustedProxies(value = ''): TrustedProxies {
  const trusted = { ipv4: new BlockList(), ipv6: new BlockList() }
  const aliases: Record<string, string[]> = {
    loopback: ['127.0.0.0/8', '::1/128'],
    private: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', 'fc00::/7'],
  }
  for (const entry of value.split(',').map(part => part.trim()).filter(Boolean)) {
    for (const subnet of aliases[entry.toLowerCase()] ?? [entry]) {
      const parts = subnet.split('/')
      const raw = parts[0]!
      const address = normalizeAddress(raw)
      const family = address ? isIP(address) : 0
      const rawFamily = isIP(raw)
      if (!address || !rawFamily || parts.length > 2) throw new Error(`Invalid WEB_TRUSTED_PROXIES entry: ${entry}`)
      const bits = rawFamily === 4 ? 32 : 128
      let prefix = parts.length === 1 ? bits : /^\d+$/.test(parts[1]!) ? Number(parts[1]) : -1
      if (prefix < 0 || prefix > bits) throw new Error(`Invalid WEB_TRUSTED_PROXIES entry: ${entry}`)
      if (rawFamily === 6 && family === 4) {
        if (prefix < 96) throw new Error(`Invalid WEB_TRUSTED_PROXIES entry: ${entry}`)
        prefix -= 96
      }
      const type = family === 4 ? 'ipv4' : 'ipv6'
      trusted[type].addSubnet(address, prefix, type)
    }
  }
  return trusted
}

function isTrusted(address: string, trusted: TrustedProxies): boolean {
  const type = isIP(address) === 4 ? 'ipv4' : 'ipv6'
  return trusted[type].check(address, type)
}

function headerValue(headers: IncomingHttpHeaders, name: string): string | undefined {
  const values = Object.entries(headers)
    .filter(([key]) => key.toLowerCase() === name)
    .flatMap(([, value]) => value === undefined ? [] : Array.isArray(value) ? value : [value])
  return values.length ? values.join(',') : undefined
}

function forwardedAddress(headers: IncomingHttpHeaders, peer: string, trusted: TrustedProxies): string {
  if (!isTrusted(peer, trusted)) return peer
  const forwarded = headerValue(headers, 'x-forwarded-for')
  if (forwarded !== undefined) {
    const hops = forwarded.split(',')
    if (forwarded.length > 4096 || hops.length > 32) return peer
    let current = peer
    for (let index = hops.length - 1; index >= 0; index--) {
      if (!isTrusted(current, trusted)) return current
      const next = normalizeAddress(hops[index]!)
      if (!next) return peer
      current = next
    }
    return current
  }
  return normalizeAddress(headerValue(headers, 'x-real-ip') ?? '') ?? peer
}

const forwardingAliases = new Set([
  'forwarded', 'x-real-ip', 'client-ip', 'x-client-ip', 'true-client-ip',
  'cf-connecting-ip', 'cf-connecting-ipv6', 'fastly-client-ip', 'x-cluster-client-ip',
  'x-original-forwarded-for', 'x-envoy-external-address', 'x-original-host',
  'x-original-proto', 'x-url-scheme', 'front-end-https',
])

export function sanitizeForwardingHeaders(headers: IncomingHttpHeaders, remoteAddress: string | undefined, encrypted: boolean, trusted: TrustedProxies): void {
  const peer = normalizeAddress((remoteAddress ?? '').split('%')[0]!)
  const address = peer ? forwardedAddress(headers, peer, trusted) : null
  let protocol = encrypted ? 'https' : 'http'
  if (!encrypted && peer && isTrusted(peer, trusted)) {
    const forwardedProtocol = headerValue(headers, 'x-forwarded-proto')?.trim().toLowerCase()
    if (forwardedProtocol === 'http' || forwardedProtocol === 'https') protocol = forwardedProtocol
  }
  for (const key of Object.keys(headers)) {
    const name = key.toLowerCase()
    if (forwardingAliases.has(name) || name.startsWith('x-forwarded-')) Reflect.deleteProperty(headers, key)
  }
  if (address) {
    headers['x-forwarded-for'] = address
    headers['x-real-ip'] = address
  }
  headers['x-forwarded-proto'] = protocol
}
