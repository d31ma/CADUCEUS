import { readFileSync } from 'node:fs'
import { AcmeService } from '../../domains/services/AcmeService.js'
import { listenForConnections } from '../../../platform/runtime/ListenableServer.ts'
import { ServerEnvironment } from '../../../platform/runtime/ServerEnvironment.js'

export class SubmissionServer {
  /** @param {SubmissionOptions} options */
  constructor(options) {
    this.secure = !!options.secure
    this.port = Number(options.port ?? (this.secure ? 465 : 587))
    this.host = options.host ?? ServerEnvironment.value('SMTP_SUBMISSION_HOST') ?? '0.0.0.0'
    this.apiUrl = (options.apiUrl ?? `http://127.0.0.1:${ServerEnvironment.value('PORT') || 8080}`).replace(/\/$/, '')
    this.hostname = options.hostname ?? ServerEnvironment.value('MAIL_HOSTNAME') ?? 'localhost'
    this.maxSize = Number(options.maxSize ?? ServerEnvironment.value('SMTP_MAX_SIZE') ?? 26_214_400)
    this.tls = options.tls ?? SubmissionServer.loadTls()
    this.allowInsecureAuth = !!options.allowInsecureAuth

    if (!this.tls && !this.allowInsecureAuth) {
      throw new Error('SMTP submission requires TLS (MAIL_TLS_CERT/KEY)')
    }
  }

  /**
   * Start the submission server. Resolves once it is listening.
   *
   * @returns {Promise<import('smtp-server').SMTPServer>}
   */
  async start() {
    const { SMTPServer } = await import('../../../deps/npm/smtp-server.mjs')
    const { simpleParser } = await import('../../../deps/npm/mailparser.mjs')

    // Serve a fresh context per handshake when TLS lives in files, so ACME
    // renewals apply without restarting the submission server.
    const keyFile = ServerEnvironment.value('MAIL_TLS_KEY_FILE'), certFile = ServerEnvironment.value('MAIL_TLS_CERT_FILE')
    const sni = this.tls && keyFile && certFile ? AcmeService.sniFromFiles(keyFile, certFile) : null

    const server = new SMTPServer({
      name: this.hostname,
      banner: `${this.hostname} ESMTP CADUCEUS submission`,
      size: this.maxSize,
      secure: this.secure,
      authMethods: ['PLAIN', 'LOGIN'],
      // Never let credentials cross the wire in the clear: require STARTTLS
      // before AUTH on 587 (465 is already implicitly TLS). `allowInsecureAuth`
      // is a dev/testing escape hatch only.
      allowInsecureAuth: this.allowInsecureAuth,
      ...(this.tls ? { key: this.tls.key, cert: this.tls.cert } : {}),
      ...(sni ? { SNICallback: sni } : {}),

      onAuth: (auth, session, callback) => this.onAuth(auth, callback),
      onData: (stream, session, callback) => this.onData(stream, session, callback, simpleParser),
    })

    server.on('error', (err) => console.error('[smtp-sub] server error:', err))
    await listenForConnections(server, this.port, this.host)
    console.log(`[smtp-sub] submission server listening on ${this.host}:${this.port} (${this.secure ? 'implicit TLS' : 'STARTTLS'})`)
    return server
  }

  /**
   * Refuse submission authentication.
   *
   * A mail client offers one secret. A CADUCEUS session requires multi-factor
   * assurance, so no password on its own can produce one, and issuing a session
   * from a single factor for this protocol alone would be exactly the silent
   * policy weakening the boundary exists to prevent.
   *
   * Phase 6 adds application passwords: a SESAME workload principal per client,
   * holding `mailbox:send` for one mailbox and nothing else, revocable on its
   * own without touching the human's credentials.
   *
   * @param {{ username?: string, password?: string }} auth
   * @param {(err: Error | null, response?: { user: string }) => void} callback
   */
  onAuth(auth, callback) {
    void auth
    callback(new Error(
      'SMTP submission requires an application password, which this release does not issue yet. ' +
      'Send through the web client until application passwords ship.',
    ))
  }

  /**
   * Parse an authenticated submission and push it through `POST /send`,
   * enforcing that the envelope sender matches the authenticated user.
   *
   * @param {NodeJS.ReadableStream & { sizeExceeded?: boolean }} stream
   * @param {import('smtp-server').SMTPServerSession} session
   * @param {(err?: Error) => void} callback
   * @param {(raw: Buffer) => Promise<import('mailparser').ParsedMail>} simpleParser
   */
  onData(stream, session, callback, simpleParser) {
    SubmissionServer.readStream(stream, this.maxSize)
      .then(async (raw) => {
        if (stream.sizeExceeded) return SubmissionServer.reject(callback, 552, 'Message exceeds maximum permitted size')
        const { email, token } = JSON.parse(session.user || '{}')

        // Enforce that the envelope sender matches the authenticated user — an
        // authenticated client must not spoof another address.
        const envelopeFrom = session.envelope.mailFrom ? session.envelope.mailFrom.address.toLowerCase() : ''
        if (envelopeFrom && envelopeFrom !== email) {
          return SubmissionServer.reject(callback, 550, 'Sender address does not match the authenticated user')
        }

        const parsed = await simpleParser(raw)
        /** @param {'to' | 'cc'} field */
        const addrs = (field) => {
          const value = parsed[field]
          const groups = Array.isArray(value) ? value : value ? [value] : []
          return groups
            .flatMap((group) => group.value)
            .map((address) => String(address.address).toLowerCase())
            .filter(Boolean)
        }
        const to = addrs('to')
        const cc = addrs('cc')
        // BCC recipients appear in the SMTP envelope but not the headers.
        const headerRcpts = new Set([...to, ...cc])
        const bcc = session.envelope.rcptTo
          .map((r) => r.address.toLowerCase())
          .filter((a) => !headerRcpts.has(a))

        const payload = {
          to, ...(cc.length ? { cc } : {}), ...(bcc.length ? { bcc } : {}),
          subject: parsed.subject || '(no subject)',
          ...(parsed.text ? { text: parsed.text } : {}),
          ...(parsed.html ? { html: parsed.html } : {}),
        }

        const res = await fetch(`${this.apiUrl}/send`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify(payload),
        })
        if (res.ok) return callback()
        if (res.status === 429) return SubmissionServer.reject(callback, 452, 'Sending rate limit exceeded, try again later')
        if (res.status === 422) return SubmissionServer.reject(callback, 550, 'One or more recipients are not allowed')
        SubmissionServer.reject(callback, 554, 'Message rejected by server')
      })
      .catch((err) => {
        console.error('[smtp-sub] submission failed:', err)
        SubmissionServer.reject(callback, 451, 'Temporary failure, try again later')
      })
  }

  /** @param {string} path */
  static safeRead(path) { try { return readFileSync(path, 'utf8') } catch { return '' } }

  /** Read TLS material from env (paths or inline PEM). */
  static loadTls() {
    const key = ServerEnvironment.value('MAIL_TLS_KEY') || (ServerEnvironment.value('MAIL_TLS_KEY_FILE') ? SubmissionServer.safeRead(ServerEnvironment.value('MAIL_TLS_KEY_FILE')) : '')
    const cert = ServerEnvironment.value('MAIL_TLS_CERT') || (ServerEnvironment.value('MAIL_TLS_CERT_FILE') ? SubmissionServer.safeRead(ServerEnvironment.value('MAIL_TLS_CERT_FILE')) : '')
    return key && cert ? { key, cert } : null
  }

  /**
   * @param {NodeJS.ReadableStream & { sizeExceeded?: boolean }} stream
   * @param {number} maxSize
   * @returns {Promise<Buffer>}
   */
  static readStream(stream, maxSize) {
    return new Promise((resolve, reject) => {
      /** @type {Buffer[]} */ const chunks = []
      let total = 0
      stream.on('data', (c) => { total += c.length; if (total <= maxSize) chunks.push(c) })
      stream.on('end', () => resolve(Buffer.concat(chunks)))
      stream.on('error', reject)
    })
  }

  /** @param {(err?: Error) => void} cb @param {number} code @param {string} message */
  static reject(cb, code, message) {
    const err = /** @type {Error & { responseCode?: number }} */ (new Error(message))
    err.responseCode = code
    cb(err)
  }
}
