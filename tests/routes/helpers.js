/**
 * In-process route-handler harness.
 *
 * tests/api spawns `ty serve` in a subprocess, so none of its execution shows
 * up in this process's coverage. Route handlers are plain static methods that
 * take `{ paths, query, body, headers, context }` and return a plain object,
 * so they can be called directly against a temp-rooted provider container.
 *
 * ponytail: no HTTP layer, no schema validation, no decorator emulation --
 * the decorators are real, they just run in-process. Add a transport-level
 * suite only if a bug ever escapes through routing itself.
 */
import { startIdentity, identityRequirements } from '../support/identity.mjs'
import { TestContainer } from '../support/TestContainer.mjs'
import { TestServer } from '../support/TestServer.mjs'

export { identityRequirements }

/**
 * Identities the route suites sign in as. They are minted once, up front,
 * because a real login is a round trip to the engine and `token()` is called
 * from assertions that are not async.
 */
const POOL = [
  { email: 'alice@example.com', role: 'admin' },
  { email: 'bob@example.com', role: 'admin' },
  { email: 'carol@other.com', role: 'admin' },
  // A mailbox with an identity but no CADUCEUS user record, for the routes
  // that must answer 404 rather than assume one exists.
  { email: 'ghost@example.com', role: 'viewer' },
  // Holds no domain authority at all, for the routes that must refuse one.
  { email: 'viewer@example.com', role: 'none' },
  // Administrators of domains that hold no CADUCEUS records, for the routes
  // that must answer 404 or 403 rather than leak another domain's data.
  { email: 'dave@unset.com', role: 'admin' },
  { email: 'erin@far.com', role: 'admin' },
]

/** @type {{ identity: any, app: any, tokens: Map<string, string>, restore: Map<string, string | undefined> } | null} */
let active = null

/**
 * Create a route harness backed by the real Go owner.
 *
 * Route handlers now reach an authorization boundary, so there is no in-process
 * shortcut left: the guard asks the engine, and the engine has to be running.
 * Persistence goes through the same bridge for the same reason it does in
 * production — one writer per FYLO root.
 */
export async function setupRoutes() {
  const identity = await startIdentity()
  // Remember what this harness overwrites. A suite that runs after it in the
  // same process must not inherit a pointer to a bridge that has stopped.
  const restore = new Map()
  for (const [key, value] of Object.entries(identity.env)) {
    restore.set(key, process.env[key])
    process.env[key] = value
  }

  // Suites seed and inspect records through this container; the routes run
  // in the application server over HTTP and share the same FYLO bridge.
  const app = await TestContainer.open()
  await TestServer.start({
    ...identity.env,
    NODE_ENV: 'test',
    CADUCEUS_ENABLE_TEST_ROUTES: 'true',
    STREAM_MAX_MS: '750',
    STREAM_POLL_MS: '50',
  })
  const tokens = new Map()
  for (const member of POOL) {
    const seeded = await identity.mint(member.email, member.role)
    tokens.set(member.email, seeded.token)
  }
  active = { identity, app, tokens, restore }
  return app
}

export async function teardownRoutes() {
  if (!active) return
  TestServer.stop()
  await TestContainer.close()
  active.identity.stop()
  for (const [key, value] of active.restore) {
    if (value === undefined) delete process.env[key]
    else process.env[key] = value
  }
  active = null
}

export function routeDiagnostics() {
  return active?.identity?.diagnostics?.() ?? ''
}

/**
 * Return the real token for one pooled identity.
 *
 * There is no claims argument beyond the address any more, and that is the
 * change: a token cannot assert a role or a domain list, so a test cannot
 * either. Authority is a grant, and a test that needs different authority
 * grants it through the harness.
 *
 * @param {Partial<{ email: string }>} [identity]
 * @returns {string}
 */
export function token(identity = {}) {
  if (!active) throw new Error('call setupRoutes() before token()')
  const email = identity.email ?? 'alice@example.com'
  const issued = active.tokens.get(email)
  if (!issued) {
    throw new Error(
      `${email} is not in the route-test identity pool; add it to POOL in tests/routes/helpers.js`,
    )
  }
  return issued
}

/** Give one pooled identity authority over another domain. */
export async function grantDomain(email, domain, role = 'domain-admin') {
  if (!active) throw new Error('call setupRoutes() before grantDomain()')
  const { grantDomain: grant } = await import('../support/identity.mjs')
  return grant(active.identity.env, email, domain, role)
}

let seq = 0

/** @param {Partial<import('@/types').StoredEmail>} [over] */
export function email(over = {}) {
  seq++
  return {
    id: `e${seq}`,
    domain: 'example.com',
    recipient: 'alice@example.com',
    sender: 'bob@other.com',
    subject: `Subject ${seq}`,
    body: 'body text',
    folder: 'inbox',
    read: false,
    starred: false,
    receivedAt: new Date(Date.now() - seq * 1000).toISOString(),
    processed: true,
    ...over,
  }
}

/**
 * Build the request object Tachyon hands a handler.
 * @param {{ body?: any, paths?: Record<string, string>, query?: Record<string, string>, headers?: Record<string, string>, token?: string, ip?: string }} [opts]
 */
export function req(opts = {}) {
  // Handlers read query/paths from either the top level or `context`,
  // depending on the route. Populate both.
  return {
    body: opts.body,
    paths: opts.paths ?? {},
    query: opts.query ?? {},
    headers: opts.headers ?? {},
    context: {
      bearer: opts.token ? { token: opts.token } : undefined,
      query: opts.query ?? {},
      paths: opts.paths ?? {},
      ipAddress: opts.ip ?? '127.0.0.1',
    },
  }
}
