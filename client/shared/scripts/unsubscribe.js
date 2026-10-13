// @ts-check

/**
 * Convert persisted List-Unsubscribe metadata into one bounded reader action.
 * The browser validates the server-provided fields again before navigating or
 * sending a cross-origin request.
 *
 * @param {Record<string, any> | null | undefined} email
 * @returns {{ type: 'post'|'open', url: string } | { type: 'compose', to: string[], subject: string, body: string } | null}
 */
export function buildUnsubscribeAction(email) {
  const url = secureWebUrl(email?.unsubscribeUrl)
  if (url && email?.unsubscribeOneClick === true) return { type: 'post', url }

  const mailto = mailtoAction(email?.unsubscribeMailto)
  if (mailto) return mailto
  if (url) return { type: 'open', url }

  const bodyUrl = secureWebUrl(email?.unsubscribeBodyUrl)
  return bodyUrl ? { type: 'open', url: bodyUrl } : null
}

/** @param {unknown} value */
function secureWebUrl(value) {
  if (typeof value !== 'string' || value.length > 2_048) return ''
  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password) return ''
    return parsed.href
  } catch {
    return ''
  }
}

/**
 * @param {unknown} value
 * @returns {{ type: 'compose', to: string[], subject: string, body: string } | null}
 */
function mailtoAction(value) {
  if (typeof value !== 'string' || value.length > 2_048) return null
  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'mailto:' || parsed.username || parsed.password || parsed.host) return null
    const mailbox = decodeURIComponent(parsed.pathname).trim().toLowerCase()
    if (!/^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/.test(mailbox)) return null
    return {
      type: 'compose',
      to: [mailbox],
      subject: boundedText(parsed.searchParams.get('subject')),
      body: boundedText(parsed.searchParams.get('body')),
    }
  } catch {
    return null
  }
}

/** @param {string | null} value */
function boundedText(value) {
  return typeof value === 'string' && value.length <= 2_000 && !/[\r\n]/.test(value)
    ? value
    : ''
}

/**
 * Send one RFC 8058 List-Unsubscribe-Post request. The target is a validated
 * https URL of the message sender, so the request is an opaque cross-origin
 * POST that carries no credentials or referrer and whose answer is never
 * read. The Go reader decides whether a message offers this action.
 *
 * @param {string} url
 * @returns {Promise<boolean>}
 */
export async function postOneClickUnsubscribe(url) {
  try {
    await fetch(url, {
      method: 'POST',
      mode: 'no-cors',
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: 'List-Unsubscribe=One-Click',
    })
    return true
  } catch {
    return false
  }
}
