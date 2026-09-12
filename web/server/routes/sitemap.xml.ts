import { normalizeOrigin, resolveSiteOrigin } from '~/utils/site-url'

const PUBLIC_ROUTES = ['/', '/models', '/rankings', '/pricing', '/terms', '/privacy']

function escapeXml(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&apos;')
}

export default defineEventHandler((event) => {
  const requestUrl = getRequestURL(event)
  const configured = String(useRuntimeConfig(event).public.siteUrl || '')
  const siteUrl = resolveSiteOrigin(configured, requestUrl.origin)
  const urls = PUBLIC_ROUTES.map((path) => {
    const loc = escapeXml(new URL(path, `${siteUrl}/`).toString())
    return `  <url><loc>${loc}</loc></url>`
  }).join('\n')

  setResponseHeader(event, 'Content-Type', 'application/xml; charset=utf-8')
  if (normalizeOrigin(configured)) {
    setResponseHeader(event, 'Cache-Control', 'public, max-age=3600')
  } else {
    setResponseHeader(event, 'Cache-Control', 'no-store')
    setResponseHeader(event, 'Vary', 'Host')
  }
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`
})
