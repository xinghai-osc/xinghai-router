export default defineEventHandler((event) => {
  const url = getRequestURL(event)
  const upstream = process.env.API_INTERNAL_URL || 'http://beta.platform.ai.hixinghai.top/api'
  return proxyRequest(event, `${upstream}${url.pathname.replace(/^\/api/, '')}${url.search}`)
})
