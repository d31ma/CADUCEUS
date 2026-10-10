import { RouteRequest } from '../../server/shared/http/RouteRequest.js'

/**
 * Calls one Tachyon controller route the way Tachyon would: it builds the
 * Fetch `Request`, passes the `@FromPath` values in path order, and hands back
 * what the route answered: its plain value, or the `Response` it built itself.
 *
 * @param {any} Controller
 * @param {string} verb
 * @param {{ paths?: Record<string, any>, query?: Record<string, any>, body?: any, headers?: Record<string, string>, context?: { bearer?: { token?: string }, ipAddress?: string } }} [input]
 */
export async function call(Controller, verb, input = {}) {
  const response = await respond(Controller, verb, input)
  const wrapped = response instanceof Response ? RouteRequest.answerOf(response) : undefined
  return wrapped ? wrapped.answer : response
}

/**
 * Same as {@link call}, but hands back the `Response` so a suite can read
 * its status and headers.
 *
 * @param {any} Controller
 * @param {string} verb
 * @param {Parameters<typeof call>[2]} [input]
 * @returns {Promise<Response>}
 */
export async function respond(Controller, verb, input = {}) {
  const path = Controller.tachyonPath ?? '/'
  const paths = input.paths ?? {}
  const names = [...path.matchAll(/\{([^}]+)\}/g)].map(match => match[1])
  const url = new URL(`http://caduceus.test${path.replace(/\{([^}]+)\}/g, (_, name) => encodeURIComponent(String(paths[name] ?? '')))}`)
  for (const [key, value] of Object.entries(input.query ?? {})) {
    if (value !== undefined && value !== null) url.searchParams.set(key, String(value))
  }
  const headers = new Headers(input.headers ?? {})
  const token = input.context?.bearer?.token
  if (token) headers.set('authorization', `Bearer ${token}`)
  if (input.context?.ipAddress) headers.set('x-forwarded-for', input.context.ipAddress)
  const method = verb.toUpperCase()
  const hasBody = input.body !== undefined && method !== 'GET' && method !== 'HEAD'
  if (hasBody && typeof input.body === 'string' && !headers.has('content-type')) headers.set('content-type', 'text/plain')
  const request = new Request(url, {
    method,
    headers,
    body: hasBody ? (typeof input.body === 'string' ? input.body : JSON.stringify(input.body)) : undefined,
  })
  return await new Controller()[verb.toLowerCase()](...names.map(name => paths[name]), request)
}
