/**
 * Automated Let's Encrypt (ACME) certificate provisioning for the mail server.
 *
 * Obtains and renews a TLS certificate for `MAIL_HOSTNAME` via the HTTP-01
 * challenge, writing the key/cert to the same `MAIL_TLS_KEY_FILE` /
 * `MAIL_TLS_CERT_FILE` paths the SMTP servers already read. The challenge is
 * answered by a tiny HTTP listener on port 80 (only while validating).
 *
 * The certificate replaces the manual "bring your own PEM" flow — nothing else
 * about the SMTP servers changes, and an SNI callback (see
 * {@link AcmeService.sniFromFiles}) lets them pick up a renewed cert on the next
 * TLS handshake without a restart.
 *
 * @typedef {object} AcmeOptions
 * @property {string} hostname     Certificate common name (MAIL_HOSTNAME).
 * @property {string} email        ACME account contact (MAIL_ACME_EMAIL).
 * @property {string} certPath     Where to write the fullchain PEM.
 * @property {string} keyPath      Where to write the private key PEM.
 * @property {string} accountPath  Where to persist the ACME account key.
 * @property {number} [httpPort]   Challenge listener port (default 80).
 * @property {boolean} [staging]   Use Let's Encrypt staging (MAIL_ACME_STAGING).
 */
export {}
