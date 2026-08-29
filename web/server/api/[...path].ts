export default defineEventHandler((event) => {
  const url = getRequestURL(event)
  const upstream = process.env.API_INTERNAL_URL
  if (!upstream) {
    throw createError({ statusCode: 500, statusMessage: 'API_INTERNAL_URL is not configured' })
  }
  const target = `${upstream.replace(/\/+$/, '')}${url.pathname.replace(/^\/api/, '')}${url.search}`
  return proxyRequest(event, target)
})
