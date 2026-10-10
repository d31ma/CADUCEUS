/**
 * Real application API for Playwright.
 *
 * Tachyon remains the frontend compiler and preview server. Its current Yon
 * runtime deliberately isolates one handler per process, defaults JavaScript
 * to Node, clears application environment variables, and does not support the
 * legacy CADUCEUS routes importing shared services/repositories. Production
 * API ownership is moving to Go; until that cutover is complete, this bounded
 * test-only Bun adapter invokes the same decorated route classes in-process.
 * It uses the real Go FYLO owner, released SESAME and FYLO binaries, schemas,
 * authorization guards, repositories, and response objects.
 */
import { Glob } from 'bun'
import { resolve, sep } from 'node:path'
import { pathToFileURL } from 'node:url'

const projectRoot = resolve(import.meta.dirname, '..')
const routesRoot = resolve(projectRoot, 'server', 'routes')
const apiPort = Number(process.env.E2E_API_PORT) || 19876
const corsHeaders = {
  'Access-Control-Allow-Origin': 'http://localhost:19875',
  'Access-Control-Allow-Methods': 'GET, POST, PUT, PATCH, DELETE, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Caduceus-Signature',
  'Access-Control-Allow-Private-Network': 'true',
  'Access-Control-Max-Age': '86400',
  'Vary': 'Origin',
}

const { startIdentity } = await import('../tests/support/identity.mjs')
const useExistingIdentity = process.env.CADUCEUS_TEST_USE_EXISTING_IDENTITY === 'true'
if (useExistingIdentity && (!process.env.FYLO_BRIDGE_ADDRESS || !process.env.FYLO_BRIDGE_SECRET)) {
  throw new Error('Existing test identity requires FYLO_BRIDGE_ADDRESS and FYLO_BRIDGE_SECRET')
}
const identity = useExistingIdentity
  ? { env: process.env, stop() {} }
  : await startIdentity()
Object.assign(process.env, identity.env, {
  ...(useExistingIdentity ? {} : { INBOUND_WEBHOOK_SECRET: 'caduceus-e2e-inbound-secret' }),
  STREAM_MAX_MS: '750',
  STREAM_POLL_MS: '50',
})

const routes = await discoverRoutes()
let ready = false
const server = Bun.serve({
  port: apiPort,
  // Listen on both loopback families. WebKit resolves localhost to ::1 while
  // Chrome commonly uses 127.0.0.1; a dual-stack listener keeps the browser
  // origin stable and avoids cross-host CORS/PNA differences.
  hostname: '::',
  idleTimeout: 30,
  fetch: dispatch,
})

if (!useExistingIdentity) await warmApplication()
ready = true
console.log(`[e2e-api] Bun route adapter listening on http://127.0.0.1:${apiPort}`)

let stopping = false
async function stop(code = 0) {
  if (stopping) return
  stopping = true
  server.stop(true)
  try {
    const { resetApp } = await import('../tests/support/legacy-server/app.js')
    await resetApp()
  } finally {
    identity.stop()
    process.exit(code)
  }
}

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.once(signal, () => { void stop(0) })
}

async function warmApplication() {
  const response = await fetch(`http://127.0.0.1:${apiPort}/test/seed/user`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'phase0-warmup@example.com',
      domains: ['example.com'],
      role: 'viewer',
    }),
    signal: AbortSignal.timeout(60_000),
  })
  if (!response.ok) {
    throw new Error(`CADUCEUS application warm-up failed (${response.status}): ${await response.text()}`)
  }
}

async function discoverRoutes() {
  const discovered = []
  for (const relative of new Glob('**/yon.js').scanSync({ cwd: routesRoot })) {
    const routePath = relative.split(sep).join('/').replace(/\/yon\.js$/, '')
    const segments = routePath.split('/').filter(Boolean)
    const parameters = []
    const pattern = segments.map(segment => {
      if (segment.startsWith('_')) {
        parameters.push(segment.slice(1))
        return '([^/]+)'
      }
      return escapeRegExp(segment)
    }).join('/')
    const module = await import(pathToFileURL(resolve(routesRoot, relative)).href)
    const staticSegments = segments.filter(segment => !segment.startsWith('_')).length
    discovered.push({
      pattern: new RegExp(`^/${pattern}/?$`),
      parameters,
      Handler: module.Handler,
      score: staticSegments * 100 + segments.length,
    })
  }
  return discovered.sort((left, right) => right.score - left.score)
}

async function dispatch(request) {
  // Playwright probes the configured webServer URL with GET before starting
  // tests. Keep this endpoint side-effect free and independent of app state.
  if (request.method === 'GET' && new URL(request.url).pathname === '/') {
    return json({ ok: ready, service: 'caduceus-e2e-api' }, ready ? 200 : 503)
  }
  if (request.method === 'OPTIONS') return withCors(new Response(null, { status: 204 }))
  const url = new URL(request.url)
  const matched = routes.map(route => ({ route, match: route.pattern.exec(url.pathname) }))
    .find(candidate => candidate.match)
  if (!matched) return json({ error: 'Not found', notFound: true }, 404)

  const method = matched.route.Handler?.[request.method]
  if (typeof method !== 'function') return json({ error: 'Method not allowed' }, 405)

  const paths = Object.fromEntries(matched.route.parameters.map((name, index) => [
    name,
    decodePathSegment(matched.match[index + 1]),
  ]))
  const query = Object.fromEntries(url.searchParams)
  const headers = Object.fromEntries(request.headers)
  const bearer = /^Bearer\s+(.+)$/i.exec(headers.authorization || '')?.[1]
  let body
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    const source = await request.text()
    if (source) {
      try { body = JSON.parse(source) } catch { return json({ error: 'Invalid JSON body' }, 400) }
    }
  }

  try {
    const result = await method.call(matched.route.Handler, {
      body,
      paths,
      query,
      headers,
      context: {
        bearer: bearer ? { token: bearer } : undefined,
        paths,
        query,
        ipAddress: request.headers.get('x-forwarded-for') || '127.0.0.1',
      },
    })
    return present(result)
  } catch (error) {
    console.error('[e2e-api] route failed:', error instanceof Error ? error.message : error)
    return json({ error: 'Internal server error' }, 500)
  }
}

function present(value) {
  if (value instanceof Response) return withCors(value)
  if (value && typeof value[Symbol.asyncIterator] === 'function') {
    return withCors(new Response(iterableStream(value), {
      status: 200,
      headers: {
        'Content-Type': 'text/event-stream; charset=utf-8',
        'Cache-Control': 'no-cache',
      },
    }))
  }
  if (value && typeof value === 'object' && Number.isInteger(value.status) && 'body' in value) {
    return withCors(new Response(value.body, { status: value.status, headers: value.headers }))
  }
  return json(value ?? null, responseStatus(value))
}

function responseStatus(value) {
  if (!value || typeof value !== 'object') return 200
  if (value.rateLimited) return 429
  if (value.blocked) return 422
  if (value.unavailable) return 503
  if (value.conflict) return 409
  if (value.notFound) return 404
  if (value.forbidden) return 403
  if (value.unauthorized) return 401
  if (value.error) return 400
  return 200
}

function json(value, status) {
  return withCors(Response.json(value, { status }))
}

function withCors(response) {
  const headers = new Headers(response.headers)
  for (const [name, value] of Object.entries(corsHeaders)) headers.set(name, value)
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  })
}

function iterableStream(iterable) {
  const encoder = new TextEncoder()
  return new ReadableStream({
    async start(controller) {
      try {
        for await (const chunk of iterable) controller.enqueue(encoder.encode(String(chunk)))
        controller.close()
      } catch (error) {
        controller.error(error)
      }
    },
  })
}

function decodePathSegment(value) {
  try { return decodeURIComponent(value) } catch { return value }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
