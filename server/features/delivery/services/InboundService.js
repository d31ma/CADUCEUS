import { Security } from '../../../platform/security/Security.js'
import { FyloAccessContext } from '../../../platform/persistence/FyloAccessContext.js'
import { Buffer } from 'node:buffer'
import { threadMetadataFromHeaders } from '../../mailbox/services/threading.js'
import { recipientsFromHeaders } from '../../mailbox/services/personal-level.js'
import { evaluateInboxRule } from '../../mailbox/services/rule-engine.js'
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
   * @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo
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
    const matched = enabled.find(rule => ruleMatches(recipient, rule))
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
      const messageRecipients = recipientsFromHeaders(parsed.headers, payload.recipient)
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
        ...threadMetadataFromHeaders(parsed.headers, id, domain),
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
    if (!binary && !looksLikeRfcMessage(rawText)) {
      const html = looksLikeHtml(rawText) ? rawText : ''
      return {
        text: stripHtml(html) || rawText,
        html,
        attachments: [],
        headers: {},
      }
    }

    const simple = binary ? null : parseSimpleRfcMessage(rawText)
    if (simple) return simple

    try {
      const { simpleParser } = await import('mailparser')
      const parsed = await simpleParser(binary ? Buffer.from(rawBody) : rawText)
      const html = typeof parsed.html === 'string'
        ? parsed.html
        : looksLikeHtml(rawText) ? rawText : ''
      const attachments = (parsed.attachments || []).map(a => ({
        filename: a.filename || 'attachment',
        contentType: a.contentType || 'application/octet-stream',
        content: a.content instanceof Uint8Array ? a.content : new TextEncoder().encode(String(a.content || '')),
        disposition: a.contentDisposition || 'attachment',
        contentId: a.contentId || undefined,
      }))
      return { text: parsed.text || stripHtml(html) || rawText, html, attachments, headers: flattenHeaders(parsed.headers) }
    } catch (e) {
      const fallback = parseMultipartMessage(rawText)
      if (fallback) return fallback
      console.error('[inbound] mailparser parse failed, using raw body:', e)
      return { text: stripHtml(rawText) || rawText, html: looksLikeHtml(rawText) ? rawText : '', attachments: [], headers: {} }
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
        const evaluation = evaluateInboxRule(rule, meta)
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

/**
 * @param {string} recipient
 * @param {InboxRule} rule
 */
function ruleMatches(recipient, rule) {
  if (!rule.conditions || rule.conditions.length === 0) return true
  const domain = recipient.split('@')[1]
  const parts = { recipient, domain }
  const results = rule.conditions.map(c => conditionMatchesRecipient(c, parts))
  return rule.conditionMatch === 'all' ? results.every(Boolean) : results.some(Boolean)
}

/**
 * @param {RuleCondition} condition
 * @param {{ recipient: string, domain: string }} parts
 */
function conditionMatchesRecipient(condition, parts) {
  if (condition.field !== 'to') return false
  return matchString(parts.recipient, condition.op, condition.value)
}

/** @param {string} haystack @param {string} op @param {string} needle */
function matchString(haystack, op, needle) {
  const h = haystack.toLowerCase()
  const n = needle.toLowerCase()
  if (op === 'contains') return h.includes(n)
  if (op === 'equals') return h === n
  if (op === 'startsWith') return h.startsWith(n)
  if (op === 'endsWith') return h.endsWith(n)
  return false
}

/** @param {string} value */
function looksLikeHtml(value) {
  return /<([a-z][\w:-]*)(\s[^>]*)?>[\s\S]*<\/\1>|<(br|hr|img|meta|link|input)\b[^>]*>/i.test(value)
}

/** @param {string} html */
function stripHtml(html) {
  return String(html || '')
    .replace(/<style\b[\s\S]*?<\/style>/gi, ' ')
    .replace(/<script\b[\s\S]*?<\/script>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/g, "'")
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * Flatten mailparser's headers Map into a plain object of lower-cased
 * `name → string`. Non-string header values (addresses, dates) are coerced so
 * downstream consumers (e.g. the spam filter reading `x-spam-*`) get a simple
 * lookup table.
 *
 * @param {Map<string, unknown> | undefined} headers
 * @returns {Record<string, string>}
 */
function flattenHeaders(headers) {
  /** @type {Record<string, string>} */
  const out = {}
  if (!headers || typeof headers.forEach !== 'function') return out
  headers.forEach((value, key) => {
    const headerValue = /** @type {any} */ (value)
    const addressValues = Array.isArray(headerValue?.value) ? headerValue.value : null
    out[String(key).toLowerCase()] = typeof headerValue === 'string'
      ? headerValue
      : addressValues
        ? addressValues.map(entry => entry?.address ?? '').filter(Boolean).join(', ')
        : String(headerValue?.value ?? headerValue ?? '')
  })
  return out
}

/**
 * Whether the text begins with a syntactically plausible RFC header block.
 * Plain messages such as "Note: lunch\n\nTomorrow" are not treated as MIME
 * unless every line before the first blank line is a header or continuation.
 *
 * @param {string} rawBody
 */
function looksLikeRfcMessage(rawBody) {
  const splitAt = rawBody.search(/\r?\n\r?\n/)
  if (splitAt < 0) return false
  const lines = rawBody.slice(0, splitAt).split(/\r?\n/)
  const validBlock = lines.length > 0 && lines.every((line) =>
    /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+:\s*/.test(line) || /^[ \t]+/.test(line),
  )
  const recognizable = lines.some((line) =>
    /^(?:authentication-results|cc|content-[^:]+|date|from|message-id|mime-version|received|reply-to|subject|to|x-[^:]+):/i.test(line),
  )
  return validBlock && recognizable
}

/**
 * Parse a simple, unencoded single-part RFC message without loading
 * `mailparser`. Multipart messages, attachments, and encoded bodies continue
 * through the full parser.
 *
 * @param {string} rawBody
 * @returns {{ text: string, html: string, attachments: ParsedAttachment[], headers: Record<string, string> } | null}
 */
function parseSimpleRfcMessage(rawBody) {
  const splitAt = rawBody.search(/\r?\n\r?\n/)
  if (splitAt < 0) return null
  const headers = parseHeaderBlock(rawBody.slice(0, splitAt))
  const contentType = headers['content-type']?.toLowerCase() ?? 'text/plain'
  const transferEncoding = headers['content-transfer-encoding']?.toLowerCase()
  if (
    contentType.includes('multipart/')
    || /content-disposition:\s*attachment/i.test(rawBody.slice(0, splitAt))
    || (transferEncoding && !['7bit', '8bit', 'binary'].includes(transferEncoding))
  ) return null

  const body = rawBody.slice(splitAt).replace(/^\r?\n\r?\n/, '')
  const html = contentType.includes('text/html') || looksLikeHtml(body) ? body : ''
  return {
    text: stripHtml(html) || body,
    html,
    attachments: [],
    headers,
  }
}

/**
 * Dependency-free MIME fallback for isolated Yon handlers where an optional
 * CommonJS mailparser dependency cannot be loaded by the standalone runtime.
 *
 * Exported so it can be exercised directly: reaching it through
 * {@link InboundService.parseMessage} requires mailparser to be missing or
 * throwing, which is exactly the situation tests cannot arrange cheaply.
 *
 * @param {string} rawBody
 * @returns {{ text: string, html: string, attachments: ParsedAttachment[], headers: Record<string, string> } | null}
 */
export function parseMultipartMessage(rawBody) {
  const splitAt = rawBody.search(/\r?\n\r?\n/)
  if (splitAt < 0) return null
  const topHeaders = parseHeaderBlock(rawBody.slice(0, splitAt))
  const boundary = topHeaders['content-type']?.match(/boundary=(?:"([^"]+)"|([^;\s]+))/i)?.slice(1).find(Boolean)
  if (!boundary) return null

  const attachments = []
  const textParts = []
  const htmlParts = []
  for (const rawPart of rawBody.slice(splitAt).split(`--${boundary}`).slice(1)) {
    const part = rawPart.replace(/^\r?\n/, '').replace(/\r?\n--\s*$/, '').trim()
    if (!part || part === '--') continue
    const partSplitAt = part.search(/\r?\n\r?\n/)
    if (partSplitAt < 0) continue
    const headers = parseHeaderBlock(part.slice(0, partSplitAt))
    const body = part.slice(partSplitAt).replace(/^\r?\n\r?\n/, '').replace(/\r?\n$/, '')
    const disposition = headers['content-disposition'] ?? ''
    const contentType = (headers['content-type'] ?? 'text/plain').split(';', 1)[0].trim().toLowerCase()
    const filename = disposition.match(/filename=(?:"([^"]+)"|([^;\s]+))/i)?.slice(1).find(Boolean)
      ?? headers['content-type']?.match(/name=(?:"([^"]+)"|([^;\s]+))/i)?.slice(1).find(Boolean)

    if (/attachment/i.test(disposition) || filename) {
      const encoded = headers['content-transfer-encoding']?.toLowerCase() === 'base64'
        ? Buffer.from(body.replace(/\s+/g, ''), 'base64')
        : Buffer.from(body)
      attachments.push({
        filename: filename || 'attachment',
        contentType,
        content: encoded,
        disposition: /inline/i.test(disposition) ? 'inline' : 'attachment',
        contentId: headers['content-id']?.replace(/^<|>$/g, ''),
      })
    } else if (contentType === 'text/html') {
      htmlParts.push(body)
    } else if (contentType === 'text/plain') {
      textParts.push(body)
    }
  }

  const html = htmlParts.join('\n')
  return {
    text: textParts.join('\n') || stripHtml(html) || rawBody,
    html,
    attachments,
    headers: topHeaders,
  }
}

/** @param {string} block @returns {Record<string, string>} */
function parseHeaderBlock(block) {
  const headers = /** @type {Record<string, string>} */ ({})
  for (const line of block.replace(/\r?\n[ \t]+/g, ' ').split(/\r?\n/)) {
    const colon = line.indexOf(':')
    if (colon <= 0) continue
    headers[line.slice(0, colon).trim().toLowerCase()] = line.slice(colon + 1).trim()
  }
  return headers
}
