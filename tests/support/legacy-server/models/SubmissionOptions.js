/**
 * SMTP message-submission server (ports 587 / 465).
 *
 * This is what lets standard mail clients (Thunderbird, Apple Mail, Outlook)
 * send *through* CADUCEUS. Unlike the inbound MX (which accepts mail from the
 * world without auth), submission REQUIRES authentication and TLS: the client
 * logs in with their CADUCEUS email + password, and the accepted message is
 * pushed through the exact same `POST /send` pipeline the webmail uses —
 * rate-limiting, suppression list, "Sent" copy, DKIM signing, and the outbound
 * retry/bounce queue. Fully decoupled from the data store: it submits via
 * `/send`, so the API server stays the single writer.
 *
 * Authentication is currently refused. SMTP AUTH carries exactly one secret,
 * and since Phase 3 a password alone cannot produce a session: every login
 * must reach multi-factor assurance. The answer is an application password
 * bound to a SESAME workload principal holding only `mailbox:send` for one
 * mailbox — the same shape Gmail and Fastmail use — and that is Phase 6 work,
 * where submission is owned. Until then this refuses with an accurate reason
 * rather than weakening the policy for one protocol or calling a route that
 * no longer exists.
 *
 * @typedef {object} SubmissionOptions
 * @property {number} [port]     Listen port (587 for STARTTLS, 465 for implicit TLS).
 * @property {boolean} [secure]  true → implicit TLS (465); false → STARTTLS (587).
 * @property {string} [host]     Bind address (default 0.0.0.0).
 * @property {string} [apiUrl]   Base URL of the local API server.
 * @property {string} [hostname] SMTP banner hostname (MAIL_HOSTNAME).
 * @property {number} [maxSize]  Max message size in bytes.
 * @property {{ key: string, cert: string }} [tls] TLS key/cert (PEM). Required for 465; STARTTLS for 587.
 * @property {boolean} [allowInsecureAuth] Permit AUTH without TLS. Dev/testing only — never enable in production.
 */
export {}
