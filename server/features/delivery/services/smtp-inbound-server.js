import { createHmac } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { AcmeService } from '../../domains/services/acme.js'
import { listenForConnections } from '../../../platform/runtime/listener.ts'

/**
 * Standalone inbound SMTP server (a real MX listener).
 *
 * This is the piece that makes CADUCEUS a true mail server: it listens for SMTP
 * connections from other mail servers on the internet, accepts messages for the
 * domains this deployment hosts, and hands each one to the *existing* inbound
 * pipeline through the private `/inbound/webhook` bridge.
 *
 * It is deliberately a thin SMTP↔HTTP bridge rather than talking to Fylo
 * directly: the API server (`ty serve`) stays the single owner of the data
 * store (no cross-process write races), and every message flows through the
 * already-tested delivery/spam/routing/rules path. No third-party mail service
 * is required.
 *
 * @typedef {object} InboundSmtpOptions
 * @property {number} [port]      Listen port (default 25 / SMTP_INBOUND_PORT).
 * @property {string} [host]      Bind address (default 0.0.0.0).
 * @property {string} [apiUrl]    Base URL of the local API server.
 * @property {string} [secret]    INBOUND_WEBHOOK_SECRET for signing.
 * @property {string} [hostname]  SMTP banner hostname (MAIL_HOSTNAME).
 * @property {number} [maxSize]   Max message size in bytes.
 * @property {{ key: string, cert: string }} [tls] STARTTLS key/cert (PEM).
 * @property {boolean} [verifyAuth] Run SPF/DKIM/DMARC verification (default on; MAIL_AUTH_VERIFY=off disables).
 */

export class InboundSmtpServer {
  /** @param {InboundSmtpOptions} [options] */
  constructor(options = {}) {
    this.port = Number(options.port ?? process.env.SMTP_INBOUND_PORT ?? 25)
    this.host = options.host ?? process.env.SMTP_INBOUND_HOST ?? '0.0.0.0'
    this.apiUrl = (options.apiUrl ?? process.env.CADUCEUS_API_URL ?? `http://127.0.0.1:${process.env.PORT || 8080}`).replace(/\/$/, '')
    this.secret = options.secret ?? process.env.INBOUND_WEBHOOK_SECRET
    this.hostname = options.hostname ?? process.env.MAIL_HOSTNAME ?? 'caduceus'
    this.maxSize = Number(options.maxSize ?? process.env.SMTP_MAX_SIZE ?? 26_214_400) // 25 MiB
    this.tls = options.tls ?? InboundSmtpServer.loadTls()
    this.verifyAuth = options.verifyAuth ?? process.env.MAIL_AUTH_VERIFY !== 'off'

    if (!this.secret) throw new Error('INBOUND_WEBHOOK_SECRET is required to run the inbound SMTP server')
  }

  /**
   * Start the inbound SMTP server. Resolves once it is listening.
   *
   * @returns {Promise<import('smtp-server').SMTPServer>}
   */
  async start() {
    const { SMTPServer } = await import('smtp-server')
    const { simpleParser } = await import('mailparser')

    // Serve a fresh context per handshake when TLS lives in files, so ACME
    // renewals apply without a restart.
    const keyFile = process.env.MAIL_TLS_KEY_FILE, certFile = process.env.MAIL_TLS_CERT_FILE
    const sni = this.tls && keyFile && certFile ? AcmeService.sniFromFiles(keyFile, certFile) : null

    const server = new SMTPServer({
      name: this.hostname,
      banner: `${this.hostname} ESMTP CADUCEUS`,
      size: this.maxSize,
      // A public MX accepts mail without authenticating the sending server;
      // message-submission (authenticated) is a separate concern on 587/465.
      authOptional: true,
      disabledCommands: ['AUTH'],
      ...(this.tls ? { key: this.tls.key, cert: this.tls.cert } : {}),
      ...(sni ? { SNICallback: sni } : {}),
      hideSTARTTLS: !this.tls,

      onData: (stream, session, callback) => {
        InboundSmtpServer.readStream(stream, this.maxSize)
          .then(async (raw) => {
            const failure = await this.processMessage(raw, session, Boolean(stream.sizeExceeded), simpleParser)
            if (failure) return InboundSmtpServer.reject(callback, failure.code, failure.message)
            callback()
          })
          .catch((err) => {
            console.error('[smtp-in] delivery error:', err)
            InboundSmtpServer.reject(callback, 451, 'Temporary local delivery failure, try again later')
          })
      },
    })

    server.on('error', (err) => console.error('[smtp-in] server error:', err))

    await listenForConnections(server, this.port, this.host)
    console.log(`[smtp-in] inbound SMTP server listening on ${this.host}:${this.port} → ${this.apiUrl}/inbound/webhook (STARTTLS ${this.tls ? 'on' : 'off'})`)
    return server
  }

  /**
   * Authenticate one received message and fan it out to every hosted
   * recipient. Split out of the SMTPServer `onData` closure so the delivery
   * decisions can be exercised without opening a socket.
   *
   * @param {Buffer} raw
   * @param {import('smtp-server').SMTPServerSession} session
   * @param {boolean} sizeExceeded
   * @param {(raw: Buffer) => Promise<import('mailparser').ParsedMail>} simpleParser
   * @returns {Promise<{ code: number, message: string } | null>} SMTP failure, or null on success.
   */
  async processMessage(raw, session, sizeExceeded, simpleParser) {
    if (sizeExceeded) return { code: 552, message: 'Message exceeds maximum permitted size' }

    const parsed = await simpleParser(raw).catch(() => null)
    const subject = parsed?.subject || '(no subject)'
    const envelopeFrom = session.envelope.mailFrom ? session.envelope.mailFrom.address : ''
    const sender = envelopeFrom || parsed?.from?.value?.[0]?.address || ''

    // Verify SPF/DKIM/DMARC and prepend the Authentication-Results header so
    // downstream tooling (and the stored message) can see the result.
    const auth = this.verifyAuth
      ? await this.authenticateMessage(raw, session)
      : { headerText: '', summary: 'auth=off' }
    const rawText = auth.headerText + raw.toString('utf8')

    // Deliver a copy to each hosted recipient. The pipeline generates a
    // distinct stored id per recipient, so shared Message-IDs never clash.
    let anyHosted = false
    for (const { address: recipient } of session.envelope.rcptTo) {
      const result = await this.deliverViaWebhook({
        recipient, sender, subject, body: rawText,
        ...(auth.spamVerdict ? { spamVerdict: auth.spamVerdict } : {}),
        ...(auth.spamScore ? { spamScore: auth.spamScore } : {}),
      })
      if (!result.ok) return { code: 451, message: 'Temporary local delivery failure, try again later' }
      // "domain not configured" means we do not host this recipient — a real MX
      // should refuse rather than silently blackhole it.
      if (result.accepted || result.reason !== 'domain not configured') anyHosted = true
    }
    return anyHosted ? null : { code: 550, message: 'No such user here' }
  }

  /**
   * POST a parsed inbound message to the local `/inbound/webhook`, signed with
   * the shared HMAC secret, and return the delivery pipeline's decision.
   *
   * @param {{ recipient: string, sender: string, subject: string, body: string, spamVerdict?: string, spamScore?: number }} payload
   * @returns {Promise<{ ok: boolean, accepted: boolean, reason?: string }>}
   */
  async deliverViaWebhook(payload) {
    const json = JSON.stringify(payload)
    const signature = createHmac('sha256', this.secret).update(json).digest('hex')
    const res = await fetch(`${this.apiUrl}/inbound/webhook`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Caduceus-Signature': signature },
      body: json,
    })
    if (!res.ok) return { ok: false, accepted: false, reason: `webhook ${res.status}` }
    const data = /** @type {{ accepted?: boolean, reason?: string }} */ (await res.json().catch(() => ({})))
    return { ok: true, accepted: data.accepted !== false, reason: data.reason }
  }

  /**
   * Verify SPF, DKIM, and DMARC for an inbound message and derive a spam signal
   * the delivery pipeline understands. Returns the `Authentication-Results`
   * header block to prepend to the message, plus an optional verdict/score:
   * a DMARC failure is treated as an authenticated spoof (spam), and a hard SPF
   * failure on an unsigned message adds suspicion.
   *
   * @param {Buffer} raw
   * @param {import('smtp-server').SMTPServerSession} session
   * @param {((raw: Buffer, opts: object) => Promise<any>) | undefined} [verifier]
   *   Override for `mailauth`'s `authenticate`. It performs live DNS lookups,
   *   so tests supply their own.
   * @returns {Promise<{ headerText: string, spamVerdict?: string, spamScore?: number, summary: string }>}
   */
  async authenticateMessage(raw, session, verifier) {
    try {
      const authenticate = verifier ?? (await import('mailauth')).authenticate
      const result = await authenticate(raw, {
        ip: session.remoteAddress,
        helo: session.hostNameAppearsAs || session.clientHostname || undefined,
        sender: session.envelope.mailFrom ? session.envelope.mailFrom.address : undefined,
        mta: this.hostname,
      })
      const spf = result?.spf && result.spf.status?.result
      const dmarc = result?.dmarc && result.dmarc.status?.result
      const dkimPass = (result?.dkim?.results || []).some((r) => r?.status?.result === 'pass')

      let spamVerdict, spamScore
      if (dmarc === 'fail') { spamVerdict = 'fail'; spamScore = 6 }        // authenticated spoof
      else if (spf === 'fail' && !dkimPass) { spamScore = 3 }              // hard SPF fail, unsigned

      return {
        headerText: result?.headers || '',
        spamVerdict, spamScore,
        summary: `spf=${spf ?? 'n/a'} dkim=${dkimPass ? 'pass' : 'fail/none'} dmarc=${dmarc ?? 'n/a'}`,
      }
    } catch (err) {
      // Never fail delivery because verification errored (e.g. DNS timeout).
      console.error('[smtp-in] mailauth verification failed (delivering unverified):', err instanceof Error ? err.message : err)
      return { headerText: '', summary: 'auth=error' }
    }
  }

  /** Read STARTTLS material from env (paths or inline PEM). Returns null if none. */
  static loadTls() {
    const keyPath = process.env.MAIL_TLS_KEY_FILE
    const certPath = process.env.MAIL_TLS_CERT_FILE
    const key = process.env.MAIL_TLS_KEY || (keyPath ? InboundSmtpServer.safeRead(keyPath) : '')
    const cert = process.env.MAIL_TLS_CERT || (certPath ? InboundSmtpServer.safeRead(certPath) : '')
    return key && cert ? { key, cert } : null
  }

  /** @param {string} path */
  static safeRead(path) {
    try { return readFileSync(path, 'utf8') } catch { return '' }
  }

  /**
   * Read a message stream into a Buffer, enforcing a hard size cap.
   *
   * @param {NodeJS.ReadableStream & { sizeExceeded?: boolean }} stream
   * @param {number} maxSize
   * @returns {Promise<Buffer>}
   */
  static readStream(stream, maxSize) {
    return new Promise((resolve, reject) => {
      /** @type {Buffer[]} */
      const chunks = []
      let total = 0
      stream.on('data', (chunk) => {
        total += chunk.length
        if (total <= maxSize) chunks.push(chunk)
      })
      stream.on('end', () => resolve(Buffer.concat(chunks)))
      stream.on('error', reject)
    })
  }

  /** @param {(err?: Error) => void} callback @param {number} code @param {string} message */
  static reject(callback, code, message) {
    const err = /** @type {Error & { responseCode?: number }} */ (new Error(message))
    err.responseCode = code
    callback(err)
  }
}
