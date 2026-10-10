import { AuthBridgeError } from './AuthBridgeError.js'
import { createConnection } from 'node:net'
import { createHmac, randomBytes } from 'node:crypto'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

/** Composed operations. This list is the whole authentication surface. */
export class Auth {
  /**
   * Transport to the Go authentication and authorization boundary.
   *
   * This runtime holds no authentication logic. It carries a request to the
   * boundary and reports the answer: every credential, session, token, and
   * permission decision is made by the SESAME engine that the Go owner runs, and
   * nothing here can reach that engine except through the operations below.
   *
   * The envelope matches the FYLO bridge exactly, because it is the same
   * listener with the same key: one signed, replay-resistant, loopback-only
   * frame per connection.
   */

  static #PROTOCOL_VERSION = 1

  static #DEFAULT_MAX_REQUEST_BYTES = 1 << 20

  static #DEFAULT_MAX_RESPONSE_BYTES = 1 << 20

  static #REQUEST_TIMEOUT_MS = 15_000

  /** @returns {{ host: string, port: number } | null} */
  static #endpoint() {
    const address = ServerEnvironment.value('FYLO_BRIDGE_ADDRESS')
    if (!address) return null
    const match = /^\[(.+)\]:(\d+)$/.exec(address) ?? /^([^:]+):(\d+)$/.exec(address)
    if (!match) return null
    const [, host, port] = match
    if (host !== '127.0.0.1' && host !== '::1') return null
    return { host, port: Number(port) }
  }

  /**
   * Send one composed auth operation to the Go boundary.
   *
   * @param {string} op
   * @param {Record<string, unknown>} [params]
   * @param {{ maxRequestBytes?: number, maxResponseBytes?: number }} [limits]
   * @returns {Promise<any>}
   */
  static boundaryCall(op, params = {}, limits = {}) {
    return new Promise((resolve, reject) => {
      const target = Auth.#endpoint()
      const secret = ServerEnvironment.value('FYLO_BRIDGE_SECRET')
      if (!target || typeof secret !== 'string' || Buffer.byteLength(secret) < 32) {
        reject(new AuthBridgeError(
          'EUNAVAILABLE',
          'The Go boundary requires FYLO_BRIDGE_ADDRESS and FYLO_BRIDGE_SECRET',
        ))
        return
      }
      const maxRequestBytes = limits.maxRequestBytes ?? Auth.#DEFAULT_MAX_REQUEST_BYTES
      const maxResponseBytes = limits.maxResponseBytes ?? Auth.#DEFAULT_MAX_RESPONSE_BYTES
      const payload = Buffer.from(JSON.stringify({ op, params }), 'utf8')
      if (payload.byteLength > maxRequestBytes) {
        reject(new AuthBridgeError('EBADREQUEST', 'Boundary request is too large'))
        return
      }
      const timestamp = Date.now()
      const nonce = randomBytes(24).toString('base64url')
      const encoded = payload.toString('base64url')
      const signature = createHmac('sha256', secret)
        .update(`${timestamp}.${nonce}.${encoded}`)
        .digest('hex')
      const frame = Buffer.from(`${JSON.stringify({
        version: Auth.#PROTOCOL_VERSION, timestamp, nonce, payload: encoded, signature,
      })}\n`)

      const socket = createConnection({ host: target.host, port: target.port })
      let response = Buffer.alloc(0)
      let settled = false
      const finish = (error, value) => {
        if (settled) return
        settled = true
        socket.destroy()
        if (error) reject(error)
        else resolve(value)
      }
      socket.setTimeout(Auth.#REQUEST_TIMEOUT_MS, () =>
        finish(new AuthBridgeError('EUNAVAILABLE', 'The Go boundary did not answer')))
      socket.once('error', error =>
        finish(new AuthBridgeError('EUNAVAILABLE', 'The Go boundary is unavailable')))
      socket.on('data', chunk => {
        response = Buffer.concat([response, Buffer.from(chunk)])
        if (response.byteLength > maxResponseBytes + 1) {
          finish(new AuthBridgeError('EUNAVAILABLE', 'The Go boundary sent an oversized response'))
          return
        }
        const newline = response.indexOf(0x0a)
        if (newline === -1) return
        let parsed
        try {
          parsed = JSON.parse(response.subarray(0, newline).toString('utf8'))
        } catch {
          finish(new AuthBridgeError('EUNAVAILABLE', 'The Go boundary sent malformed JSON'))
          return
        }
        if (!parsed?.ok) {
          finish(new AuthBridgeError(
            parsed?.error?.code ?? 'EUNAVAILABLE',
            parsed?.error?.message ?? 'auth failed',
          ))
          return
        }
        finish(undefined, parsed.result)
      })
      socket.once('connect', () => socket.write(frame))
    })
  }

  // Compatibility name retained for authentication call sites while composed
  // feature boundaries share the same signed loopback transport.
  static authCall = Auth.boundaryCall

  /** Actions the authorization boundary understands. */
  static Actions = {
    MAILBOX_READ:      'mailbox:read',
    MAILBOX_WRITE:     'mailbox:write',
    MAILBOX_SEND:      'mailbox:send',
    DOMAIN_READ:       'domain:read',
    DOMAIN_ADMIN:      'domain:admin',
    CREDENTIAL_MANAGE: 'credential:manage',
  }

  /** @param {string} address */
  static loginBegin(address) {
    return Auth.authCall('auth.login.begin', { address })
  }

  /** @param {string} transactionId @param {string} password */
  static verifyPassword(transactionId, password) {
    return Auth.authCall('auth.login.password', { transactionId, password })
  }

  /** @param {string} transactionId */
  static passkeyOptions(transactionId) {
    return Auth.authCall('auth.login.passkey.options', { transactionId })
  }

  /** @param {{ transactionId: string, credentialId: string, authenticatorData: string, clientDataJSON: string, signature: string }} assertion */
  static verifyPasskey(assertion) {
    return Auth.authCall('auth.login.passkey', assertion)
  }

  /** @param {string} transactionId @param {string} code */
  static verifyTotp(transactionId, code) {
    return Auth.authCall('auth.login.totp', { transactionId, code })
  }

  /** @param {string} transactionId @param {string} code */
  static verifyRecoveryCode(transactionId, code) {
    return Auth.authCall('auth.login.recovery', { transactionId, code })
  }

  /** @param {string} transactionId */
  static completeLogin(transactionId) {
    return Auth.authCall('auth.login.complete', { transactionId })
  }

  /** @param {string} refreshToken @param {string} clientId */
  static refresh(refreshToken, clientId) {
    return Auth.authCall('auth.token.refresh', { refreshToken, clientId })
  }

  /** @param {string} token */
  static authenticate(token) {
    return Auth.authCall('auth.token.authenticate', { token })
  }

  /**
   * @param {string} token
   * @param {string} action
   * @param {{ mailbox?: string, domain?: string, resource?: string }} target
   */
  static authorize(token, action, target) {
    return Auth.authCall('auth.request.authorize', { token, action, ...target })
  }

  /** @param {string} idToken */
  static logout(idToken) {
    return Auth.authCall('auth.logout', { idToken })
  }

  /** @param {string} token @param {string} [reason] */
  static revokeSession(token, reason) {
    return Auth.authCall('auth.session.revoke', reason ? { token, reason } : { token })
  }

  /** @param {string} token @param {string} stepUpTransactionId */
  static passkeyRegisterBegin(token, stepUpTransactionId) {
    return Auth.authCall('auth.credential.passkey.begin', { token, stepUpTransactionId })
  }

  /** @param {string} token @param {string} attestationObject @param {string} clientDataJSON */
  static passkeyRegisterFinish(token, attestationObject, clientDataJSON) {
    return Auth.authCall('auth.credential.passkey.finish', { token, attestationObject, clientDataJSON })
  }

  /** @param {string} token */
  static passkeyList(token) {
    return Auth.authCall('auth.credential.passkey.list', { token })
  }

  /** @param {string} token @param {string} stepUpTransactionId @param {string} credentialId */
  static passkeyRemove(token, stepUpTransactionId, credentialId) {
    return Auth.authCall('auth.credential.passkey.remove', { token, stepUpTransactionId, credentialId })
  }

  /** @param {string} token @param {string} stepUpTransactionId */
  static totpEnroll(token, stepUpTransactionId) {
    return Auth.authCall('auth.credential.totp.enroll', { token, stepUpTransactionId })
  }

  /** @param {string} token @param {string} code */
  static totpActivate(token, code) {
    return Auth.authCall('auth.credential.totp.activate', { token, code })
  }

  /** @param {string} token @param {string} stepUpTransactionId */
  static recoveryCodesIssue(token, stepUpTransactionId) {
    return Auth.authCall('auth.credential.recovery.issue', { token, stepUpTransactionId })
  }

  /** @param {string} token @param {string} stepUpTransactionId @param {string} password */
  static setPassword(token, stepUpTransactionId, password) {
    return Auth.authCall('auth.credential.password.set', { token, stepUpTransactionId, password })
  }

  /** @param {string} domain */
  static provisionDomain(domain) {
    return Auth.authCall('auth.provision.domain', { domain })
  }

  /** @param {string} address */
  static provisionMailbox(address) {
    return Auth.authCall('auth.provision.mailbox', { address })
  }

  /**
   * Join one mailbox to a domain's admin or viewer group. The domain is
   * explicit and need not be the mailbox's own: that is what lets one identity
   * administer several domains with one login.
   *
   * @param {string} address
   * @param {string} domain
   * @param {'domain-admin' | 'domain-viewer'} role
   */
  static grantDomainRole(address, domain, role) {
    return Auth.authCall('auth.provision.role', { address, domain, role })
  }

  /** @param {string} address @param {string} domain @param {'domain-admin' | 'domain-viewer'} role */
  static revokeDomainRole(address, domain, role) {
    return Auth.authCall('auth.provision.role.revoke', { address, domain, role })
  }

  /** @param {string} address */
  static suspendMailbox(address) {
    return Auth.authCall('auth.provision.suspend', { address })
  }

  static jwks() {
    return Auth.authCall('auth.jwks')
  }

  static status() {
    return Auth.authCall('auth.status')
  }
}
