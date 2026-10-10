import { createServer } from 'node:http'
import { readFileSync, writeFileSync, mkdirSync, existsSync, renameSync } from 'node:fs'
import { dirname } from 'node:path'
import { X509Certificate } from 'node:crypto'
import { createSecureContext } from 'node:tls'

const RENEW_BEFORE_MS = 30 * 24 * 60 * 60 * 1000 // renew when <30 days remain
const DAILY_MS = 24 * 60 * 60 * 1000

export class AcmeService {
  /** @param {AcmeOptions} options */
  constructor(options) {
    if (!options.hostname || !options.email) {
      throw new Error('MAIL_HOSTNAME and MAIL_ACME_EMAIL are required for ACME')
    }
    this.hostname = options.hostname
    this.email = options.email
    this.certPath = options.certPath
    this.keyPath = options.keyPath
    this.accountPath = options.accountPath
    this.httpPort = options.httpPort ?? 80
    this.staging = options.staging ?? false
    /** @type {ReturnType<typeof setInterval> | undefined} */
    this.renewalTimer = undefined
  }

  /**
   * Ensure a currently-valid certificate exists at `certPath`/`keyPath`,
   * obtaining or renewing via ACME HTTP-01 if it is missing or within the
   * renewal window.
   *
   * @param {typeof import('acme-client')} [acmeModule] Override for the ACME
   *   client. Issuance talks to Let's Encrypt over the network, so tests
   *   supply their own.
   * @returns {Promise<{ obtained: boolean, reason?: string }>}
   */
  async ensureCertificate(acmeModule) {
    if (AcmeService.certMsRemaining(this.certPath) > RENEW_BEFORE_MS) {
      return { obtained: false, reason: 'cert still valid' }
    }

    const acme = acmeModule ?? await import('acme-client')
    const accountKey = existsSync(this.accountPath)
      ? readFileSync(this.accountPath)
      : await acme.crypto.createPrivateKey()
    if (!existsSync(this.accountPath)) AcmeService.writeAtomic(this.accountPath, accountKey)

    const client = new acme.Client({
      directoryUrl: this.staging ? acme.directory.letsencrypt.staging : acme.directory.letsencrypt.production,
      accountKey,
    })

    const [key, csr] = await acme.crypto.createCsr({ commonName: this.hostname })

    /** @type {Map<string,string>} token → keyAuthorization */
    const tokens = new Map()
    const challengeServer = createServer((req, res) => {
      const token = (req.url || '').split('/').pop() || ''
      const value = tokens.get(token)
      if (value) { res.writeHead(200, { 'Content-Type': 'text/plain' }); res.end(value) }
      else { res.writeHead(404); res.end() }
    })
    await new Promise((resolve, reject) => {
      challengeServer.once('error', reject)
      challengeServer.listen(this.httpPort, () => resolve(undefined))
    })

    try {
      const cert = await client.auto({
        csr,
        email: this.email,
        termsOfServiceAgreed: true,
        challengePriority: ['http-01'],
        challengeCreateFn: async (_authz, challenge, keyAuthorization) => {
          tokens.set(challenge.token, keyAuthorization)
        },
        challengeRemoveFn: async (_authz, challenge) => {
          tokens.delete(challenge.token)
        },
      })
      AcmeService.writeAtomic(this.keyPath, key)
      AcmeService.writeAtomic(this.certPath, cert)
      console.log(`[acme] obtained certificate for ${this.hostname} (${this.staging ? 'staging' : 'production'})`)
      return { obtained: true }
    } finally {
      challengeServer.close()
    }
  }

  /**
   * Provision the certificate now and re-check daily, renewing inside the
   * window. The interval is unref'd so it never keeps the process alive.
   *
   * @returns {Promise<void>}
   */
  async startRenewal() {
    if (this.renewalTimer) return
    await this.ensureCertificate()
    this.renewalTimer = setInterval(() => {
      this.ensureCertificate().catch((err) =>
        console.error('[acme] renewal check failed:', err instanceof Error ? err.message : err),
      )
    }, DAILY_MS)
    this.renewalTimer.unref()
  }

  /** Stop future renewal checks. Safe to call more than once. */
  stop() {
    if (!this.renewalTimer) return
    clearInterval(this.renewalTimer)
    this.renewalTimer = undefined
  }

  /**
   * Build an SNI callback that serves a fresh secure context from the cert files
   * on every handshake, so renewals apply without restarting the SMTP servers.
   *
   * @param {string} keyPath @param {string} certPath
   */
  static sniFromFiles(keyPath, certPath) {
    // ponytail: rebuild the context per handshake — cheap next to the TLS
    // handshake itself; add a cache keyed on file mtime only if it ever shows up.
    return (/** @type {string} */ _servername, /** @type {(e: Error|null, ctx?: import('tls').SecureContext) => void} */ cb) => {
      try {
        cb(null, createSecureContext({ key: readFileSync(keyPath), cert: readFileSync(certPath) }))
      } catch (err) {
        cb(err instanceof Error ? err : new Error(String(err)))
      }
    }
  }

  /** Milliseconds until the cert at `certPath` expires (-Infinity if missing/unreadable). @param {string} certPath */
  static certMsRemaining(certPath) {
    try {
      return new Date(new X509Certificate(readFileSync(certPath)).validTo).getTime() - Date.now()
    } catch {
      return -Infinity
    }
  }

  /** Write a file atomically (tmp + rename), creating parent dirs. @param {string} path @param {string|Buffer} data */
  static writeAtomic(path, data) {
    mkdirSync(dirname(path), { recursive: true })
    const tmp = `${path}.tmp-${process.pid}`
    writeFileSync(tmp, data, { mode: 0o600 })
    renameSync(tmp, path)
  }
}
