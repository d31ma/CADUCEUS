// @ts-check
import { createHmac, timingSafeEqual } from 'node:crypto'
import { AuthGuard } from './AuthGuard.js'
import { Security } from '../platform/security/Security.js'
import { Respond } from '../platform/http/Respond.js'
import { FyloAccessContext } from '../platform/persistence/FyloAccessContext.js'
/** @import { RouteSpec } from '../models/RouteSpec.js' */
/** @import { AuthOptions } from '../models/AuthOptions.js' */
/** @import { RouteInput } from '../models/RouteInput.js' */

/**
 * Turns the Fetch `Request` that a Tachyon controller method receives into the
 * values a CADUCEUS route reads, runs the route's access checks, and turns the
 * route's answer into a Fetch `Response`.
 *
 * The access checks are the ones the Go authorization boundary owns. Tachyon's
 * `@SignedIn` only proves that the identity provider recognised the bearer
 * token; the action and target of the request are still the boundary's call.
 */
export class RouteRequest {
  /** The shapes the authorization boundary accepts. */
  static DOMAIN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i
  static ADDRESS = /^[a-z0-9._-]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i

  /** @type {WeakMap<Response, { answer: unknown }>} */
  static #answers = new WeakMap()

  /** Marks a JSON request body that does not parse. */
  static INVALID_JSON = Symbol('invalid JSON body')

  /** Error shapes from {@link Respond}, matched in the order the old route schemas listed them. */
  static STATUS_MARKERS = /** @type {const} */ ([
    ['unauthorized', 401],
    ['forbidden', 403],
    ['notFound', 404],
    ['conflict', 409],
    ['blocked', 422],
    ['rateLimited', 429],
    ['unavailable', 503],
  ])

  /**
   * @param {Request} request
   * @param {RouteSpec} spec
   * @param {(input: RouteInput) => unknown} route
   * @returns {Promise<Response>}
   */
  static async handle(request, spec, route) {
    const input = await RouteRequest.read(request, spec.paths)
    return RouteRequest.respond(await RouteRequest.#run(input, spec, route))
  }

  /**
   * @param {Request} request
   * @param {Record<string, string>} [paths]
   * @returns {Promise<RouteInput>}
   */
  static async read(request, paths = {}) {
    const url = new URL(request.url)
    const query = Object.fromEntries(url.searchParams)
    /** @type {Record<string, string>} */
    const headers = {}
    request.headers.forEach((value, name) => { headers[name.toLowerCase()] = value })
    const token = RouteRequest.#bearer(headers.authorization)
    return {
      paths,
      query,
      body: await RouteRequest.#body(request),
      headers,
      method: request.method.toUpperCase(),
      context: {
        ...(token ? { bearer: { token } } : {}),
        ipAddress: RouteRequest.#ipAddress(headers),
        query,
      },
    }
  }

  /**
   * @param {unknown} answer
   * @returns {Response}
   */
  static respond(answer) {
    if (answer instanceof Response) return answer
    const response = answer === undefined
      ? new Response(null, { status: 204 })
      : Response.json(answer, { status: RouteRequest.status(answer) })
    RouteRequest.#answers.set(response, { answer })
    return response
  }

  /**
   * What the route returned before {@link respond} wrapped it, for callers in
   * this process such as the route test suites. A `Response` the route built
   * itself has no entry.
   *
   * @param {Response} response
   * @returns {{ answer: unknown } | undefined}
   */
  static answerOf(response) {
    return RouteRequest.#answers.get(response)
  }

  /**
   * @param {unknown} answer
   * @returns {number}
   */
  static status(answer) {
    if (!answer || typeof answer !== 'object' || Array.isArray(answer)) return 200
    const record = /** @type {Record<string, unknown>} */ (answer)
    if (typeof record.error !== 'string') return 200
    const marker = RouteRequest.STATUS_MARKERS.find(([field]) => record[field] !== undefined)
    if (marker) return marker[1]
    return Object.keys(record).length === 1 ? 400 : 200
  }

  /**
   * @param {RouteInput} input
   * @param {RouteSpec} spec
   * @param {(input: RouteInput) => unknown} route
   * @returns {Promise<unknown>}
   */
  static async #run(input, spec, route) {
    if (input.body === RouteRequest.INVALID_JSON) return Respond.r400('The request body is not valid JSON')
    if (spec.testOnly && !Security.isTestRoutesEnabled()) return Respond.r403('Only available in test mode')
    if (spec.signature && !RouteRequest.#signed(input, spec.signature)) return Respond.r401('Invalid webhook signature')
    if (!spec.guard) return await route(input)
    const result = await RouteRequest.#authorize(input, spec.guard)
    if (!result.ok) return result.response
    // Storage ownership follows the domain the boundary authorized, not the
    // caller's own address.
    const domain = result.domain ?? result.principal.domain
    input.auth = { ...result.principal, email: result.principal.address, domain, domains: [domain] }
    return await FyloAccessContext.run(
      { email: result.principal.address, domains: [domain] },
      () => route(input),
    )
  }

  /**
   * @param {RouteInput} input
   * @param {AuthOptions} options
   * @returns {Promise<import('./AuthGuard.js').AuthResult>}
   */
  static async #authorize(input, options) {
    const context = input.context
    if (!options.action) return AuthGuard.requireAuth(context)
    const named = options.domain?.(input) || options.mailbox?.(input)
    if (named && options.domain && RouteRequest.DOMAIN.test(named)) {
      return AuthGuard.requirePermission(context, options.action, { domain: named })
    }
    if (named && options.mailbox && RouteRequest.ADDRESS.test(named)) {
      return AuthGuard.requirePermission(context, options.action, { mailbox: named })
    }
    // Nothing named: the action applies to what the caller owns, which is the
    // least authority available.
    const authenticated = await AuthGuard.requireAuth(context)
    if (!authenticated.ok) return authenticated
    const target = options.action.startsWith('domain:')
      ? { domain: authenticated.principal.domain }
      : { mailbox: authenticated.principal.address }
    return AuthGuard.requirePermission(context, options.action, target)
  }

  /**
   * @param {RouteInput} input
   * @param {string} settingName
   * @returns {boolean}
   */
  static #signed(input, settingName) {
    const secret = Security.requireEnv(settingName)
    const signature = input.headers['x-caduceus-signature'] ?? ''
    const expected = createHmac('sha256', secret).update(JSON.stringify(input.body ?? {})).digest('hex')
    return signature.length === expected.length
      && timingSafeEqual(Buffer.from(signature, 'hex'), Buffer.from(expected, 'hex'))
  }

  /**
   * @param {Request} request
   * @returns {Promise<any>}
   */
  static async #body(request) {
    if (request.method === 'GET' || request.method === 'HEAD') return undefined
    const text = await request.text()
    if (!text) return undefined
    if (!/json/i.test(request.headers.get('content-type') ?? 'application/json')) return text
    try {
      return JSON.parse(text)
    } catch {
      return RouteRequest.INVALID_JSON
    }
  }

  /**
   * @param {string | undefined} authorization
   * @returns {string}
   */
  static #bearer(authorization) {
    const match = /^Bearer\s+(.+)$/i.exec(authorization ?? '')
    return match ? match[1].trim() : ''
  }

  /**
   * @param {Record<string, string>} headers
   * @returns {string}
   */
  static #ipAddress(headers) {
    return (headers['x-forwarded-for'] ?? '').split(',')[0].trim() || headers['x-real-ip'] || ''
  }
}
