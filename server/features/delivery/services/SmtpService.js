import { readFileSync } from 'node:fs'
import { prepareTrackedHtml } from './tracking-tokens.js'
import { ServerEnvironment } from '../../../platform/runtime/ServerEnvironment.js'
/** @import { SendRequest } from '../../../models/SendRequest.js' */
/** @import { SmtpAdapter } from '../../../models/SmtpAdapter.js' */

/** Outbound mail delivery for the self-hosted server. */
export class SmtpService {
  constructor() {
    /** @type {SmtpAdapter | null} */
    this._adapter = null
  }

  /** @returns {SmtpAdapter} */
  getAdapter() {
    if (!this._adapter) this._adapter = createSmtpAdapter()
    return this._adapter
  }

  /** @param {SendRequest} msg */
  async send(msg) {
    return await this.getAdapter().send(msg)
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    return await this.getAdapter().sendEmail(from, msg)
  }
}

/**
 * Production always delivers directly to recipient MX servers. The console
 * adapter exists only for local development and automated tests.
 *
 * @returns {SmtpAdapter}
 */
export function createSmtpAdapter() {
  const mode = ServerEnvironment.value('SMTP_ADAPTER') || (ServerEnvironment.value('NODE_ENV') === 'production' ? 'direct' : 'console')
  if (mode === 'direct') return new SmtpDirectAdapter()
  if (mode === 'console') return new ConsoleSmtpAdapter()
  throw new Error(`Unsupported SMTP_ADAPTER=${mode}. Use direct or console.`)
}

export class ConsoleSmtpAdapter {
  /** @param {SendRequest} msg */
  async send(msg) {
    console.log(`[smtp] to=${msg.to.join(', ')} subject="${msg.subject}" attachments=${msg.attachments?.length ?? 0}`)
    return { messageId: msg.messageId || `console-${Date.now()}` }
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    console.log(`[smtp] from=${from} to=${msg.to.join(', ')} subject="${msg.subject}" attachments=${msg.attachments?.length ?? 0}`)
    return { messageId: msg.messageId || `console-${Date.now()}` }
  }
}

/** Direct-to-MX delivery without an email relay or cloud mail provider. */
export class SmtpDirectAdapter {
  /**
   * @param {object} [deps] Transport seams. Delivery opens real SMTP
   *   connections to remote MX hosts, so tests supply their own.
   * @param {(options: object) => { sendMail: (message: object) => Promise<any> }} [deps.createTransport]
   * @param {(domain: string) => Promise<string[]>} [deps.resolveMx]
   */
  constructor(deps = {}) {
    this.deps = deps
  }

  /** @param {string} from @param {SendRequest} msg */
  async sendEmail(from, msg) {
    return this.send({ ...msg, replyTo: msg.replyTo ?? [from] }, from)
  }

  /** @param {SendRequest} msg @param {string} [fromOverride] */
  async send(msg, fromOverride) {
    const hostname = ServerEnvironment.value('MAIL_HOSTNAME') || 'localhost'
    const from = fromOverride || ServerEnvironment.value('SMTP_FROM') || `postmaster@${hostname}`
    /** @type {(options: any) => { sendMail: (message: any) => Promise<any> }} */
    const createTransport = this.deps.createTransport
      ?? /** @type {any} */ ((await import('../../../deps/npm/nodemailer.mjs')).default.createTransport)
    const resolveMx = this.deps.resolveMx ?? resolveMailExchangers
    const dkim = loadDkim()
    const recipients = uniqueRecipients(msg.envelopeRecipients || [...(msg.to || []), ...(msg.cc || []), ...(msg.bcc || [])])
    const tracked = Boolean(msg.trackingId && (msg.openTracking || msg.linkTracking))
    const deliveries = deliveryPlan(recipients, tracked)
    const results = []
    for (const delivery of deliveries) {
      results.push(await deliverToDomain({ delivery, from, msg, hostname, dkim, createTransport, resolveMx }))
    }
    const failed = results.filter(result => result.failedRecipients.length)
    if (failed.length) throw new SmtpDeliveryError(results)
    const succeeded = results.filter(result => result.deliveredRecipients.length)
    return { messageId: String(succeeded[0].messageId || `direct-${Date.now()}`), results }
  }
}

export class SmtpDeliveryError extends Error {
  /** @param {Array<Record<string,any>>} results */
  constructor(results) {
    const failed = results.filter(result => result.failedRecipients?.length)
    super(`Direct SMTP delivery failed — ${failed.map(result => `${result.domain}: ${result.error || 'recipient rejected'}`).join('; ')}`)
    this.name = 'SmtpDeliveryError'
    this.deliveryResults = results
    this.deliveredRecipients = uniqueRecipients(results.flatMap(result => result.deliveredRecipients || []))
    this.failedRecipients = uniqueRecipients(failed.flatMap(result => result.failedRecipients || []))
    const failures = failed.flatMap(recipientFailures)
    this.responseCode = failures.every(failure => permanentReply(failure.responseCode)) ? 550 : 451
  }
}

/** @param {string[]} recipients @param {boolean} tracked */
function deliveryPlan(recipients, tracked) {
  return tracked
    ? recipients.map(recipient => ({ domain: recipientDomain(recipient), recipients: [recipient], trackedRecipient: recipient }))
    : [...groupRecipientsByDomain(recipients)].map(([domain, grouped]) => ({ domain, recipients: grouped }))
}

/** @param {string} address */
function recipientDomain(address) { return String(address).split('@')[1]?.toLowerCase() || '' }

/** @param {{delivery:Record<string,any>,from:string,msg:SendRequest,hostname:string,dkim:any,createTransport:Function,resolveMx:Function}} input */
async function deliverToDomain(input) {
  const { delivery, from, msg, hostname, dkim, createTransport, resolveMx } = input
  if (!delivery.domain) return failedDelivery(delivery, null)
  let hosts
  try { hosts = await resolveMx(delivery.domain) } catch (error) { return failedDelivery(delivery, error) }
  let lastError
  for (const host of hosts) {
    try {
      const transporter = createTransport(transportOptions(host, hostname, dkim))
      const message = buildSmtpMessage(from, msg, delivery.trackedRecipient)
      const info = await transporter.sendMail({ ...message, envelope: { from, to: delivery.recipients } })
      return acceptedDelivery(delivery, host, info)
    } catch (error) { lastError = error }
  }
  return failedDelivery(delivery, lastError)
}

/** @param {string} host @param {string} hostname @param {any} dkim */
export function transportOptions(host, hostname, dkim) {
  const policy = ServerEnvironment.value('SMTP_TLS_POLICY') || 'opportunistic'
  if (policy !== 'opportunistic' && policy !== 'require') throw Object.assign(new Error('SMTP_TLS_POLICY must be opportunistic or require'), { responseCode: 550 })
  const requireTLS = policy === 'require'
  return {
    host, port: 25, secure: false, name: hostname,
    requireTLS, opportunisticTLS: !requireTLS,
    tls: { servername: host, rejectUnauthorized: requireTLS },
    connectionTimeout: 20_000, greetingTimeout: 20_000, socketTimeout: 30_000,
    ...(dkim ? { dkim } : {}),
  }
}

/** @param {Record<string,any>} delivery @param {string} host @param {Record<string,any>} info */
function acceptedDelivery(delivery, host, info) {
  const accepted = normalizedAccepted(info.accepted, delivery.recipients)
  const acceptedSet = new Set(accepted.map(value => value.toLowerCase()))
  const failed = delivery.recipients.filter(recipient => !acceptedSet.has(recipient.toLowerCase()))
  const failures = failed.map(recipient => rejectedRecipient(recipient, failed, info.rejectedErrors))
  const rejectedCode = failures.every(failure => permanentReply(failure.responseCode)) ? 550 : 451
  return {
    domain: delivery.domain, recipients: delivery.recipients, deliveredRecipients: accepted, failedRecipients: failed,
    ...(delivery.trackedRecipient ? { recipient: delivery.trackedRecipient } : {}), host,
    messageId: info.messageId, accepted: info.accepted, rejected: info.rejected, response: info.response,
    ...(failed.length ? { error: 'Remote server rejected one or more recipients.', responseCode: rejectedCode, recipientFailures: failures } : {}),
  }
}

/** @param {string} recipient @param {string[]} failed @param {unknown} rejectedErrors */
function rejectedRecipient(recipient, failed, rejectedErrors) {
  const errors = Array.isArray(rejectedErrors) ? rejectedErrors : []
  const matched = errors.find(error => String(error?.recipient || '').toLowerCase() === recipient.toLowerCase())
    || (failed.length === 1 && errors.length === 1 ? errors[0] : undefined)
  return {
    recipient,
    responseCode: Number(matched?.responseCode) || 451,
    error: matched instanceof Error ? safeSmtpError(matched.message) : 'recipient rejected',
  }
}

/** @param {Record<string, any>} result */
function recipientFailures(result) {
  if (Array.isArray(result.recipientFailures) && result.recipientFailures.length) return result.recipientFailures
  return (result.failedRecipients || []).map(recipient => ({ recipient, responseCode: result.responseCode }))
}

/** @param {unknown} accepted @param {string[]} requested */
function normalizedAccepted(accepted, requested) {
  if (!Array.isArray(accepted)) return requested
  const allowed = new Set(requested.map(value => value.toLowerCase()))
  return uniqueRecipients(accepted.map(String).filter(value => allowed.has(value.toLowerCase())))
}

/** @param {Record<string,any>} delivery @param {unknown} error */
function failedDelivery(delivery, error) {
  return {
    domain: delivery.domain, recipients: delivery.recipients, deliveredRecipients: [], failedRecipients: delivery.recipients,
    ...(delivery.trackedRecipient ? { recipient: delivery.trackedRecipient } : {}),
    error: error instanceof Error ? safeSmtpError(error.message) : 'delivery failed',
    responseCode: Number(/** @type {any} */ (error)?.responseCode) || 451,
  }
}

/** @param {unknown} value */
function permanentReply(value) { const code = Number(value); return code >= 500 && code < 600 }

/** @param {unknown} value */
function safeSmtpError(value) { return String(value || 'delivery failed').replace(/[\r\n\0]+/g, ' ').slice(0, 500) }

/** @returns {{ domainName: string, keySelector: string, privateKey: string } | undefined} */
export function loadDkim() {
  const domainName = ServerEnvironment.value('DKIM_DOMAIN')
  const keySelector = ServerEnvironment.value('DKIM_SELECTOR')
  let privateKey = ServerEnvironment.value('DKIM_PRIVATE_KEY') || ''
  if (!privateKey && ServerEnvironment.value('DKIM_PRIVATE_KEY_FILE')) {
    try { privateKey = readFileSync(ServerEnvironment.value('DKIM_PRIVATE_KEY_FILE'), 'utf8') } catch { /* optional */ }
  }
  return domainName && keySelector && privateKey ? { domainName, keySelector, privateKey } : undefined
}

/** @param {string[]} addresses @returns {Map<string, string[]>} */
export function groupRecipientsByDomain(addresses) {
  /** @type {Map<string, string[]>} */
  const byDomain = new Map()
  for (const address of addresses) {
    const domain = String(address).split('@')[1]?.toLowerCase()
    if (!domain) continue
    const recipients = byDomain.get(domain) ?? []
    recipients.push(address)
    byDomain.set(domain, recipients)
  }
  return byDomain
}

/** @param {string[]} addresses */
function uniqueRecipients(addresses) {
  const seen = new Set()
  return addresses.filter(address => {
    const normalized = String(address || '').trim().toLowerCase()
    if (!normalized || seen.has(normalized)) return false
    seen.add(normalized)
    return true
  })
}

/** @param {string} domain @returns {Promise<string[]>} */
export async function resolveMailExchangers(domain, resolver) {
  if (!resolver) ({ resolveMx: resolver } = await import('node:dns/promises'))
  try {
    const records = await resolver(domain)
    if (records.length > 0) {
      if (records.some(record => record.exchange === '.')) {
        throw Object.assign(new Error('Domain publishes a null MX and does not accept email.'), { responseCode: 556, nullMx: true })
      }
      return records.sort((a, b) => a.priority - b.priority).map(record => record.exchange)
    }
  } catch (error) {
    if (/** @type {any} */ (error)?.nullMx) throw error
    const code = String(/** @type {any} */ (error)?.code || '')
    if (code && code !== 'ENODATA' && code !== 'ENOTFOUND') throw Object.assign(new Error('MX lookup temporarily failed.'), { responseCode: 451 })
  }
  return [domain]
}

/** @param {string} from @param {SendRequest} msg @param {string} [trackedRecipient] */
export function buildSmtpMessage(from, msg, trackedRecipient) {
  const renderedHtml = trackedRecipient && msg.trackingId && (msg.openTracking || msg.linkTracking)
    ? prepareTrackedHtml(msg.html || '', {
      trackingId: msg.trackingId,
      recipient: trackedRecipient,
      openTracking: msg.openTracking,
      linkTracking: msg.linkTracking,
    }).html
    : msg.html
  return {
    from,
    to: msg.to.join(', '),
    cc: msg.cc?.join(', '),
    bcc: msg.bcc?.join(', '),
    subject: msg.subject,
    text: msg.text,
    html: renderedHtml,
    replyTo: msg.replyTo?.join(', '),
    messageId: msg.messageId,
    inReplyTo: msg.inReplyTo,
    references: msg.references,
    attachments: msg.attachments?.map(attachment => ({
      filename: attachment.filename,
      content: Buffer.from(attachment.contentBase64, 'base64'),
      contentType: attachment.contentType,
      cid: attachment.contentId,
    })),
  }
}
