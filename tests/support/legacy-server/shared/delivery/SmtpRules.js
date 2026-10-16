/** @import { SmtpAdapter } from '../../models/SmtpAdapter.js' */
/** @import { SendRequest } from '../../models/SendRequest.js' */
import { readFileSync } from 'node:fs'
import { TrackingTokens } from './TrackingTokens.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'
import { SmtpDirectAdapter } from './SmtpDirectAdapter.js'
import { ConsoleSmtpAdapter } from './ConsoleSmtpAdapter.js'

export class SmtpRules {
  /**
   * Production always delivers directly to recipient MX servers. The console
   * adapter exists only for local development and automated tests.
   *
   * @returns {SmtpAdapter}
   */
  static createSmtpAdapter() {
    const mode = ServerEnvironment.value('SMTP_ADAPTER') || (ServerEnvironment.value('NODE_ENV') === 'production' ? 'direct' : 'console')
    if (mode === 'direct') return new SmtpDirectAdapter()
    if (mode === 'console') return new ConsoleSmtpAdapter()
    throw new Error(`Unsupported SMTP_ADAPTER=${mode}. Use direct or console.`)
  }

  /** @param {string[]} recipients @param {boolean} tracked */
  static deliveryPlan(recipients, tracked) {
    return tracked
      ? recipients.map(recipient => ({ domain: SmtpRules.recipientDomain(recipient), recipients: [recipient], trackedRecipient: recipient }))
      : [...SmtpRules.groupRecipientsByDomain(recipients)].map(([domain, grouped]) => ({ domain, recipients: grouped }))
  }

  /** @param {string} address */
  static recipientDomain(address) { return String(address).split('@')[1]?.toLowerCase() || '' }

  /** @param {{delivery:Record<string,any>,from:string,msg:SendRequest,hostname:string,dkim:any,createTransport:Function,resolveMx:Function}} input */
  static async deliverToDomain(input) {
    const { delivery, from, msg, hostname, dkim, createTransport, resolveMx } = input
    if (!delivery.domain) return SmtpRules.failedDelivery(delivery, null)
    let hosts
    try { hosts = await resolveMx(delivery.domain) } catch (error) { return SmtpRules.failedDelivery(delivery, error) }
    let lastError
    for (const host of hosts) {
      try {
        const transporter = createTransport(SmtpRules.transportOptions(host, hostname, dkim))
        const message = SmtpRules.buildSmtpMessage(from, msg, delivery.trackedRecipient)
        const info = await transporter.sendMail({ ...message, envelope: { from, to: delivery.recipients } })
        return SmtpRules.acceptedDelivery(delivery, host, info)
      } catch (error) { lastError = error }
    }
    return SmtpRules.failedDelivery(delivery, lastError)
  }

  /** @param {string} host @param {string} hostname @param {any} dkim */
  static transportOptions(host, hostname, dkim) {
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
  static acceptedDelivery(delivery, host, info) {
    const accepted = SmtpRules.normalizedAccepted(info.accepted, delivery.recipients)
    const acceptedSet = new Set(accepted.map(value => value.toLowerCase()))
    const failed = delivery.recipients.filter(recipient => !acceptedSet.has(recipient.toLowerCase()))
    const failures = failed.map(recipient => SmtpRules.rejectedRecipient(recipient, failed, info.rejectedErrors))
    const rejectedCode = failures.every(failure => SmtpRules.permanentReply(failure.responseCode)) ? 550 : 451
    return {
      domain: delivery.domain, recipients: delivery.recipients, deliveredRecipients: accepted, failedRecipients: failed,
      ...(delivery.trackedRecipient ? { recipient: delivery.trackedRecipient } : {}), host,
      messageId: info.messageId, accepted: info.accepted, rejected: info.rejected, response: info.response,
      ...(failed.length ? { error: 'Remote server rejected one or more recipients.', responseCode: rejectedCode, recipientFailures: failures } : {}),
    }
  }

  /** @param {string} recipient @param {string[]} failed @param {unknown} rejectedErrors */
  static rejectedRecipient(recipient, failed, rejectedErrors) {
    const errors = Array.isArray(rejectedErrors) ? rejectedErrors : []
    const matched = errors.find(error => String(error?.recipient || '').toLowerCase() === recipient.toLowerCase())
      || (failed.length === 1 && errors.length === 1 ? errors[0] : undefined)
    return {
      recipient,
      responseCode: Number(matched?.responseCode) || 451,
      error: matched instanceof Error ? SmtpRules.safeSmtpError(matched.message) : 'recipient rejected',
    }
  }

  /** @param {Record<string, any>} result */
  static recipientFailures(result) {
    if (Array.isArray(result.recipientFailures) && result.recipientFailures.length) return result.recipientFailures
    return (result.failedRecipients || []).map(recipient => ({ recipient, responseCode: result.responseCode }))
  }

  /** @param {unknown} accepted @param {string[]} requested */
  static normalizedAccepted(accepted, requested) {
    if (!Array.isArray(accepted)) return requested
    const allowed = new Set(requested.map(value => value.toLowerCase()))
    return SmtpRules.uniqueRecipients(accepted.map(String).filter(value => allowed.has(value.toLowerCase())))
  }

  /** @param {Record<string,any>} delivery @param {unknown} error */
  static failedDelivery(delivery, error) {
    return {
      domain: delivery.domain, recipients: delivery.recipients, deliveredRecipients: [], failedRecipients: delivery.recipients,
      ...(delivery.trackedRecipient ? { recipient: delivery.trackedRecipient } : {}),
      error: error instanceof Error ? SmtpRules.safeSmtpError(error.message) : 'delivery failed',
      responseCode: Number(/** @type {any} */ (error)?.responseCode) || 451,
    }
  }

  /** @param {unknown} value */
  static permanentReply(value) { const code = Number(value); return code >= 500 && code < 600 }

  /** @param {unknown} value */
  static safeSmtpError(value) { return String(value || 'delivery failed').replace(/[\r\n\0]+/g, ' ').slice(0, 500) }

  /** @returns {{ domainName: string, keySelector: string, privateKey: string } | undefined} */
  static loadDkim() {
    const domainName = ServerEnvironment.value('DKIM_DOMAIN')
    const keySelector = ServerEnvironment.value('DKIM_SELECTOR')
    let privateKey = ServerEnvironment.value('DKIM_PRIVATE_KEY') || ''
    const privateKeyFile = ServerEnvironment.value('DKIM_PRIVATE_KEY_FILE')
    if (!privateKey && privateKeyFile) {
      try { privateKey = readFileSync(privateKeyFile, 'utf8') } catch { /* optional */ }
    }
    return domainName && keySelector && privateKey ? { domainName, keySelector, privateKey } : undefined
  }

  /** @param {string[]} addresses @returns {Map<string, string[]>} */
  static groupRecipientsByDomain(addresses) {
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
  static uniqueRecipients(addresses) {
    const seen = new Set()
    return addresses.filter(address => {
      const normalized = String(address || '').trim().toLowerCase()
      if (!normalized || seen.has(normalized)) return false
      seen.add(normalized)
      return true
    })
  }

  /** @param {string} domain @returns {Promise<string[]>} */
  static async resolveMailExchangers(domain, resolver) {
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
  static buildSmtpMessage(from, msg, trackedRecipient) {
    const renderedHtml = trackedRecipient && msg.trackingId && (msg.openTracking || msg.linkTracking)
      ? TrackingTokens.prepareTrackedHtml(msg.html || '', {
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
}
