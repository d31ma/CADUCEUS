// @ts-check

const RASTER_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'])
const TEXT_TYPES = new Set(['application/json', 'application/xml', 'application/yaml', 'application/x-yaml'])
const BASE64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/

/** @param {unknown} value */
export function normalizedContentType(value) {
  return String(value || 'application/octet-stream').split(';', 1)[0].trim().toLowerCase()
}

/** @param {unknown} value */
export function normalizeCidReference(value) {
  let normalized = String(value || '').trim().replace(/^cid:/i, '')
  try { normalized = decodeURIComponent(normalized) } catch {}
  if (normalized.startsWith('<') && normalized.endsWith('>')) normalized = normalized.slice(1, -1).trim()
  if (!normalized || normalized.length > 255 || /[\s<>\u0000-\u001f\u007f-\u009f]/.test(normalized)) return ''
  return normalized.toLowerCase()
}

/** @param {unknown} contentType */
export function isRasterImage(contentType) {
  return RASTER_IMAGE_TYPES.has(normalizedContentType(contentType))
}

/** @param {unknown} contentType */
export function attachmentPreviewKind(contentType) {
  const normalized = normalizedContentType(contentType)
  if (RASTER_IMAGE_TYPES.has(normalized)) return 'image'
  if (normalized.startsWith('text/') || TEXT_TYPES.has(normalized)) return 'text'
  if (normalized === 'application/pdf') return 'pdf'
  return ''
}

/**
 * Produce a preview source only for inert raster images and PDFs. SVG is
 * deliberately excluded because it is an active document format.
 *
 * @param {{ contentType?: unknown, contentBase64?: unknown }} data
 */
export function attachmentPreviewDataUrl(data) {
  const kind = attachmentPreviewKind(data?.contentType)
  if (kind !== 'image' && kind !== 'pdf') return ''
  const contentBase64 = String(data?.contentBase64 || '')
  if (!contentBase64 || !BASE64.test(contentBase64)) return ''
  return `data:${normalizedContentType(data.contentType)};base64,${contentBase64}`
}

/**
 * Decode a bounded text preview. Oversized content remains downloadable but
 * is never expanded into the DOM.
 *
 * @param {{ contentBase64?: unknown }} data
 * @param {number} [maxBytes]
 */
export function decodeAttachmentText(data, maxBytes = 1024 * 1024) {
  const contentBase64 = String(data?.contentBase64 || '')
  if (!contentBase64 || !BASE64.test(contentBase64)) throw new Error('Attachment content is not valid base64.')
  const binary = atob(contentBase64)
  if (binary.length > maxBytes) throw new Error('This text attachment is too large to preview.')
  const bytes = Uint8Array.from(binary, character => character.charCodeAt(0))
  return new TextDecoder('utf-8', { fatal: false }).decode(bytes)
}
