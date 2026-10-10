// @ts-check

const CONTROL_OR_DIRECTIONAL = /[\u0000-\u001f\u007f-\u009f\u202a-\u202e\u2066-\u2069]/g
const HAS_CONTROL_OR_DIRECTIONAL = /[\u0000-\u001f\u007f-\u009f\u202a-\u202e\u2066-\u2069]/
const RESERVED_PATH_CHARACTER = /[/\\?%*:|"<>]/g

/**
 * Convert an untrusted MIME filename into one portable display/download name.
 * Path separators, control characters, and bidirectional overrides never
 * reach storage keys, MIME output, HTML, or a browser download attribute.
 *
 * @param {unknown} value
 * @param {string} [fallback]
 */
export function sanitizeAttachmentFilename(value, fallback = 'attachment') {
  const untrusted = String(value || '')
    .normalize('NFKC')
    .replace(CONTROL_OR_DIRECTIONAL, '')
  const leaf = untrusted.split(/[/\\]/).filter(Boolean).at(-1) || ''
  const normalized = leaf
    .replace(RESERVED_PATH_CHARACTER, '-')
    .replace(/\s+/g, ' ')
    .replace(/^\.+/, '')
    .replace(/[. ]+$/, '')
    .trim()
  return (normalized || fallback).slice(0, 160)
}

/**
 * Normalize an RFC Content-ID used by a `cid:` URL. Angle brackets and the
 * optional URL prefix are removed; whitespace, controls, and overlong values
 * are rejected so IDs can safely become exact lookup keys.
 *
 * @param {unknown} value
 * @returns {string | undefined}
 */
export function normalizeAttachmentContentId(value) {
  if (value == null || value === '') return undefined
  let normalized = String(value).trim().replace(/^cid:/i, '')
  if (normalized.startsWith('<') && normalized.endsWith('>')) normalized = normalized.slice(1, -1).trim()
  if (!normalized || normalized.length > 255) return undefined
  if (HAS_CONTROL_OR_DIRECTIONAL.test(normalized) || /[\s<>]/.test(normalized)) return undefined
  return normalized
}
