import { createCipheriv, createDecipheriv, createHmac, randomBytes } from 'node:crypto'
import { DomUtils, parseDocument } from 'htmlparser2'

const TOKEN_VERSION = 1
const TOKEN_LIFETIME_MS = 2 * 365 * 24 * 60 * 60 * 1000
const MAX_TOKEN_LENGTH = 8192
const MAX_TRACKED_HTML_BYTES = 2 * 1024 * 1024
const MAX_TRACKED_LINKS = 100
const MAX_URL_LENGTH = 2048
const QUOTE_CLASSES = new Set(['gmail_quote', 'gmail_extra', 'yahoo_quoted', 'moz-cite-prefix'])

/**
 * Create an authenticated, encrypted tracking capability. The public endpoint
 * trusts no tracking id, recipient, or redirect URL outside this token.
 *
 * @param {{ type: 'open'|'click', trackingId: string, recipient?: string, url?: string, linkIndex?: number }} input
 * @param {{ secret?: string, now?: number }} [options]
 */
export function createTrackingToken(input, options = {}) {
  const now = options.now ?? Date.now()
  const payload = {
    v: TOKEN_VERSION,
    t: input.type === 'click' ? 'c' : 'o',
    i: normalizeTrackingId(input.trackingId),
    e: now + TOKEN_LIFETIME_MS,
    ...(normalizeRecipient(input.recipient) ? { r: normalizeRecipient(input.recipient) } : {}),
    ...(input.type === 'click' ? { u: normalizeTrackedUrl(input.url), n: normalizeLinkIndex(input.linkIndex) } : {}),
  }
  if (!payload.i || (payload.t === 'c' && (!payload.u || payload.n == null || payload.n < 0))) throw new TypeError('Invalid tracking capability')
  const iv = randomBytes(12)
  const cipher = createCipheriv('aes-256-gcm', encryptionKey(options.secret), iv)
  cipher.setAAD(Buffer.from('caduceus-tracking-v1'))
  const ciphertext = Buffer.concat([cipher.update(JSON.stringify(payload), 'utf8'), cipher.final()])
  return [
    `v${TOKEN_VERSION}`,
    iv.toString('base64url'),
    ciphertext.toString('base64url'),
    cipher.getAuthTag().toString('base64url'),
  ].join('.')
}

/**
 * Verify and decode one tracking capability. Invalid, expired, oversized, or
 * type-confused values return null without exposing why to the public route.
 *
 * @param {unknown} value
 * @param {{ secret?: string, now?: number, expectedType?: 'open'|'click' }} [options]
 * @returns {{ type: 'open', trackingId: string, recipient: string, expiresAt: number } | { type: 'click', trackingId: string, recipient: string, url: string, linkIndex: number, expiresAt: number } | null}
 */
export function readTrackingToken(value, options = {}) {
  const token = typeof value === 'string' ? value.trim() : ''
  if (!token || token.length > MAX_TOKEN_LENGTH) return null
  let decoded
  try {
    const [version, ivText, ciphertextText, tagText, extra] = token.split('.')
    if (version !== `v${TOKEN_VERSION}` || !ivText || !ciphertextText || !tagText || extra) return null
    const iv = Buffer.from(ivText, 'base64url')
    const ciphertext = Buffer.from(ciphertextText, 'base64url')
    const tag = Buffer.from(tagText, 'base64url')
    if (iv.length !== 12 || tag.length !== 16 || ciphertext.length === 0) return null
    if (iv.toString('base64url') !== ivText || ciphertext.toString('base64url') !== ciphertextText || tag.toString('base64url') !== tagText) return null
    const decipher = createDecipheriv('aes-256-gcm', encryptionKey(options.secret), iv)
    decipher.setAAD(Buffer.from('caduceus-tracking-v1'))
    decipher.setAuthTag(tag)
    decoded = JSON.parse(Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString('utf8'))
  } catch {
    return null
  }
  const type = /** @type {'open'|'click'|''} */ (decoded?.t === 'o' ? 'open' : decoded?.t === 'c' ? 'click' : '')
  const trackingId = normalizeTrackingId(decoded?.i)
  const recipient = normalizeRecipient(decoded?.r)
  const expiresAt = Number(decoded?.e)
  if (decoded?.v !== TOKEN_VERSION || !type || !trackingId || !Number.isSafeInteger(expiresAt)) return null
  if (expiresAt < (options.now ?? Date.now())) return null
  if (options.expectedType && type !== options.expectedType) return null
  if (type === 'click') {
    const url = normalizeTrackedUrl(decoded?.u)
    const linkIndex = Number(decoded?.n)
    if (!url || !Number.isSafeInteger(linkIndex) || linkIndex < 0 || linkIndex >= MAX_TRACKED_LINKS) return null
    return { type, trackingId, recipient, url, linkIndex, expiresAt }
  }
  return { type, trackingId, recipient, expiresAt }
}

/**
 * Produce a per-recipient delivery body. The stored Sent copy remains clean;
 * only the SMTP body receives authenticated tracking URLs.
 *
 * @param {string} html
 * @param {{ trackingId: string, recipient?: string, openTracking?: boolean, linkTracking?: boolean, baseUrl?: string, secret?: string }} options
 * @returns {{ html: string, links: Array<{ index: number, url: string, title: string }> }}
 */
export function prepareTrackedHtml(html, options) {
  const source = String(html || '')
  if (!source || Buffer.byteLength(source) > MAX_TRACKED_HTML_BYTES) return { html: source, links: [] }
  const baseUrl = resolveTrackingBaseUrl(options.baseUrl)
  const document = parseDocument(source, { decodeEntities: true })
  const links = []

  if (options.linkTracking) {
    const anchors = DomUtils.findAll(
      node => isElement(node, 'a') && typeof node.attribs?.href === 'string',
      document.children,
    )
    for (const anchor of anchors) {
      if (links.length >= MAX_TRACKED_LINKS || isInsideQuotedContent(anchor)) continue
      const url = normalizeTrackedUrl(anchor.attribs.href)
      if (!url || isTrackingEndpoint(url, baseUrl)) continue
      const index = links.length
      const token = createTrackingToken({
        type: 'click',
        trackingId: options.trackingId,
        recipient: options.recipient,
        url,
        linkIndex: index,
      }, { secret: options.secret })
      anchor.attribs.href = `${baseUrl}/track/click?t=${encodeURIComponent(token)}`
      links.push({ index, url, title: DomUtils.textContent(anchor).trim().slice(0, 200) || url })
    }
  }

  if (options.openTracking) {
    const token = createTrackingToken({
      type: 'open',
      trackingId: options.trackingId,
      recipient: options.recipient,
    }, { secret: options.secret })
    const fragment = parseDocument(`<img alt="" aria-hidden="true" width="1" height="1" style="display:none" src="${baseUrl}/track/open?t=${encodeURIComponent(token)}">`)
    const pixel = fragment.children[0]
    if (pixel) {
      pixel.parent = document
      document.children.push(pixel)
    }
  }

  return { html: DomUtils.getInnerHTML(document), links }
}

/** Return the original links that can participate in click analytics. */
export function listTrackableLinks(html) {
  const source = String(html || '')
  if (!source || Buffer.byteLength(source) > MAX_TRACKED_HTML_BYTES) return []
  const document = parseDocument(source, { decodeEntities: true })
  return DomUtils.findAll(
    node => isElement(node, 'a') && typeof node.attribs?.href === 'string',
    document.children,
  ).flatMap(anchor => {
    if (isInsideQuotedContent(anchor)) return []
    const url = normalizeTrackedUrl(anchor.attribs.href)
    if (!url) return []
    return [{
      index: 0,
      url,
      title: DomUtils.textContent(anchor).trim().slice(0, 200) || url,
    }]
  }).slice(0, MAX_TRACKED_LINKS).map((link, index) => ({ ...link, index }))
}

/** @param {string | undefined} configured */
export function resolveTrackingBaseUrl(configured) {
  const candidate = configured || process.env.CADUCEUS_PUBLIC_URL || process.env.SESAME_ISSUER
    || (process.env.NODE_ENV === 'production' ? '' : `http://127.0.0.1:${process.env.PORT || 8080}`)
  let url
  try { url = new URL(String(candidate || '')) } catch { throw new TypeError('CADUCEUS_PUBLIC_URL must be an absolute HTTP(S) URL') }
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new TypeError('CADUCEUS_PUBLIC_URL must be a credential-free HTTP(S) origin')
  }
  const loopback = ['localhost', '127.0.0.1', '::1'].includes(url.hostname)
  if (process.env.NODE_ENV === 'production' && url.protocol !== 'https:' && !loopback) {
    throw new TypeError('CADUCEUS_PUBLIC_URL must use HTTPS in production')
  }
  return url.origin
}

/** @param {string | undefined} explicit */
function encryptionKey(explicit) {
  const secret = explicit || process.env.TRACKING_SECRET || process.env.INBOUND_WEBHOOK_SECRET
  if (!secret || secret.length < 16) throw new TypeError('TRACKING_SECRET or INBOUND_WEBHOOK_SECRET must contain at least 16 characters')
  return createHmac('sha256', secret).update('caduceus-tracking-aead-v1').digest()
}

/** @param {unknown} value */
function normalizeTrackingId(value) {
  const id = typeof value === 'string' ? value.trim() : ''
  return /^[A-Za-z0-9_-]{16,128}$/.test(id) ? id : ''
}

/** @param {unknown} value */
function normalizeRecipient(value) {
  const recipient = typeof value === 'string' ? value.trim().toLowerCase() : ''
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(recipient) && recipient.length <= 320 ? recipient : ''
}

/** @param {unknown} value */
function normalizeTrackedUrl(value) {
  const source = typeof value === 'string' ? value.trim() : ''
  if (!source || source.length > MAX_URL_LENGTH || /[\u0000-\u001f\u007f]/.test(source)) return ''
  let url
  try { url = new URL(source) } catch { return '' }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return ''
  return url.href
}

/** @param {unknown} value */
function normalizeLinkIndex(value) {
  const index = Number(value)
  return Number.isSafeInteger(index) && index >= 0 && index < MAX_TRACKED_LINKS ? index : -1
}

/** @param {any} node @param {string} [name] */
function isElement(node, name) {
  return Boolean(node && (node.type === 'tag' || node.type === 'script' || node.type === 'style') && (!name || node.name === name))
}

/** @param {any} node */
function isInsideQuotedContent(node) {
  for (let current = node?.parent; current; current = current.parent) {
    if (!isElement(current)) continue
    if (current.name === 'blockquote' || String(current.attribs?.type || '').toLowerCase() === 'cite') return true
    const classes = String(current.attribs?.class || '').split(/\s+/)
    if (classes.some(value => QUOTE_CLASSES.has(value.toLowerCase()))) return true
  }
  return false
}

/** @param {string} value @param {string} baseUrl */
function isTrackingEndpoint(value, baseUrl) {
  try {
    const url = new URL(value)
    return url.origin === baseUrl && /^\/track\/(?:open|click)$/.test(url.pathname)
  } catch {
    return false
  }
}
