// @ts-check

/**
 * Carrier for the composer's large requests: sends, drafts and draft loads.
 *
 * Tachyon's browser transport (every HTTPClient, in every language) caps a
 * request at 64 KiB and an answer at 1 MiB, and reports any error status as
 * HTTP_STATUS without its code or body. A message with attachments carries up
 * to 25 MB of base64, and the composer must read 409 conflicts and the
 * server's error text, so these few requests keep a plain same-origin fetch.
 * Every other compose request goes through the Go HTTPClients in
 * client/remotes.
 *
 * @param {string} path
 * @param {{ method?: string, body?: unknown, token: string }} options
 * @returns {Promise<{ ok: boolean, status: number, data: any }>}
 */
export async function mailRequest(path, { method = 'GET', body, token }) {
  const apiUrl = window.CADUCEUS_CONFIG?.apiUrl || ''
  const response = await fetch(apiUrl + path, {
    method,
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  let data = null
  try { data = await response.json() } catch { data = null }
  return { ok: response.ok, status: response.status, data }
}
