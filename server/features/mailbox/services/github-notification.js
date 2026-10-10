import { Buffer } from 'node:buffer'
import { PhishingDetection } from '../../delivery/services/phishing-detection.js'

const MAX_HTML_BYTES = 8 * 1024 * 1024
const VIEW_ON_GITHUB = /\bview(?:\s+(?:it|this|thread|issue|pull\s+request|comment))?\s+on\s+github\b/i

/**
 * Recognize the exact capability exposed by Mailspring's
 * message-view-on-github package without trusting arbitrary mail links.
 */
export class GitHubNotification {
  /**
   * @param {{ sender?: unknown, headerFrom?: unknown, htmlBody?: unknown }} message
   * @returns {string}
   */
  static findTarget(message) {
    if (!isGitHubSender(message?.headerFrom) && !isGitHubSender(message?.sender)) return ''
    const html = typeof message?.htmlBody === 'string' ? message.htmlBody : ''
    if (!html || Buffer.byteLength(html, 'utf8') > MAX_HTML_BYTES) return ''

    for (const match of html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a\s*>/gi)) {
      const text = decodeHtml(stripTags(match[2])).replace(/\s+/g, ' ').trim()
      if (!VIEW_ON_GITHUB.test(text)) continue
      const hrefMatch = match[1].match(/\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/i)
      const target = safeGitHubUrl(decodeHtml(hrefMatch?.[1] || hrefMatch?.[2] || hrefMatch?.[3] || ''))
      if (target) return target
    }
    return ''
  }
}

/** @param {unknown} value */
function isGitHubSender(value) {
  return PhishingDetection.extractAddresses(value)
    .some(address => address.split('@')[1] === 'github.com')
}

/** @param {string} value */
function stripTags(value) {
  return value.replace(/<[^>]*>/g, ' ')
}

/** @param {string} value */
function decodeHtml(value) {
  return value
    .replace(/&amp;/gi, '&')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&#(x[0-9a-f]+|\d+);/gi, (_match, code) => {
      const number = String(code).toLowerCase().startsWith('x')
        ? Number.parseInt(String(code).slice(1), 16)
        : Number.parseInt(String(code), 10)
      return Number.isSafeInteger(number) && number > 0 && number <= 0x10ffff
        ? String.fromCodePoint(number)
        : ''
    })
}

/** @param {string} value */
function safeGitHubUrl(value) {
  if (!value || value.length > 2048 || /[\u0000-\u001f\u007f]/.test(value)) return ''
  try {
    const url = new URL(value)
    if (url.protocol !== 'https:' || url.username || url.password || url.port) return ''
    if (url.hostname !== 'github.com' && url.hostname !== 'www.github.com') return ''
    return url.href
  } catch {
    return ''
  }
}
