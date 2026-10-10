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
export {}
