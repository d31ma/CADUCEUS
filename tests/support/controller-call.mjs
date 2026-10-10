import { TestServer } from './TestServer.mjs'

/**
 * Calls one route over HTTP on the running test server and hands back what
 * it answered: the parsed JSON body, with the HTTP status and headers as
 * non-enumerable `status` and `headers` when the body has no such fields, or
 * the Response itself when the body is not JSON.
 *
 * @param {{ path: string }} route  an entry of Routes
 * @param {string} verb
 * @param {{ paths?: Record<string, any>, query?: Record<string, any>, body?: any, headers?: Record<string, string>, context?: { bearer?: { token?: string }, ipAddress?: string } }} [input]
 */
export async function call(route, verb, input = {}) {
  const response = await respond(route, verb, input)
  if (response.status === 204) return undefined
  if (!/json/i.test(response.headers.get('content-type') ?? '')) return response
  const body = await response.json()
  if (body && typeof body === 'object') {
    for (const [name, value] of [['status', response.status], ['headers', response.headers]]) {
      if (!(name in body)) Object.defineProperty(body, name, { value, enumerable: false })
    }
  }
  return body
}

/**
 * Same as {@link call}, but hands back the Response.
 *
 * @param {{ path: string }} route
 * @param {string} verb
 * @param {Parameters<typeof call>[2]} [input]
 * @returns {Promise<Response>}
 */
export async function respond(route, verb, input = {}) {
  const server = TestServer.current
  if (!server) throw new Error('No test server is running; call setupRoutes() first')
  const paths = input.paths ?? {}
  const path = route.path.replace(/\{([^}]+)\}/g, (_, name) => encodeURIComponent(String(paths[name] ?? '')))
  const url = new URL(server.url + path)
  for (const [key, value] of Object.entries(input.query ?? {})) {
    if (value !== undefined && value !== null) url.searchParams.set(key, String(value))
  }
  const headers = new Headers(input.headers ?? {})
  const token = input.context?.bearer?.token
  if (token) headers.set('authorization', `Bearer ${token}`)
  if (input.context?.ipAddress) headers.set('x-forwarded-for', input.context.ipAddress)
  const method = verb.toUpperCase()
  const hasBody = input.body !== undefined && method !== 'GET' && method !== 'HEAD'
  // A suite that passes its own headers controls the content type, as the
  // MCP checks rely on; otherwise JSON bodies go as JSON.
  if (hasBody && !input.headers && !headers.has('content-type')) headers.set('content-type', typeof input.body === 'string' ? 'text/plain' : 'application/json')
  return await fetch(url, {
    method,
    headers,
    body: hasBody ? (typeof input.body === 'string' ? input.body : JSON.stringify(input.body)) : undefined,
    redirect: 'manual',
  })
}
