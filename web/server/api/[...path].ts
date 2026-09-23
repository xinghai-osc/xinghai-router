import { request as requestHTTP } from 'node:http'
import { request as requestHTTPS } from 'node:https'
import { Readable } from 'node:stream'

const fetchUpstream: typeof fetch = async (input, init) => {
  const request = new Request(input, init)
  const url = new URL(request.url)
  const body = request.body ? Readable.fromWeb(request.body as Parameters<typeof Readable.fromWeb>[0]) : null
  return new Promise<Response>((resolve, reject) => {
    const upstream = (url.protocol === 'https:' ? requestHTTPS : requestHTTP)(url, {
      method: request.method,
      headers: Object.fromEntries(request.headers),
      signal: request.signal,
    }, (response) => {
      const headers = new Headers()
      for (let index = 0; index < response.rawHeaders.length; index += 2) {
        headers.append(response.rawHeaders[index]!, response.rawHeaders[index + 1]!)
      }
      const status = response.statusCode ?? 502
      const empty = request.method === 'HEAD' || [204, 205, 304].includes(status)
      if (empty) response.resume()
      resolve(new Response(empty ? null : Readable.toWeb(response) as ReadableStream<Uint8Array>, {
        status,
        statusText: response.statusMessage,
        headers,
      }))
    })
    upstream.on('error', (error) => {
      body?.destroy()
      reject(error)
    })
    if (body) {
      body.on('error', error => upstream.destroy(error))
      body.pipe(upstream)
    } else {
      upstream.end()
    }
  })
}

export default defineEventHandler(async (event) => {
  const url = getRequestURL(event)
  const upstream = process.env.API_INTERNAL_URL
  if (!upstream) {
    throw createError({ statusCode: 500, statusMessage: 'API_INTERNAL_URL is not configured' })
  }
  const target = `${upstream.replace(/\/+$/, '')}${url.pathname.replace(/^\/api/, '')}${url.search}`
  const host = getRequestHeader(event, 'host')
  const controller = new AbortController()
  const abort = () => controller.abort()
  const close = () => { if (!event.node.res.writableEnded) abort() }
  event.node.req.once('aborted', abort)
  event.node.res.once('close', close)
  if (['PATCH', 'POST', 'PUT', 'DELETE'].includes(event.method)) {
    event._requestBody = Readable.toWeb(event.node.req, { strategy: { highWaterMark: 64 * 1024, size: chunk => chunk.length } }) as ReadableStream<Uint8Array>
  }
  try {
    return await proxyRequest(event, target, {
      fetch: fetchUpstream,
      streamRequest: true,
      headers: host ? { host } : undefined,
      fetchOptions: { redirect: 'manual', signal: controller.signal },
    })
  } finally {
    controller.abort()
    event.node.req.off('aborted', abort)
    event.node.res.off('close', close)
  }
})
