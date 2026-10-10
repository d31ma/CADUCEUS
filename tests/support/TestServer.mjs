import { createServer } from 'node:net'
import { rmSync, readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

/**
 * Runs the application under `ty serve` for the suites, so they exercise the
 * server through HTTP whatever language it is written in. The server's
 * settings go to the project `.env` as YON_ names, the way the entry point
 * hands them over. CADUCEUS_SERVE_ROOT points at another project directory
 * (for example a copy whose client builds), TACHYON_BIN at the ty to use.
 */
export class TestServer {
  static ROOT = resolve(process.env.CADUCEUS_SERVE_ROOT || join(import.meta.dirname, '..', '..'))
  static MARKER = '# Written by scripts/entry.mjs from the process environment.'
  static STARTUP_MS = Number(process.env.TEST_SERVER_START_TIMEOUT_MS) || 900_000

  /** The settings a handler reads; nothing else from the test process reaches the server. */
  static SETTINGS = [
    'ATTACHMENT_ROOT',
    'CADUCEUS_API_URL',
    'CADUCEUS_ENABLE_TEST_ROUTES',
    'CADUCEUS_PUBLIC_URL',
    'CADUCEUS_TEST_REAL_TRANSLATION',
    'DKIM_DOMAIN',
    'DKIM_PRIVATE_KEY',
    'DKIM_PRIVATE_KEY_FILE',
    'DKIM_SELECTOR',
    'FYLO_BINARY',
    'FYLO_BRIDGE_ADDRESS',
    'FYLO_BRIDGE_SECRET',
    'FYLO_EXEC_PATH',
    'FYLO_INCOMING_ROOT',
    'FYLO_POSIX_ACCESS',
    'FYLO_ROOT',
    'INBOUND_WEBHOOK_SECRET',
    'MAIL_AUTH_VERIFY',
    'MAIL_HOSTNAME',
    'MAIL_TLS_CERT',
    'MAIL_TLS_CERT_FILE',
    'MAIL_TLS_KEY',
    'MAIL_TLS_KEY_FILE',
    'NODE_ENV',
    'SESAME_ISSUER',
    'SMTP_ADAPTER',
    'SMTP_FROM',
    'SMTP_INBOUND_HOST',
    'SMTP_INBOUND_PORT',
    'SMTP_MAX_SIZE',
    'SMTP_SUBMISSION_HOST',
    'SMTP_TLS_POLICY',
    'SPAM_FILTER_DISABLED',
    'SPAM_SCORE_THRESHOLD',
    'STREAM_MAX_MS',
    'STREAM_POLL_MS',
    'STREAM_REPLAY_LIMIT',
    'TRACKING_SECRET',
    'TRANSLATION_ASSET_ROOT',
    'VAPID_PRIVATE_KEY',
    'VAPID_PUBLIC_KEY',
    'VAPID_SUBJECT',
    'WEB_PUSH_DISABLED',
  ]

  /** @type {{ url: string, stop: () => void } | null} */
  static current = null

  /**
   * @param {Record<string, string | undefined>} settings
   * @returns {Promise<{ url: string, stop: () => void }>}
   */
  static async start(settings) {
    TestServer.stop()
    const envFile = join(TestServer.ROOT, '.env')
    TestServer.#refuseForeignEnv(envFile)
    const lines = [TestServer.MARKER]
    for (const name of TestServer.SETTINGS) {
      const value = settings[name]
      if (value !== undefined && value !== '' && !/[\r\n]/.test(value)) lines.push(`YON_${name}=${value}`)
    }
    writeFileSync(envFile, `${lines.join('\n')}\n`, { mode: 0o600 })
    const port = await TestServer.#freePort()
    const proc = Bun.spawn([process.env.TACHYON_BIN || 'ty', 'serve', '--no-watch', '--port', String(port)], {
      cwd: TestServer.ROOT,
      env: { PATH: process.env.PATH, HOME: process.env.HOME },
      stdout: 'ignore',
      stderr: process.env.TEST_SERVER_LOG ? 'inherit' : 'ignore',
    })
    const url = `http://127.0.0.1:${port}`
    const stop = () => {
      proc.kill()
      rmSync(envFile, { force: true })
      if (TestServer.current?.url === url) TestServer.current = null
    }
    await TestServer.#waitUntilReady(url, stop)
    TestServer.current = { url, stop }
    return TestServer.current
  }

  static stop() {
    TestServer.current?.stop()
  }

  /** @param {string} envFile */
  static #refuseForeignEnv(envFile) {
    let existing = null
    try { existing = readFileSync(envFile, 'utf8') } catch {}
    if (existing !== null && !existing.startsWith(TestServer.MARKER)) {
      throw new Error(`${envFile} was not written by CADUCEUS; move it aside before running the suites`)
    }
  }

  /** @param {string} url @param {() => void} stop */
  static async #waitUntilReady(url, stop) {
    const deadline = Date.now() + TestServer.STARTUP_MS
    while (Date.now() < deadline) {
      try {
        // A @SignedIn route answers 401 once the application is built, so
        // the frontend fallback cannot answer the first test.
        const response = await fetch(`${url}/domains`, { signal: AbortSignal.timeout(1_000) })
        if (response.status === 401) return
      } catch {}
      await Bun.sleep(250)
    }
    stop()
    throw new Error(`Test server did not become ready at ${url}`)
  }

  /** @returns {Promise<number>} */
  static #freePort() {
    return new Promise((resolvePort, reject) => {
      const reservation = createServer()
      reservation.unref()
      reservation.once('error', reject)
      reservation.listen(0, '127.0.0.1', () => {
        const address = reservation.address()
        const port = typeof address === 'object' && address ? address.port : 0
        reservation.close(error => (error ? reject(error) : resolvePort(port)))
      })
    })
  }
}
