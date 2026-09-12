import { normalizeOrigin, resolveSiteOrigin } from '~/utils/site-url'

export default defineEventHandler((event) => {
  const requestUrl = getRequestURL(event)
  const configured = String(useRuntimeConfig(event).public.siteUrl || '')
  const siteUrl = resolveSiteOrigin(configured, requestUrl.origin)
  setResponseHeader(event, 'Content-Type', 'text/plain; charset=utf-8')
  if (normalizeOrigin(configured)) {
    setResponseHeader(event, 'Cache-Control', 'public, max-age=3600')
  } else {
    setResponseHeader(event, 'Cache-Control', 'no-store')
    setResponseHeader(event, 'Vary', 'Host')
  }
  return [
    'User-agent: *',
    'Disallow: /console',
    'Disallow: /api/',
    `Sitemap: ${siteUrl}/sitemap.xml`,
    '',
  ].join('\n')
})
