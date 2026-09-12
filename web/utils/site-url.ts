export function normalizeOrigin(value: string): string | null {
  const candidate = value.trim()
  if (!candidate) return null
  try {
    const url = new URL(candidate)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
    if (url.username || url.password || url.pathname !== '/' || url.search || url.hash) return null
    return url.origin
  } catch {
    return null
  }
}

export function resolveSiteOrigin(configured: string, fallback: string): string {
  return normalizeOrigin(configured) || normalizeOrigin(fallback) || 'http://localhost'
}
