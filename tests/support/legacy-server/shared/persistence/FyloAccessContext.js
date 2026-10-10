import { AsyncLocalStorage } from 'node:async_hooks'
import { createHash } from 'node:crypto'
import { Security } from '../security/Security.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

/**
 * Trusted, request-local identity passed to Fylo's machine protocol.
 *
 * The mapping is deterministic so it does not require host accounts or a
 * second identity database. Namespace-separated SHA-256 values make deliberate
 * UID/GID collisions impractical; collisions still fail closed at the domain
 * membership check before a write is issued.
 */
export class FyloAccessContext {
  static #POSIX_ID_MIN = 100_000

  static #POSIX_ID_SPAN = 2_000_000_000

  static #TENANT_MODE = 0o660

  static #storage = new AsyncLocalStorage()

  /** @param {'mailbox-user' | 'mail-domain' | 'caduceus-service'} namespace @param {string} key */
  static posixId(namespace, key) {
    const digest = createHash('sha256').update(`${namespace}\0${key}`).digest()
    return FyloAccessContext.#POSIX_ID_MIN + (digest.readUInt32BE(0) % FyloAccessContext.#POSIX_ID_SPAN)
  }

  /** @param {unknown} record */
  static recordDomain(record) {
    if (!record || typeof record !== 'object' || Array.isArray(record)) return null
    const value = /** @type {Record<string, unknown>} */ (record)
    const explicit = FyloAccessContext.#normalizePossibleDomain(value.domain)
    if (explicit) return explicit
    for (const field of ['recipient', 'userEmail', 'email', 'sender']) {
      if (typeof value[field] !== 'string') continue
      const email = Security.normalizeEmailAddress(value[field])
      if (email) return email.split('@')[1] ?? null
    }
    if (Array.isArray(value.domains) && value.domains.length === 1) {
      return FyloAccessContext.#normalizePossibleDomain(value.domains[0])
    }
    if (value.meta && typeof value.meta === 'object') return FyloAccessContext.recordDomain(value.meta)
    return null
  }

  /** @param {unknown} value */
  static #normalizePossibleDomain(value) {
    return typeof value === 'string' ? Security.normalizeDomain(value) : null
  }

  /** @param {unknown} values @returns {string[]} */
  static #normalizeDomains(values) {
    if (!Array.isArray(values)) return []
    return [...new Set(
      values
        .map(FyloAccessContext.#normalizePossibleDomain)
        .filter(/** @returns {value is string} */ value => value !== null),
    )]
  }

  /** @param {string} message */
  static #accessError(message) {
    const error = /** @type {Error & { code: string }} */ (new Error(message))
    error.code = 'EACCES'
    return error
  }

  static enabled() {
    return ServerEnvironment.value('FYLO_POSIX_ACCESS') === 'true'
  }

  /**
   * @param {{ email: string, domains: string[] }} claims
   * @param {() => any} task
   */
  static run(claims, task) {
    if (!this.enabled()) return task()
    const email = Security.normalizeEmailAddress(claims.email)
    const domains = FyloAccessContext.#normalizeDomains(claims.domains)
    if (!email || domains.length === 0) return task()
    return FyloAccessContext.#storage.run({
      uid: FyloAccessContext.posixId('mailbox-user', /** @type {string} */ (email)),
      groups: domains.map(domain => FyloAccessContext.posixId('mail-domain', domain)),
      domains,
    }, task)
  }

  /**
   * Run trusted delivery work as a member of one managed mail domain.
   *
   * @param {string} domain
   * @param {() => any} task
   */
  static runForDomain(domain, task) {
    if (!this.enabled()) return task()
    const normalized = Security.normalizeDomain(domain)
    if (!normalized) throw FyloAccessContext.#accessError('A valid domain is required for tenant storage')
    const uid = typeof process.getuid === 'function'
      ? process.getuid()
      : FyloAccessContext.posixId('caduceus-service', 'mail-delivery')
    return FyloAccessContext.#storage.run({
      uid,
      groups: [FyloAccessContext.posixId('mail-domain', normalized)],
      domains: [normalized],
    }, task)
  }

  static read() {
    if (!this.enabled()) return undefined
    const context = FyloAccessContext.#storage.getStore()
    if (!context) return undefined
    return { uid: context.uid, groups: [...context.groups] }
  }

  /**
   * Resolve group ownership for a newly written tenant record.
   *
   * @param {unknown} record
   */
  static write(record) {
    if (!this.enabled()) return undefined
    const context = FyloAccessContext.#storage.getStore()
    if (!context) return undefined
    const domain = FyloAccessContext.recordDomain(record)
    if (!domain) return undefined
    const index = context.domains.indexOf(domain)
    if (index === -1) throw FyloAccessContext.#accessError(`Storage access denied for domain ${domain}`)
    return { gid: context.groups[index], mode: FyloAccessContext.#TENANT_MODE }
  }
}
