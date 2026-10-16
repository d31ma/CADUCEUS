/**
 * CADUCEUS browser fetch cache.
 *
 * An IndexedDB read cache for low-sensitivity, slowly-changing GET endpoints.
 * This is a faithful port of Tachyon 26.23's `TacBrowserCache` — same policies
 * (`cache-first` / `network-first` / `reload` / `no-store`), same prefix-based
 * invalidation — exposed as a `window` global.
 *
 * Why a port instead of the framework class: this project bundles against a
 * *globally-installed* Tachyon and does all I/O through native `fetch` (see
 * `pages/tac.js` `apiFetch`), so Tachyon's optional browser-cache helper isn't
 * resolvable from a Tac companion. If Tachyon is ever vendored locally, the
 * caller can be pointed at the framework singleton with no behaviour change.
 *
 * SECURITY — this is for *non-sensitive* reads only:
 *   - Callers namespace keys per user (`caduceus:<email>:<path>`).
 *   - The shell clears the whole store on sign-out and purges it on boot when
 *     no session is present (IndexedDB outlives the sessionStorage token).
 *   - NEVER cache message bodies, attachments, or anything secret here —
 *     those stay on `no-store` via `apiFetch`.
 *
 * @module fetch-cache
 */

const DB_NAME = 'caduceus-fetch-cache'
const DB_VERSION = 1
const STORE = 'responses'

function isBrowser() {
  return typeof window !== 'undefined' && typeof indexedDB !== 'undefined'
}

/** @type {Promise<IDBDatabase | null> | null} */
let dbPromise = null

function open() {
  if (!isBrowser()) return Promise.resolve(null)
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve) => {
    let req
    try { req = indexedDB.open(DB_NAME, DB_VERSION) } catch { resolve(null); return }
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) db.createObjectStore(STORE, { keyPath: 'key' })
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => resolve(null)
  })
  return dbPromise
}

/** @param {string} key */
async function read(key) {
  const db = await open()
  if (!db) return null
  return new Promise((resolve) => {
    try {
      const r = db.transaction(STORE, 'readonly').objectStore(STORE).get(key)
      r.onsuccess = () => {
        const e = r.result
        if (!e) return resolve(null)
        resolve(new Response(e.body ? new Uint8Array(e.body) : null, {
          status: e.status, statusText: e.statusText, headers: e.headers,
        }))
      }
      r.onerror = () => resolve(null)
    } catch { resolve(null) }
  })
}

/** @param {string} key @param {Response} response */
async function write(key, response) {
  const db = await open()
  if (!db) return
  let body
  try { body = await response.clone().arrayBuffer() } catch { return }
  /** @type {Record<string,string>} */
  const headers = {}
  response.headers.forEach((v, k) => { headers[k] = v })
  await new Promise((resolve) => {
    try {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).put({ key, status: response.status, statusText: response.statusText, headers, body, updatedAt: Date.now() })
      tx.oncomplete = () => resolve(undefined)
      tx.onerror = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/** @param {string} key */
async function del(key) {
  const db = await open()
  if (!db) return
  await new Promise((resolve) => {
    try {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).delete(key)
      tx.oncomplete = () => resolve(undefined)
      tx.onerror = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/** @param {string} prefix */
async function deleteByPrefix(prefix) {
  const db = await open()
  if (!db) return
  await new Promise((resolve) => {
    try {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).delete(IDBKeyRange.bound(prefix, `${prefix}￿`))
      tx.oncomplete = () => resolve(undefined)
      tx.onerror = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/** @param {string[]} [keys] @param {string[]} [prefixes] */
async function invalidate(keys = [], prefixes = []) {
  await Promise.all([...keys.map(del), ...prefixes.map(deleteByPrefix)])
}

async function clearAll() {
  const db = await open()
  if (!db) return
  await new Promise((resolve) => {
    try {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).clear()
      tx.oncomplete = () => resolve(undefined)
      tx.onerror = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/**
 * Cache-aware fetch.
 *
 * @param {RequestInfo | URL} input
 * @param {RequestInit} [init]
 * @param {{ cache?: 'cache-first'|'network-first'|'reload'|'no-store', key?: string, invalidateKeys?: string[], invalidatePrefixes?: string[] }} [options]
 * @returns {Promise<Response>}
 */
async function cachedFetch(input, init = {}, options = {}) {
  const policy = options.cache || 'network-first'
  const key = options.key || (typeof input === 'string' ? input : String(input))

  if (options.invalidateKeys?.length || options.invalidatePrefixes?.length) {
    await invalidate(options.invalidateKeys, options.invalidatePrefixes)
  }

  if (!isBrowser() || policy === 'no-store') return fetch(input, init)

  if (policy === 'cache-first') {
    const cached = await read(key)
    if (cached) return cached
  }

  try {
    const response = await fetch(input, init)
    if (response.ok) write(key, response).catch(() => {})
    return response
  } catch (err) {
    // Network failed (offline, cold-start timeout, transient 5xx at fetch
    // layer): fall back to the last good cached copy when the policy allows.
    if (policy !== 'reload') {
      const cached = await read(key)
      if (cached) return cached
    }
    throw err
  }
}

window._caduceusCache = {
  fetch: cachedFetch,
  read,
  write,
  delete: del,
  deleteByPrefix,
  invalidate,
  clearAll,
}
