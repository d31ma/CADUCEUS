// @ts-check

/**
 * Session-bound encrypted persistence for sensitive offline writes.
 *
 * IndexedDB keeps the ciphertext durable across navigation while the raw
 * AES-GCM key lives only in sessionStorage. A browser restart or sign-out
 * therefore leaves no reusable decryption material behind. The unauthenticated
 * shell boot clears any orphaned ciphertext before another user can sign in.
 */

const DB_NAME = 'caduceus-offline-write-vault'
const DB_VERSION = 1
const WRITES_STORE = 'writes'
const SNAPSHOTS_STORE = 'snapshots'
const KEY_STORAGE = 'caduceus_offline_write_key_v1'
const LEGACY_MAIL_ACTIONS_STORAGE = 'caduceus_offline_mail_actions_v1'
const MAX_WRITES = 100
const MAX_WRITE_AGE_MS = 7 * 24 * 60 * 60 * 1000
const MAX_PLAINTEXT_BYTES = 50 * 1024 * 1024
const encoder = new TextEncoder()
const decoder = new TextDecoder()

/** @type {Promise<IDBDatabase | null> | null} */
let databasePromise = null
/** @type {Promise<CryptoKey | null> | null} */
let keyPromise = null
/** @type {Map<string, Promise<any>>} */
const activeFlushes = new Map()

function available() {
  return typeof indexedDB !== 'undefined'
    && typeof crypto?.subtle?.encrypt === 'function'
    && typeof sessionStorage !== 'undefined'
}

/** @returns {Promise<IDBDatabase | null>} */
function openDatabase() {
  if (!available()) return Promise.resolve(null)
  if (databasePromise) return databasePromise
  databasePromise = new Promise(resolve => {
    let request
    try { request = indexedDB.open(DB_NAME, DB_VERSION) } catch { resolve(null); return }
    request.onupgradeneeded = () => {
      const database = request.result
      if (!database.objectStoreNames.contains(WRITES_STORE)) database.createObjectStore(WRITES_STORE, { keyPath: 'id' })
      if (!database.objectStoreNames.contains(SNAPSHOTS_STORE)) database.createObjectStore(SNAPSHOTS_STORE, { keyPath: 'id' })
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => resolve(null)
  })
  return databasePromise
}

/** @param {Uint8Array} bytes */
function base64url(bytes) {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

/** @param {string} value */
function decodeBase64url(value) {
  const padded = value.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - value.length % 4) % 4)
  const binary = atob(padded)
  return Uint8Array.from(binary, character => character.charCodeAt(0))
}

/** @param {boolean} create */
async function encryptionKey(create) {
  if (!available()) return null
  if (keyPromise) return keyPromise
  keyPromise = (async () => {
    let encoded = sessionStorage.getItem(KEY_STORAGE) || ''
    if (!encoded && create) {
      const bytes = crypto.getRandomValues(new Uint8Array(32))
      encoded = base64url(bytes)
      sessionStorage.setItem(KEY_STORAGE, encoded)
    }
    if (!encoded) return null
    try {
      const bytes = decodeBase64url(encoded)
      if (bytes.byteLength !== 32) return null
      return await crypto.subtle.importKey('raw', bytes, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'])
    } catch { return null }
  })()
  return keyPromise
}

/** @param {string} value */
async function digest(value) {
  const bytes = await crypto.subtle.digest('SHA-256', encoder.encode(value))
  return base64url(new Uint8Array(bytes))
}

/** @param {string} aad @param {unknown} value */
async function seal(aad, value) {
  const key = await encryptionKey(true)
  if (!key) throw new Error('Encrypted offline storage is unavailable')
  const plaintext = encoder.encode(JSON.stringify(value))
  if (plaintext.byteLength > MAX_PLAINTEXT_BYTES) throw new Error('Offline write is too large')
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv, additionalData: encoder.encode(aad) }, key, plaintext)
  return { iv: iv.buffer, ciphertext }
}

/** @param {string} aad @param {{iv:ArrayBuffer,ciphertext:ArrayBuffer}} record */
async function unseal(aad, record) {
  const key = await encryptionKey(false)
  if (!key) throw new Error('Offline vault key is unavailable')
  const plaintext = await crypto.subtle.decrypt({
    name: 'AES-GCM', iv: new Uint8Array(record.iv), additionalData: encoder.encode(aad),
  }, key, record.ciphertext)
  return JSON.parse(decoder.decode(plaintext))
}

/** @param {string} storeName @returns {Promise<any[]>} */
async function all(storeName) {
  const database = await openDatabase()
  if (!database) return []
  return await new Promise(resolve => {
    try {
      const request = database.transaction(storeName, 'readonly').objectStore(storeName).getAll()
      request.onsuccess = () => resolve(Array.isArray(request.result) ? request.result : [])
      request.onerror = () => resolve([])
    } catch { resolve([]) }
  })
}

/** @param {string} storeName @param {Record<string,unknown>} value */
async function put(storeName, value) {
  const database = await openDatabase()
  if (!database) throw new Error('Encrypted offline storage is unavailable')
  await new Promise((resolve, reject) => {
    try {
      const transaction = database.transaction(storeName, 'readwrite')
      transaction.objectStore(storeName).put(value)
      transaction.oncomplete = () => resolve(undefined)
      transaction.onerror = () => reject(transaction.error || new Error('Could not persist offline data'))
      transaction.onabort = () => reject(transaction.error || new Error('Could not persist offline data'))
    } catch (error) { reject(error) }
  })
}

/** @param {string} storeName @param {string} id */
async function remove(storeName, id) {
  const database = await openDatabase()
  if (!database) return
  await new Promise(resolve => {
    try {
      const transaction = database.transaction(storeName, 'readwrite')
      transaction.objectStore(storeName).delete(id)
      transaction.oncomplete = () => resolve(undefined)
      transaction.onerror = () => resolve(undefined)
      transaction.onabort = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/** @param {string} storeName */
async function clearStore(storeName) {
  const database = await openDatabase()
  if (!database) return
  await new Promise(resolve => {
    try {
      const transaction = database.transaction(storeName, 'readwrite')
      transaction.objectStore(storeName).clear()
      transaction.oncomplete = () => resolve(undefined)
      transaction.onerror = () => resolve(undefined)
      transaction.onabort = () => resolve(undefined)
    } catch { resolve(undefined) }
  })
}

/** @param {string} path @param {string} method */
function validateRequest(path, method) {
  if (!/^\/(?!\/)[^\r\n\0]{1,499}$/.test(path)) throw new Error('Offline request path is invalid')
  if (!['POST', 'PUT', 'DELETE'].includes(method)) throw new Error('Offline request method is invalid')
}

/** @param {OfflineWriteInput} input */
export async function queueOfflineWrite(input) {
  const owner = String(input.owner || '').trim().toLowerCase()
  const path = String(input.path || '')
  const method = String(input.method || '').toUpperCase()
  if (!owner || !owner.includes('@')) throw new Error('Offline write owner is required')
  validateRequest(path, method)
  const id = input.id || crypto.randomUUID()
  const ownerHash = await digest(owner)
  const coalesceHash = input.coalesceKey ? await digest(`${owner}\0${input.coalesceKey}`) : ''
  const createdAt = Date.now()
  const payload = {
    path,
    method,
    body: input.body ?? null,
    message: String(input.message || 'Offline write').slice(0, 200),
    metadata: input.metadata && typeof input.metadata === 'object' ? input.metadata : {},
  }
  const aad = `${id}\0${ownerHash}\0${createdAt}`
  const encrypted = await seal(aad, payload)
  const existing = await all(WRITES_STORE)
  if (coalesceHash) {
    await Promise.all(existing
      .filter(record => record.ownerHash === ownerHash && record.coalesceHash === coalesceHash && record.state !== 'conflict')
      .map(record => remove(WRITES_STORE, record.id)))
  }
  await put(WRITES_STORE, {
    id, ownerHash, coalesceHash, kind: input.kind, state: 'queued', createdAt,
    expiresAt: createdAt + MAX_WRITE_AGE_MS, ...encrypted,
  })
  const current = (await all(WRITES_STORE)).sort((left, right) => left.createdAt - right.createdAt)
  await Promise.all(current.slice(0, Math.max(0, current.length - MAX_WRITES)).map(record => remove(WRITES_STORE, record.id)))
  void navigator.serviceWorker?.ready
    .then(registration => /** @type {any} */ (registration).sync?.register?.('caduceus-offline-writes'))
    .catch(() => {})
  return { id, kind: input.kind, createdAt }
}

/**
 * Replay writes in creation order. A retryable failure or conflict stops the
 * pass so a later mutation cannot overtake the state it was based on.
 *
 * @param {string} owner
 * @param {(write: Record<string,any>) => Promise<{status:'accepted'|'retry'|'conflict'|'permanent',response?:unknown}>} send
 */
export async function flushOfflineWrites(owner, send) {
  const ownerHash = await digest(String(owner || '').toLowerCase())
  const active = activeFlushes.get(ownerHash)
  if (active) return await active
  const operation = typeof navigator?.locks?.request === 'function'
    ? navigator.locks.request(`caduceus-offline-write-flush:${ownerHash}`, () => flushOwner(ownerHash, send))
    : flushOwner(ownerHash, send)
  activeFlushes.set(ownerHash, operation)
  try { return await operation } finally { activeFlushes.delete(ownerHash) }
}

/** @param {string} ownerHash @param {(write: Record<string,any>) => Promise<{status:'accepted'|'retry'|'conflict'|'permanent',response?:unknown}>} send */
async function flushOwner(ownerHash, send) {
  const records = (await all(WRITES_STORE))
    .filter(record => record.ownerHash === ownerHash)
    .sort((left, right) => left.createdAt - right.createdAt)
  const summary = { completed: 0, failed: 0, conflicts: 0, results: /** @type {any[]} */ ([]) }
  for (const record of records) {
    if (record.expiresAt < Date.now()) {
      await remove(WRITES_STORE, record.id)
      summary.failed += 1
      summary.results.push({ id: record.id, kind: record.kind, status: 'expired' })
      continue
    }
    if (record.state === 'conflict') { summary.conflicts += 1; break }
    let payload
    try { payload = await unseal(`${record.id}\0${record.ownerHash}\0${record.createdAt}`, record) } catch {
      await remove(WRITES_STORE, record.id)
      summary.failed += 1
      summary.results.push({ id: record.id, kind: record.kind, status: 'unreadable' })
      continue
    }
    let outcome
    try { outcome = await send({ id: record.id, kind: record.kind, ...payload }) } catch { break }
    if (outcome.status === 'retry') break
    if (outcome.status === 'conflict') {
      await put(WRITES_STORE, { ...record, state: 'conflict' })
      summary.conflicts += 1
      summary.results.push({ id: record.id, kind: record.kind, status: 'conflict', response: outcome.response, ...payload })
      break
    }
    await remove(WRITES_STORE, record.id)
    const status = outcome.status === 'accepted' ? 'accepted' : 'failed'
    if (status === 'accepted') summary.completed += 1
    else summary.failed += 1
    summary.results.push({ id: record.id, kind: record.kind, status, response: outcome.response, ...payload })
  }
  return summary
}

/** @param {string} owner */
export async function pendingOfflineWrites(owner) {
  const ownerHash = await digest(String(owner || '').toLowerCase())
  return (await all(WRITES_STORE)).filter(record => record.ownerHash === ownerHash).length
}

/** @param {string} id */
export async function discardOfflineWrite(id) {
  await remove(WRITES_STORE, id)
}

/** @param {string} owner @param {string} coalesceKey */
export async function discardOfflineWrites(owner, coalesceKey) {
  const normalized = String(owner || '').toLowerCase()
  const ownerHash = await digest(normalized)
  const coalesceHash = await digest(`${normalized}\0${coalesceKey}`)
  const records = await all(WRITES_STORE)
  await Promise.all(records
    .filter(record => record.ownerHash === ownerHash && record.coalesceHash === coalesceHash)
    .map(record => remove(WRITES_STORE, record.id)))
}

/** @param {string} owner @param {string} name @param {unknown} value */
export async function saveOfflineSnapshot(owner, name, value) {
  const ownerHash = await digest(String(owner || '').toLowerCase())
  const id = await digest(`${ownerHash}\0${name}`)
  const updatedAt = Date.now()
  const encrypted = await seal(`${id}\0${ownerHash}\0${updatedAt}`, value)
  await put(SNAPSHOTS_STORE, { id, ownerHash, updatedAt, ...encrypted })
}

/** @param {string} owner @param {string} name */
export async function loadOfflineSnapshot(owner, name) {
  const ownerHash = await digest(String(owner || '').toLowerCase())
  const id = await digest(`${ownerHash}\0${name}`)
  const record = (await all(SNAPSHOTS_STORE)).find(candidate => candidate.id === id && candidate.ownerHash === ownerHash)
  if (!record) return null
  try { return await unseal(`${id}\0${ownerHash}\0${record.updatedAt}`, record) } catch {
    await remove(SNAPSHOTS_STORE, id)
    return null
  }
}

/** @param {string} owner @param {string} name */
export async function removeOfflineSnapshot(owner, name) {
  const ownerHash = await digest(String(owner || '').toLowerCase())
  await remove(SNAPSHOTS_STORE, await digest(`${ownerHash}\0${name}`))
}

export async function clearOfflineWriteVault() {
  activeFlushes.clear()
  await Promise.all([clearStore(WRITES_STORE), clearStore(SNAPSHOTS_STORE)])
  try { sessionStorage.removeItem(KEY_STORAGE) } catch {}
  try { localStorage.removeItem(LEGACY_MAIL_ACTIONS_STORAGE) } catch {}
  keyPromise = null
}

/** Import and then erase the former plaintext localStorage action queue. */
export async function migrateLegacyOfflineMailActions(owner) {
  let legacy
  try { legacy = JSON.parse(localStorage.getItem(LEGACY_MAIL_ACTIONS_STORAGE) || '[]') } catch { legacy = [] }
  if (!Array.isArray(legacy) || legacy.length === 0) return 0
  let migrated = 0
  for (const item of legacy) {
    if (!item || String(item.owner || '').toLowerCase() !== String(owner || '').toLowerCase()
      || !Array.isArray(item.ids) || !item.patch || typeof item.patch !== 'object') continue
    await queueOfflineWrite({
      id: typeof item.clientRequestId === 'string' ? item.clientRequestId : undefined,
      kind: 'mail-action', owner, path: '/inbox/actions', method: 'POST',
      body: { ids: item.ids, patch: item.patch, clientRequestId: item.clientRequestId || crypto.randomUUID() },
      message: String(item.message || 'Mail action'),
    })
    migrated += 1
  }
  try { localStorage.removeItem(LEGACY_MAIL_ACTIONS_STORAGE) } catch {}
  return migrated
}
