// @ts-check

/**
 * Reads one CADUCEUS setting by its environment name, such as `FYLO_ROOT`.
 *
 * A process that CADUCEUS starts itself (the entry point, the workers, the
 * tests) reads the variable directly. A Tachyon handler process receives no
 * application variables: Tachyon hands it the resolved `yon.*` configuration
 * from server/application.json and the project's `.env` as
 * `YON_APPLICATION_JSON` instead. There `FYLO_ROOT` is `yon.fylo.root`, which
 * `.env` supplies as `YON_FYLO_ROOT`.
 */
export class ServerEnvironment {
  /** The settings a handler process reads; the entry point hands them to Tachyon. */
  static HANDLER_SETTINGS = [
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

  /** Marks a `.env` that the entry point wrote, so it never replaces one a person wrote. */
  static GENERATED_MARKER = '# Written by scripts/entry.mjs from the process environment.'

  /** @type {Record<string, unknown> | undefined} */
  static #application = undefined

  /**
   * @param {string} name
   * @returns {string | undefined}
   */
  static value(name) {
    const direct = process.env[name]
    if (direct !== undefined) return direct
    return ServerEnvironment.#configured(name)
  }

  /**
   * The project `.env` that gives Tachyon's handler processes the settings
   * this process holds. Only single-line values can be written; a multi-line
   * value, such as a PEM key, has to come from its `_FILE` setting instead.
   *
   * @param {Record<string, string | undefined>} environment
   * @returns {{ text: string, skipped: string[] }}
   */
  static dotenv(environment) {
    const lines = [ServerEnvironment.GENERATED_MARKER]
    /** @type {string[]} */
    const skipped = []
    for (const name of ServerEnvironment.HANDLER_SETTINGS) {
      const value = environment[name]
      if (value === undefined || value === '') continue
      if (/[\r\n]/.test(value)) skipped.push(name)
      else lines.push(`YON_${name}=${value}`)
    }
    return { text: `${lines.join('\n')}\n`, skipped }
  }

  /**
   * @param {string} name
   * @returns {string | undefined}
   */
  static #configured(name) {
    /** @type {unknown} */
    let node = ServerEnvironment.#yon()
    for (const key of name.toLowerCase().split('_')) {
      if (!node || typeof node !== 'object') return undefined
      node = /** @type {Record<string, unknown>} */ (node)[key]
    }
    if (node === undefined || node === null || typeof node === 'object') return undefined
    return String(node)
  }

  /** @returns {Record<string, unknown>} */
  static #yon() {
    if (ServerEnvironment.#application === undefined) {
      ServerEnvironment.#application = ServerEnvironment.#parse(process.env.YON_APPLICATION_JSON)
    }
    return ServerEnvironment.#application
  }

  /**
   * @param {string | undefined} text
   * @returns {Record<string, unknown>}
   */
  static #parse(text) {
    if (!text) return {}
    try {
      const parsed = JSON.parse(text)
      return parsed && typeof parsed.yon === 'object' && parsed.yon ? parsed.yon : {}
    } catch {
      return {}
    }
  }
}
