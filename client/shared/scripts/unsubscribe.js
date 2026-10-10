// @ts-check

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
