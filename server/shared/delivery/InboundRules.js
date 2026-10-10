import { Security } from '../security/Security.js'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'
import { Buffer } from 'node:buffer'
import { Threading } from '../mailbox/Threading.js'
import { PersonalLevel } from '../mailbox/PersonalLevel.js'
import { RuleEngine } from '../mailbox/RuleEngine.js'

export class InboundRules {
  /**
   * @param {string} recipient
   * @param {InboxRule} rule
   */
  static ruleMatches(recipient, rule) {
    if (!rule.conditions || rule.conditions.length === 0) return true
    const domain = recipient.split('@')[1]
    const parts = { recipient, domain }
    const results = rule.conditions.map(c => InboundRules.conditionMatchesRecipient(c, parts))
    return rule.conditionMatch === 'all' ? results.every(Boolean) : results.some(Boolean)
  }

  /**
   * @param {RuleCondition} condition
   * @param {{ recipient: string, domain: string }} parts
   */
  static conditionMatchesRecipient(condition, parts) {
    if (condition.field !== 'to') return false
    return InboundRules.matchString(parts.recipient, condition.op, condition.value)
  }

  /** @param {string} haystack @param {string} op @param {string} needle */
  static matchString(haystack, op, needle) {
    const h = haystack.toLowerCase()
    const n = needle.toLowerCase()
    if (op === 'contains') return h.includes(n)
    if (op === 'equals') return h === n
    if (op === 'startsWith') return h.startsWith(n)
    if (op === 'endsWith') return h.endsWith(n)
    return false
  }

  /** @param {string} value */
  static looksLikeHtml(value) {
    return /<([a-z][\w:-]*)(\s[^>]*)?>[\s\S]*<\/\1>|<(br|hr|img|meta|link|input)\b[^>]*>/i.test(value)
  }

  /** @param {string} html */
  static stripHtml(html) {
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
  static flattenHeaders(headers) {
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
  static looksLikeRfcMessage(rawBody) {
    const splitAt = rawBody.search(/\r?\n\r?\n/)
    if (splitAt < 0) return false
    const lines = rawBody.slice(0, splitAt).split(/\r?\n/)
    const validBlock = lines.length > 0 && lines.every((line) =>
      /^[!#$%&\x27*+\-.^_\x60|~0-9A-Za-z]+:\s*/.test(line) || /^[ \t]+/.test(line),
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
  static parseSimpleRfcMessage(rawBody) {
    const splitAt = rawBody.search(/\r?\n\r?\n/)
    if (splitAt < 0) return null
    const headers = InboundRules.parseHeaderBlock(rawBody.slice(0, splitAt))
    const contentType = headers['content-type']?.toLowerCase() ?? 'text/plain'
    const transferEncoding = headers['content-transfer-encoding']?.toLowerCase()
    if (
      contentType.includes('multipart/')
      || /content-disposition:\s*attachment/i.test(rawBody.slice(0, splitAt))
      || (transferEncoding && !['7bit', '8bit', 'binary'].includes(transferEncoding))
    ) return null

    const body = rawBody.slice(splitAt).replace(/^\r?\n\r?\n/, '')
    const html = contentType.includes('text/html') || InboundRules.looksLikeHtml(body) ? body : ''
    return {
      text: InboundRules.stripHtml(html) || body,
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
  static parseMultipartMessage(rawBody) {
    const splitAt = rawBody.search(/\r?\n\r?\n/)
    if (splitAt < 0) return null
    const topHeaders = InboundRules.parseHeaderBlock(rawBody.slice(0, splitAt))
    const boundary = topHeaders['content-type']?.match(/boundary=(?:\x22([^\x22]+)\x22|([^;\s]+))/i)?.slice(1).find(Boolean)
    if (!boundary) return null

    const attachments = []
    const textParts = []
    const htmlParts = []
    for (const rawPart of rawBody.slice(splitAt).split(`--${boundary}`).slice(1)) {
      const part = rawPart.replace(/^\r?\n/, '').replace(/\r?\n--\s*$/, '').trim()
      if (!part || part === '--') continue
      const partSplitAt = part.search(/\r?\n\r?\n/)
      if (partSplitAt < 0) continue
      const headers = InboundRules.parseHeaderBlock(part.slice(0, partSplitAt))
      const body = part.slice(partSplitAt).replace(/^\r?\n\r?\n/, '').replace(/\r?\n$/, '')
      const disposition = headers['content-disposition'] ?? ''
      const contentType = (headers['content-type'] ?? 'text/plain').split(';', 1)[0].trim().toLowerCase()
      const filename = disposition.match(/filename=(?:\x22([^\x22]+)\x22|([^;\s]+))/i)?.slice(1).find(Boolean)
        ?? headers['content-type']?.match(/name=(?:\x22([^\x22]+)\x22|([^;\s]+))/i)?.slice(1).find(Boolean)

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
      text: textParts.join('\n') || InboundRules.stripHtml(html) || rawBody,
      html,
      attachments,
      headers: topHeaders,
    }
  }

  /** @param {string} block @returns {Record<string, string>} */
  static parseHeaderBlock(block) {
    const headers = /** @type {Record<string, string>} */ ({})
    for (const line of block.replace(/\r?\n[ \t]+/g, ' ').split(/\r?\n/)) {
      const colon = line.indexOf(':')
      if (colon <= 0) continue
      headers[line.slice(0, colon).trim().toLowerCase()] = line.slice(colon + 1).trim()
    }
    return headers
  }
}
