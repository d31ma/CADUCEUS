// @ts-check

const UNSAFE_BLOCK_RE = /<(script|style|iframe|frame|frameset|object|embed|applet|form|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi
const UNSAFE_SINGLE_RE = /<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*\/?\s*>/gi
const QUOTE_START_RE = /<(?:section\b[^>]*\bdata-caduceus-quote\s*=|blockquote\b|[^>]+class\s*=\s*["'][^"']*(?:gmail_quote|quoted-message)[^"']*["'])/i
const SIGNATURE_RE = /(?:<signature\b[^>]*\b(?:data-signature-id|id)\s*=\s*["']([^"']+)["'][^>]*>[\s\S]*?<\/signature\s*>|<div\b(?=[^>]*\bclass\s*=\s*["'][^"']*\bcaduceus-signature\b[^"']*["'])(?=[^>]*\bdata-signature-id\s*=\s*["']([^"']+)["'])[^>]*>[\s\S]*?<\/div\s*>)/i

/** @param {unknown} input */
export function sanitizeSignatureHTML(input) {
  return String(input ?? '')
    .replace(/\u0000/g, '')
    .replace(UNSAFE_BLOCK_RE, '')
    .replace(UNSAFE_SINGLE_RE, '')
    .replace(/\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/\s+(href|src|xlink:href)\s*=\s*(["'])\s*(?:javascript|vbscript):[\s\S]*?\2/gi, ' $1="#"')
    .replace(/\s+style\s*=\s*(["'])[^"']*(?:expression\s*\(|url\s*\(\s*["']?\s*(?:javascript|vbscript):)[^"']*\1/gi, '')
}

/** @param {string} html */
function quoteStart(html) {
  const index = String(html || '').search(QUOTE_START_RE)
  return index < 0 ? String(html || '').length : index
}

/** @param {string} html @returns {string} */
export function currentSignatureId(html) {
  const match = SIGNATURE_RE.exec(String(html || '').slice(0, quoteStart(html)))
  return match?.[1] || match?.[2] || ''
}

/** @param {string} html */
function trailingBreaks(html) {
  let content = html.trim()
  let count = 0
  while (/<br\s*\/?>\s*$/i.test(content) || /<div>\s*<\/div>\s*$/i.test(content)) {
    content = content.replace(/(?:<br\s*\/?>|<div>\s*<\/div>)\s*$/i, '').trim()
    count += 1
  }
  return count
}

/**
 * Replace, remove, or insert the active signature before the first quoted
 * reply block without disturbing prior conversation content.
 *
 * @param {string} body
 * @param {{ id: string, text: string } | null | undefined} signature
 */
export function applySignature(body, signature) {
  let next = String(body || '')
  let spacer = '<br>'
  const boundary = quoteStart(next)
  const editable = next.slice(0, boundary)
  const protectedSuffix = next.slice(boundary)
  if (SIGNATURE_RE.test(editable)) {
    next = `${editable.replace(SIGNATURE_RE, '')}${protectedSuffix}`
    spacer = ''
  }
  if (!signature) return next
  const insertion = quoteStart(next)
  const before = next.slice(0, insertion)
  const after = next.slice(insertion)
  if (trailingBreaks(before) > 1) spacer = ''
  const id = String(signature.id).replace(/[^A-Za-z0-9._:-]/g, '')
  const content = sanitizeSignatureHTML(signature.text)
  return `${before}${spacer}<signature data-signature-id="${id}" class="caduceus-signature" contenteditable="false">${content}</signature>${after}`
}

/** @param {Array<{ id: string, text: string, defaultFor?: string[] }>} signatures @param {string} address */
export function defaultSignatureForAddress(signatures, address) {
  const normalized = String(address || '').trim().toLocaleLowerCase()
  return signatures.find(signature => (signature.defaultFor ?? []).some(item => item.toLocaleLowerCase() === normalized)) ?? null
}

/**
 * Prefer sanitized rich clipboard markup, except when the plain flavor is
 * clearly HTML source the user intentionally copied.
 *
 * @param {string} plain
 * @param {string} html
 * @returns {string}
 */
export function signatureHTMLFromClipboard(plain, html) {
  if (/<[a-z!/][\s\S]*>/i.test(plain)) return sanitizeSignatureHTML(plain)
  return sanitizeSignatureHTML(html).trim()
}
