import { Security } from '../security/Security.js'

/**
 * Parses RFC 2369 / RFC 8058 unsubscribe metadata into fields that FYLO can
 * persist without embedded object arrays. Unsafe or ambiguous targets are
 * discarded at the delivery boundary and are never exposed to the browser.
 */
export class ListUnsubscribe {
  static #EMPTY_RESULT = Object.freeze({
    unsubscribeUrl: '',
    unsubscribeMailto: '',
    unsubscribeBodyUrl: '',
    unsubscribeOneClick: false,
  })

  static #BODY_KEYWORDS = [
    /unsubscribe/i, /unfollow/i, /opt[ -]{0,2}out/i, /email preferences/i,
    /subscription/i, /notification settings/i, /remove yourself from this mailing/i,
    /afmeld/i, /afmelden/i, /af te melden voor/i, /darse de baja/i,
    /désabonnement/i, /désinscrire/i, /désinscription/i, /désabonner/i,
    /préférences d[\x27’]email/i, /préférences d[\x27’]abonnement/i,
    /отказаться от подписки/i, /одјавити/i, /afskrá/i, /לבטל את המנוי/i,
    /koupe abònman/i, /退订/i, /退訂/i, /إلغاء الاشتراك/i,
    /պետք է նախ միանալ/i, /abbestellen/i, /abbestellung/i, /abmelden/i,
    /abmeldung/i, /ausschreiben/i, /austragen/i, /avprenumerera/i,
    /avregistrera/i, /prenumeration/i, /notisinställningar/i,
  ]

  /** @param {string} html */
  static #findBodyUnsubscribeUrl(html) {
    if (typeof html !== 'string' || !html || html.length > 8_388_608) return ''
    const withoutQuotes = html.replace(/<blockquote\b[^>]*>[\s\S]*?<\/blockquote\s*>/gi, '')
    const candidates = []
    for (const match of withoutQuotes.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a\s*>/gi)) {
      const hrefMatch = match[1].match(/\bhref\s*=\s*(?:\x22([^\x22]*)\x22|\x27([^\x27]*)\x27|([^\s>]+))/i)
      const href = ListUnsubscribe.#decodeHtmlAttribute(hrefMatch?.[1] || hrefMatch?.[2] || hrefMatch?.[3] || '')
      const url = ListUnsubscribe.#secureHttps(href)
      if (!url) continue
      const text = ListUnsubscribe.#stripTags(match[2])
      const start = Math.max(0, (match.index || 0) - 256)
      const end = Math.min(withoutQuotes.length, (match.index || 0) + match[0].length + 256)
      const context = ListUnsubscribe.#stripTags(withoutQuotes.slice(start, end))
      if (ListUnsubscribe.#hasBodyKeyword(url)) return url
      if (ListUnsubscribe.#hasBodyKeyword(text) || ListUnsubscribe.#hasBodyKeyword(context)) candidates.push(url)
    }
    return candidates[0] || ''
  }

  /** @param {string} value */
  static #secureHttps(value) {
    const parsed = ListUnsubscribe.#safeUrl(value)
    return parsed?.protocol === 'https:' && !parsed.username && !parsed.password
      ? parsed.href
      : ''
  }

  /** @param {string} value */
  static #hasBodyKeyword(value) {
    return ListUnsubscribe.#BODY_KEYWORDS.some(pattern => pattern.test(value))
  }

  /** @param {string} value */
  static #stripTags(value) {
    return value.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
  }

  /** @param {string} value */
  static #decodeHtmlAttribute(value) {
    return value
      .replace(/&amp;/gi, '&')
      .replace(/&quot;/gi, '"')
      .replace(/&#39;|&apos;/gi, "'")
  }

  /** @param {Record<string, string>} headers @param {string} name */
  static #header(headers, name) {
    const entry = Object.entries(headers || {})
      .find(([key]) => key.toLowerCase() === name)
    return typeof entry?.[1] === 'string' ? entry[1].trim() : ''
  }

  /** @param {string} value */
  static #bracketedTargets(value) {
    return [...value.matchAll(/<([^<>\r\n]{1,2048})>/g)].map(match => match[1].trim())
  }

  /** @param {string} value */
  static #safeUrl(value) {
    try {
      return new URL(value)
    } catch {
      return null
    }
  }

  /** @param {URL} url */
  static #safeMailto(url) {
    if (url.username || url.password || url.host) return ''
    let mailbox = ''
    try {
      mailbox = decodeURIComponent(url.pathname)
    } catch {
      return ''
    }
    if (mailbox.includes(',')) return ''
    const normalized = Security.normalizeEmailAddress(mailbox)
    if (!normalized) return ''

    const query = new URLSearchParams()
    for (const key of ['subject', 'body']) {
      const value = url.searchParams.get(key)
      if (value != null && value.length <= 2_000 && !/[\r\n]/.test(value)) query.set(key, value)
    }
    const suffix = query.size ? `?${query.toString()}` : ''
    return `mailto:${normalized}${suffix}`
  }

  /**
   * @param {Record<string, string>} headers
   * @param {string} [html]
   * @returns {{ unsubscribeUrl: string, unsubscribeMailto: string, unsubscribeBodyUrl: string, unsubscribeOneClick: boolean }}
   */
  static parse(headers, html = '') {
    const raw = ListUnsubscribe.#header(headers, 'list-unsubscribe')

    let unsubscribeUrl = ''
    let unsubscribeMailto = ''
    for (const target of ListUnsubscribe.#bracketedTargets(raw)) {
      const parsed = ListUnsubscribe.#safeUrl(target)
      if (!parsed) continue
      if (!unsubscribeUrl && parsed.protocol === 'https:' && !parsed.username && !parsed.password) {
        unsubscribeUrl = parsed.href
        continue
      }
      if (!unsubscribeMailto && parsed.protocol === 'mailto:') {
        unsubscribeMailto = ListUnsubscribe.#safeMailto(parsed)
      }
    }

    const oneClick = ListUnsubscribe.#header(headers, 'list-unsubscribe-post') === 'List-Unsubscribe=One-Click'
    const unsubscribeBodyUrl = unsubscribeUrl || unsubscribeMailto
      ? ''
      : ListUnsubscribe.#findBodyUnsubscribeUrl(html)

    return {
      unsubscribeUrl,
      unsubscribeMailto,
      unsubscribeBodyUrl,
      unsubscribeOneClick: Boolean(unsubscribeUrl && oneClick),
    }
  }
}
