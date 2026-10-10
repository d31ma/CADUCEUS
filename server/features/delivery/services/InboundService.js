import { Security } from '../../../shared/security/Security.js'
import { FyloAccessContext } from '../../../shared/persistence/FyloAccessContext.js'
import { Buffer } from 'node:buffer'
import { Threading } from '../../../shared/mailbox/Threading.js'
import { PersonalLevel } from '../../../shared/mailbox/PersonalLevel.js'
import { RuleEngine } from '../../../shared/mailbox/RuleEngine.js'
import { InboundRules } from '../../../shared/delivery/InboundRules.js'

/** @import { RouteRule } from '../../../types' */
/** @import { InboxRule } from '../../../types' */
/** @import { RuleCondition } from '../../../types' */
/** @import { ParsedAttachment } from '../../mailbox/repositories/AttachmentsRepository.js' */
/** @import { EmailsRepository } from '../../mailbox/repositories/EmailsRepository.js' */
/** @import { RulesRepository } from '../../mailbox/repositories/RulesRepository.js' */
/** @import { SuppressedRepository } from '../repositories/SuppressedRepository.js' */
/** @import { AttachmentsRepository } from '../../mailbox/repositories/AttachmentsRepository.js' */
/** @import { RawMessagesRepository } from '../../mailbox/repositories/RawMessagesRepository.js' */
/** @import { SmtpService } from './SmtpService.js' */

/**
 * Inbound mail dispatcher. Validates the recipient, looks up the matching
 * inbox rule for the recipient's domain, and dispatches to one of:
 *   - store (default): persist the email to Fylo + attachments
 *   - forward: relay via SMTP and store a forwarded copy
 *   - webhook: POST to the configured URL with an HMAC signature
 *   - drop: ack without persisting
 *
 * `applyRouteAction` and `applyInboxRules` cover domain routing and
 * post-delivery rules that operate on already-persisted emails.
 */
export class InboundService {
  /**
   * @param {EmailsRepository} emails
   * @param {RulesRepository} rules
   * @param {SuppressedRepository} suppressed
   * @param {AttachmentsRepository} attachments
   * @param {SmtpService} smtp
   * @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo
   * @param {RawMessagesRepository} [rawMessages]
   * @param {import('../../mailbox/services/CategoryMappingService.js').CategoryMappingService} [categoryMappings]
   */
  constructor(emails, rules, suppressed, attachments, smtp, fylo, rawMessages, categoryMappings) {
    this.emails = emails
    this.rules = rules
    this.suppressed = suppressed
    this.attachments = attachments
    this.smtp = smtp
    this.fylo = fylo
    this.rawMessages = rawMessages
    this.categoryMappings = categoryMappings
  }

  /**
   * @param {InboundPayload} payload
   * @returns {Promise<{ ok: boolean, emailId?: string, error?: string }>}
   */
  async processInbound(payload) {
    const recipient = Security.normalizeEmailAddress(payload.recipient)
    if (!recipient) return { ok: false, error: 'Invalid recipient' }

    const domain = recipient.split('@')[1]
    const enabled = await this.rules.listEnabledForDomain(domain)
    const matched = enabled.find(rule => InboundRules.ruleMatches(recipient, rule))
    if (!matched) return { ok: false, error: 'No matching route' }

    const suppressed = await this.suppressed.asSet()
    if (suppressed.has(payload.sender)) return { ok: false, error: 'Sender is suppressed' }

    return await this._applyAction(matched, { ...payload, recipient })
  }

  /**
   * @param {any} rule
   * @param {InboundPayload} payload
   */
  async _applyAction(rule, payload) {
    for (const action of rule.actions) {
      switch (action.type) {
        case 'store': return await this._store(payload)
        // @ts-ignore
        case 'forward': return await this._forward(action.to, payload)
        // @ts-ignore
        case 'webhook': return await this._webhook(action.url, action.secret, payload)
        // @ts-ignore
        case 'drop': return { ok: true }
      }
    }
    return await this._store(payload)
  }

  /** @param {InboundPayload} payload */
  async _store(payload) {
    const domain = payload.recipient.split('@')[1]
    const inboxFolder = await this.categoryMappings?.pathFor(payload.recipient, domain, 'inbox') || 'inbox'
    return await FyloAccessContext.runForDomain(domain, async () => {
      const parsed = await this.parseMessage(payload.body)
      const messageRecipients = PersonalLevel.recipientsFromHeaders(parsed.headers, payload.recipient)
      const id = payload.messageId || `email-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
      const email = {
        id,
        domain,
        recipient: payload.recipient,
        sender: payload.sender,
        subject: payload.subject,
        body: payload.body,
        ...(payload.htmlBody ? { htmlBody: payload.htmlBody } : {}),
        ...(messageRecipients.to.length ? { to: messageRecipients.to } : {}),
        ...(messageRecipients.cc.length ? { cc: messageRecipients.cc } : {}),
        ...(messageRecipients.bcc.length ? { bcc: messageRecipients.bcc } : {}),
        ...Threading.threadMetadataFromHeaders(parsed.headers, id, domain),
        folder: inboxFolder,
        read: false,
        starred: false,
        receivedAt: new Date().toISOString(),
        processed: true,
      }
      await this.emails.put(email)
      await this.rawMessages?.saveForEmail(email.id, domain, payload.body)
      if (payload.attachments?.length) {
        await this.attachments.saveForEmail(email.id, domain, payload.attachments)
      }
      return { ok: true, emailId: email.id }
    })
  }

  /** @param {string} to @param {InboundPayload} payload */
  async _forward(to, payload) {
    await this.smtp.send({ to: [to], subject: `Fwd: ${payload.subject}`, text: payload.body })
    return await this._store({ ...payload, recipient: to })
  }

  /** @param {string} url @param {string | undefined} secret @param {InboundPayload} payload */
  async _webhook(url, secret, payload) {
    const body = JSON.stringify(payload)
    /** @type {Record<string, string>} */
    const headers = { 'Content-Type': 'application/json' }
    if (secret) headers['X-Caduceus-Signature'] = Security.hmacSha256Hex(body, secret)
    await fetch(url, { method: 'POST', headers, body })
    return { ok: true }
  }

  /**
   * Parse a raw inbound message body (plain text, HTML, or MIME) into clean
   * text, optional HTML, plus any extracted attachments.
   *
   * @param {string | Uint8Array} rawBody
   * @returns {Promise<{ text: string, html: string, attachments: ParsedAttachment[], headers: Record<string, string> }>}
   */
  async parseMessage(rawBody) {
    if (!rawBody || (rawBody instanceof Uint8Array && rawBody.byteLength === 0)) {
      return { text: '', html: '', attachments: [], headers: {} }
    }

    const binary = rawBody instanceof Uint8Array
    const rawText = binary ? Buffer.from(rawBody).toString('utf8') : rawBody

    // Most API-originated messages are already plain text or HTML. Avoid
    // loading the comparatively large MIME parser until an RFC-style message
    // actually needs it; this also keeps first-delivery latency predictable.
    if (!binary && !InboundRules.looksLikeRfcMessage(rawText)) {
      const html = InboundRules.looksLikeHtml(rawText) ? rawText : ''
      return {
        text: InboundRules.stripHtml(html) || rawText,
        html,
        attachments: [],
        headers: {},
      }
    }

    const simple = binary ? null : InboundRules.parseSimpleRfcMessage(rawText)
    if (simple) return simple

    try {
      const { simpleParser } = await import('../../../deps/npm/mailparser.mjs')
      const parsed = await simpleParser(binary ? Buffer.from(rawBody) : rawText)
      const html = typeof parsed.html === 'string'
        ? parsed.html
        : InboundRules.looksLikeHtml(rawText) ? rawText : ''
      const attachments = (parsed.attachments || []).map(a => ({
        filename: a.filename || 'attachment',
        contentType: a.contentType || 'application/octet-stream',
        content: a.content instanceof Uint8Array ? a.content : new TextEncoder().encode(String(a.content || '')),
        disposition: a.contentDisposition || 'attachment',
        contentId: a.contentId || undefined,
      }))
      return { text: parsed.text || InboundRules.stripHtml(html) || rawText, html, attachments, headers: InboundRules.flattenHeaders(parsed.headers) }
    } catch (e) {
      const fallback = InboundRules.parseMultipartMessage(rawText)
      if (fallback) return fallback
      console.error('[inbound] mailparser parse failed, using raw body:', e)
      return { text: InboundRules.stripHtml(rawText) || rawText, html: InboundRules.looksLikeHtml(rawText) ? rawText : '', attachments: [], headers: {} }
    }
  }

  /**
   * Match a recipient against a domain's route rules. Returns
   * the first enabled rule whose `match` glob accepts the recipient.
   *
   * @param {RouteRule[]} routes
   * @param {string} recipient
   * @returns {RouteRule | null}
   */
  matchRoute(routes, recipient) {
    for (const route of routes) {
      if (!route.enabled) continue
      if (route.match === '*') return route
      if (route.match === `*@${recipient.split('@')[1]}`) return route
      if (route.match === recipient) return route
    }
    return null
  }

  /**
   * Dispatch a route action for an already-accepted message when the email
   * body is not available.
   *
   * @param {RouteRule} rule
   * @param {{ messageId: string, sender: string, recipient: string, subject: string }} meta
   */
  async applyRouteAction(rule, { messageId, sender, recipient, subject }) {
    switch (rule.action.type) {
      case 'webhook':
        try {
          await fetch(rule.action.url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ messageId, sender, recipient, subject }),
          })
        } catch (e) {
          console.error('[inbound] route webhook action failed:', e)
        }
        return
      case 'forward':
        if (rule.action.to && rule.action.to !== recipient) {
          await this.smtp.send({ to: [rule.action.to], subject: `Fwd: ${subject}`, text: `Forwarded from ${sender} to ${recipient}` })
        }
        return
      case 'store':
      case 'drop':
        return
    }
  }

  /**
   * Apply post-delivery inbox rules to an already-persisted message. Each
   * matched rule may move the message to a folder or forward a copy.
   *
   * @param {string} messageId
   * @param {string} ownerEmail
   * @param {string} domain
   * @param {{ sender: string, recipient: string, to?: string[], cc?: string[], subject: string, body?: string, hasAttachment?: boolean }} meta
   */
  async applyInboxRules(messageId, ownerEmail, domain, meta) {
    try {
      const rules = await this.rules.listEnabledForMailbox(ownerEmail, meta.recipient, domain)
      for (const rule of rules) {
        const evaluation = RuleEngine.evaluateInboxRule(rule, meta)
        if (!evaluation.matched) continue
        await this._applyInboxRuleActions(rule, messageId, meta)
        if (evaluation.stopProcessing) break
      }
    } catch (e) {
      console.error('[inbound] applyInboxRules failed:', e)
    }
  }

  /**
   * @param {InboxRule} rule
   * @param {string} messageId
   * @param {{ sender: string, recipient: string, subject: string, body?: string }} meta
   */
  async _applyInboxRuleActions(rule, messageId, meta) {
    const [docId] = await this.emails.findById(messageId)
    if (!docId) return
    /** @type {Record<string, unknown>} */
    const patch = {}
    for (const action of rule.actions) {
      if (action.type === 'folder') {
        patch.folder = action.folder
      } else if (action.type === 'delete') {
        patch.folder = await this.categoryMappings?.pathFor(meta.recipient, meta.recipient.split('@')[1] || '', 'trash') || 'trash'
      } else if (action.type === 'markRead') {
        patch.read = action.value
      } else if (action.type === 'star') {
        patch.starred = action.value
      } else if (action.type === 'forward') {
        try {
          await this.smtp.send({ to: [action.to], subject: `Fwd: ${meta.subject}`, text: 'Forwarded' })
        } catch (e) {
          console.error('[inbound] inbox rule forward failed:', e)
        }
      }
    }
    if (Object.keys(patch).length) await this.emails.update(docId, patch)
  }
}

