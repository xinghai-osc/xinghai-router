import type { RiskProbeSettings, RiskProbeStatus } from './api'

export interface RiskProbeResult { status: RiskProbeStatus; addresses: string[] }

export const DEFAULT_STUN_URL = 'stun:stun.cloudflare.com:3478'

export function publicSrflxAddress(candidate: Pick<RTCIceCandidate, 'type' | 'address'>): string | null {
  if (candidate.type !== 'srflx' || !candidate.address) return null
  const address = candidate.address
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(address)) {
    const parts = address.split('.').map(Number)
    if (parts.some((part, index) => part > 255 || String(part) !== address.split('.')[index])) return null
    const [a, b, c] = parts as [number, number, number, number]
    if (a === 0 || a === 10 || a === 127 || a >= 224
      || (a === 100 && b >= 64 && b <= 127)
      || (a === 169 && b === 254)
      || (a === 172 && b >= 16 && b <= 31)
      || (a === 192 && (b === 168 || b === 0 || (b === 88 && c === 99)))
      || (a === 198 && (b === 18 || b === 19 || (b === 51 && c === 100)))
      || (a === 203 && b === 0 && c === 113)) return null
    return address
  }
  if (!/^[\da-f:]+$/i.test(address) || !address.includes(':')) return null
  try {
    const normalized = new URL(`http://[${address}]/`).hostname.slice(1, -1)
    const [first, second] = normalized.split(':').map(part => Number.parseInt(part || '0', 16))
    if (first === undefined || first < 0x2000 || first > 0x3fff
      || (first === 0x2001 && (second === undefined || second <= 0x1ff || second === 0xdb8))
      || first === 0x2002 || (first === 0x3fff && (second ?? 0) <= 0xfff)) return null
    return normalized
  } catch {
    return null
  }
}

export function validStunUrl(value: string): boolean {
  if (!value.startsWith('stun:')) return false
  const match = /^stun:(?:[a-z\d](?:[a-z\d.-]*[a-z\d])?|\[[\da-f:]+\]):(\d{1,5})$/i.exec(value)
  return Boolean(match && Number(match[1]) >= 1 && Number(match[1]) <= 65535)
}

export async function collectRiskProbe(
  settings?: RiskProbeSettings,
  PeerConnection: typeof RTCPeerConnection | undefined = typeof window === 'undefined' ? undefined : window.RTCPeerConnection,
): Promise<RiskProbeResult> {
  if (!settings?.enabled) return { status: 'disabled', addresses: [] }
  if (!PeerConnection) return { status: 'unsupported', addresses: [] }
  const addresses = new Set<string>()
  let peer: RTCPeerConnection | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let finished = false
  try {
    const status = await new Promise<RiskProbeStatus>((resolve) => {
      const finish = (value: RiskProbeStatus) => {
        if (finished) return
        finished = true
        resolve(value)
      }
      const timeout = Number.isFinite(settings.timeout_ms) ? Math.max(1, Math.min(2000, settings.timeout_ms)) : 2000
      timer = setTimeout(() => finish('timeout'), timeout)
      const urls = settings.stun_urls?.length ? settings.stun_urls.filter(validStunUrl) : [DEFAULT_STUN_URL]
      if (!urls.length) { finish('error'); return }
      try {
        peer = new PeerConnection({ iceServers: [{ urls }], iceCandidatePoolSize: 0 })
        peer.onicecandidate = (event) => {
          if (finished) return
          if (!event.candidate) { finish('completed'); return }
          const address = publicSrflxAddress(event.candidate)
          if (address && addresses.size < 8) addresses.add(address)
        }
        peer.onicegatheringstatechange = () => {
          if (peer?.iceGatheringState === 'complete') finish('completed')
        }
        peer.createDataChannel('risk-context')
        const connection = peer
        void connection.createOffer()
          .then(offer => finished ? undefined : connection.setLocalDescription(offer))
          .catch(() => finish('error'))
      } catch {
        finish('error')
      }
    })
    return { status, addresses: [...addresses] }
  } finally {
    finished = true
    if (timer !== undefined) clearTimeout(timer)
    if (peer) {
      peer.onicecandidate = null
      peer.onicegatheringstatechange = null
      try { peer.close() } catch { peer = undefined }
    }
  }
}
