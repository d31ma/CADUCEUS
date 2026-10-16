const MAX_SHARE_TEXT = 100_000
const MAX_LAUNCH_RECIPIENTS = 100

/** @param {unknown} value @param {number} [max] */
function bounded(value, max = MAX_SHARE_TEXT) {
  return typeof value === 'string' ? value.slice(0, max).trim() : ''
}

/** @param {string} value */
function recipients(value) {
  return value.split(',')
    .map(item => item.trim())
    .filter(Boolean)
    .slice(0, MAX_LAUNCH_RECIPIENTS)
    .join(', ')
}

/**
 * Convert an OS protocol-handler mailto URL into the existing composer
 * prefill contract. Browser decoding is deliberately performed by URL and
 * URLSearchParams rather than by hand, avoiding double-decoding ambiguities.
 *
 * @param {unknown} value
 * @returns {Record<string, string>}
 */
export function mailtoPrefill(value) {
  if (typeof value !== 'string' || value.length > MAX_SHARE_TEXT) return {}
  let url
  try { url = new URL(value) } catch { return {} }
  if (url.protocol !== 'mailto:') return {}
  const to = recipients(decodeURIComponent(url.pathname || ''))
  const cc = recipients(url.searchParams.get('cc') || '')
  const subject = bounded(url.searchParams.get('subject'), 998)
  const body = bounded(url.searchParams.get('body'))
  return {
    ...(to ? { to } : {}),
    ...(cc ? { cc } : {}),
    ...(subject ? { subject } : {}),
    ...(body ? { body } : {}),
  }
}

/**
 * Parse only manifest-owned launch parameters. Ordinary arbitrary compose
 * query keys are ignored so a link cannot silently populate recipients.
 *
 * @param {unknown} search
 * @returns {Record<string, string>}
 */
export function composeLaunchPrefill(search) {
  if (typeof search !== 'string' || search.length > MAX_SHARE_TEXT * 2) return {}
  const params = new URLSearchParams(search)
  const protocol = params.get('protocol')
  if (protocol) return mailtoPrefill(protocol)

  const shareTitle = bounded(params.get('shareTitle'), 998)
  const shareText = bounded(params.get('shareText'))
  const shareUrl = bounded(params.get('shareUrl'), 8_192)
  let safeUrl = ''
  if (shareUrl) {
    try {
      const parsed = new URL(shareUrl)
      if (parsed.protocol === 'https:' || parsed.protocol === 'http:') safeUrl = parsed.href
    } catch {}
  }
  const body = [shareText, safeUrl].filter(Boolean).join('\n\n')
  return {
    ...(shareTitle ? { subject: shareTitle } : {}),
    ...(body ? { body } : {}),
  }
}

/** @param {unknown} search */
export function isManifestComposeLaunch(search) {
  if (typeof search !== 'string') return false
  const params = new URLSearchParams(search)
  return params.has('protocol') || params.has('shareTitle') || params.has('shareText') || params.has('shareUrl')
}
