import { Fylo as BinaryFylo } from '../../deps/fylo.mjs'
import { FyloAccessContext } from './FyloAccessContext.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

/**
 * Compatibility layer for the former in-process FYLO query cursor. The latest
 * runtime returns a complete document record from its binary protocol.
 */
export class Fylo extends /** @type {any} */ (BinaryFylo) {

  static #withAccess(operation, access) {
    return access && typeof operation?.as === 'function' ? operation.as(access) : operation
  }

  static #deferredAccess(task, defaultAccess) {
    let access = defaultAccess
    let started = false
    const operation = Promise.resolve().then(() => {
      started = true
      return task(access)
    })
    return Object.assign(operation, {
      as(input) {
        if (started) throw new Error('as() must be called before the operation starts')
        access = input
        return operation
      },
    })
  }

  static #oneBatchAccess(batch) {
    const accesses = batch
      .map(record => FyloAccessContext.write(record))
      .filter(Boolean)
    if (accesses.length === 0) return undefined
    const [first] = accesses
    if (accesses.some(access => access.gid !== first.gid || access.mode !== first.mode)) {
      const error = /** @type {Error & { code: string }} */ (
        new Error('A Fylo batch write cannot span tenant domains')
      )
      error.code = 'EACCES'
      throw error
    }
    return first
  }

  /** @readonly @enum {string} */
  static Collections = {
    DOMAINS:           'domains',
    DOMAIN_MIGRATIONS: 'domain-migrations',
    EMAILS:            'emails',
    ATTACHMENTS:       'attachments',
    RAW_MESSAGES:      'raw-messages',
    SUPPRESSED:        'suppressed',
    USERS:             'users',
    INBOX_RULES:       'inbox-rules',
    MFA_DEVICES:       'mfa-devices',
    MFA_SESSIONS:      'mfa-sessions',
    SETUP_SESSIONS:    'setup-sessions',
    AUTH_CREDENTIALS:  'auth-credentials',
    AUTH_TRANSACTIONS: 'auth-transactions',
    RECOVERY_CODES:    'recovery-codes',
    AUTH_AUDIT_EVENTS: 'auth-audit-events',
    PUSH_SUBSCRIPTIONS:'push-subscriptions',
    RATE_LIMITS:       'rate-limits',
    OUTBOX:            'outbox',
    SEND_REQUESTS:     'send-requests',
    SNOOZED:           'snoozed',
    MAIL_ACTIONS:      'mail-actions',
    SCHEDULED:         'scheduled',
    FOLLOW_UP_REMINDERS:'follow-up-reminders',
    TEMPLATES:         'templates',
    SIGNATURES:        'signatures',
    CALENDAR_EVENTS:   'calendar-events',
    PRESENCE:          'presence',
    CONTACTS:          'contacts',
    CONTACT_GROUPS:    'contact-groups',
    THREAD_SHARES:     'thread-shares',
    MCP_ACCESS_TOKENS: 'mcp-access-tokens',
    MCP_AUDIT_EVENTS:  'mcp-audit-events',
    EXTENSIONS:        'extensions',
  }

  /** @readonly @enum {string} */
  static Buckets = {
    ATTACHMENT_FILES: 'attachment-files',
    RAW_MESSAGE_FILES: 'raw-message-files',
  }

  /**
   * Create a Fylo instance and ensure all collections exist.
   *
   * Pass a custom `root` for test isolation. In production `FYLO_ROOT` must be
   * set to durable local storage. The Docker image uses `/data/caduceus`.
   *
   * @param {string} [root]
   * @returns {Promise<Fylo>}
   */
  static async createDb(root) {
    const fyloRoot = root ?? ServerEnvironment.value('FYLO_ROOT')
    if (!fyloRoot) {
      throw new Error(
        'FYLO_ROOT environment variable is required. ' +
        'Set it to the directory where Fylo should store data. ' +
        'For Docker, the image already sets FYLO_ROOT=/data/caduceus.'
      )
    }

    const bridgeAddress = ServerEnvironment.value('FYLO_BRIDGE_ADDRESS')
    const bridgeSecret = ServerEnvironment.value('FYLO_BRIDGE_SECRET')
    if (ServerEnvironment.value('NODE_ENV') === 'production' && (!bridgeAddress || !bridgeSecret)) {
      throw new Error(
        'Production persistence requires the Go-owned FYLO_BRIDGE_ADDRESS and FYLO_BRIDGE_SECRET'
      )
    }
    const binary = ServerEnvironment.value('FYLO_BINARY') || ServerEnvironment.value('FYLO_EXEC_PATH')
    const options = bridgeAddress
      ? { bridgeAddress, bridgeSecret }
      : { ...(binary ? { binary } : {}), exclusiveRoot: true }
    const fylo = new Fylo(fyloRoot, options)
    const identity = await fylo.handshake()
    if (FyloAccessContext.enabled() && identity?.capabilities?.handshake !== true) {
      await fylo.close()
      throw new Error(
        'FYLO_POSIX_ACCESS requires a Fylo binary with the issue-68 machine-access protocol. ' +
        'Build the local FYLO checkout and set FYLO_BINARY to its dist-bin/fylo executable.'
      )
    }
    // Collection catalog mutation is intentionally sequential. The standalone
    // machine protocol is pipelined, but catalog initialization is a one-time
    // startup operation and concurrent creates can contend on the same local
    // catalog/root lock.
    for (const name of Object.values(Fylo.Collections)) {
      await fylo.createCollection(name)
    }
    for (const name of Object.values(Fylo.Buckets)) {
      await fylo.createCollection(name, 'file')
    }
    return fylo
  }

  /**
   * Collect a Fylo async generator into a plain `{ docId: doc }` record.
   * @template {Record<string, any>} T
   * @param {AsyncIterable<any>} gen
   * @returns {Promise<Record<string, T>>}
   */
  static async collect(gen) {
    /** @type {Record<string, T>} */
    const results = /** @type {Record<string, T>} */ ({})
    for await (const doc of gen) Object.assign(results, doc)
    return results
  }

  /**
   * @param {string} root
   * @param {{ binary?: string, env?: string | Record<string, string | number | boolean | null | undefined>, worm?: boolean, exclusiveRoot?: boolean, strictSchema?: boolean, shardWidth?: number, bridgeAddress?: string, bridgeSecret?: string, maxRequestBytes?: number, maxResponseBytes?: number }} [options]
   */
  constructor(root, options = {}) {
    super(root, options)
  }

  putData(collection, data, meta) {
    return Fylo.#withAccess(
      super.putData(collection, data, meta),
      FyloAccessContext.write(data) ?? FyloAccessContext.write(meta),
    )
  }

  batchPutData(collection, batch) {
    const access = Fylo.#oneBatchAccess(batch)
    return Fylo.#withAccess(super.batchPutData(collection, batch), access)
  }

  putFile(collection, filePath, options = {}) {
    return Fylo.#withAccess(
      super.putFile(collection, filePath, options),
      FyloAccessContext.write(options.meta),
    )
  }

  getFileBytes(collection, id) {
    return Fylo.#withAccess(super.getFileBytes(collection, id), FyloAccessContext.read())
  }

  getDoc(collection, id) {
    return Fylo.#withAccess(super.getDoc(collection, id), FyloAccessContext.read())
  }

  getMeta(collection, id) {
    return Fylo.#withAccess(super.getMeta(collection, id), FyloAccessContext.read())
  }

  setMeta(collection, id, meta) {
    return Fylo.#withAccess(super.setMeta(collection, id, meta), FyloAccessContext.read())
  }

  getLatest(collection, id, onlyId = false) {
    return Fylo.#withAccess(super.getLatest(collection, id, onlyId), FyloAccessContext.read())
  }

  findDocs(collection, query) {
    let result = super.findDocs(collection, query)
    const cursor = {
      as(access) {
        result = result.as(access)
        return cursor
      },
      async *collect() {
        const documents = await result
        for (const [id, document] of Object.entries(documents ?? {})) {
          yield { [id]: document }
        }
      },
    }
    const access = FyloAccessContext.read()
    return access ? cursor.as(access) : cursor
  }

  /**
   * Read one bounded page from FYLO's immutable query snapshot.
   *
   * The unpaged compatibility cursor remains useful for small collections,
   * but mailbox exports must not depend on one response containing an entire
   * folder.  Keep the native cursor opaque and bind it to the same trusted
   * request identity as every other read.
   */
  findDocsPage(collection, query, page = {}) {
    return Fylo.#withAccess(super.findDocsPage(collection, query, page), FyloAccessContext.read())
  }

  findDeletedDocs(collection, query = {}) {
    return Fylo.#withAccess(
      super.findDeletedDocs(collection, query),
      FyloAccessContext.read(),
    )
  }

  joinDocs(join) {
    return Fylo.#withAccess(super.joinDocs(join), FyloAccessContext.read())
  }

  patchDoc(collection, idOrDocuments, newDocument, oldDocument) {
    const defaultAccess = FyloAccessContext.read()
    if (typeof idOrDocuments === 'string') {
      return Fylo.#withAccess(
        super.patchDoc(collection, idOrDocuments, newDocument, oldDocument),
        defaultAccess,
      )
    }

    return Fylo.#deferredAccess(async access => {
      const results = []
      for (const [id, document] of Object.entries(idOrDocuments ?? {})) {
        results.push(await Fylo.#withAccess(super.patchDoc(collection, id, document), access))
      }
      return results
    }, defaultAccess)
  }

  patchDocs(collection, update) {
    return Fylo.#withAccess(super.patchDocs(collection, update), FyloAccessContext.read())
  }

  delDoc(collection, id) {
    return Fylo.#withAccess(super.delDoc(collection, id), FyloAccessContext.read())
  }

  delDocs(collection, criteria) {
    return Fylo.#withAccess(super.delDocs(collection, criteria), FyloAccessContext.read())
  }

  restoreDoc(collection, id) {
    return Fylo.#withAccess(super.restoreDoc(collection, id), FyloAccessContext.read())
  }
}
