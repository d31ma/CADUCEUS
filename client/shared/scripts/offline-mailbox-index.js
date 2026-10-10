// @ts-check

import {
  loadOfflineSnapshot,
  removeOfflineSnapshot,
  saveOfflineSnapshot,
} from './offline-write-vault.js'

const INDEX_NAME = 'mailbox-index:v1'
const MAX_MESSAGES = 500
const MAX_RECORD_BYTES = 8 * 1024 * 1024
const MAX_ATTACHMENT_BYTES = 8 * 1024 * 1024
const MAX_SYNC_ATTACHMENT_BYTES = 32 * 1024 * 1024
const encoder = new TextEncoder()

/** @param {unknown} value */
function safeClone(value) {
  try { return JSON.parse(JSON.stringify(value)) } catch { return null }
}

/** @param {unknown} value */
function boundedObject(value) {
  const cloned = safeClone(value)
  if (!cloned || typeof cloned !== 'object' || Array.isArray(cloned)) return null
  try {
    return encoder.encode(JSON.stringify(cloned)).byteLength <= MAX_RECORD_BYTES ? cloned : null
  } catch { return null }
}

/** @param {unknown} value */
function safeID(value) {
  const id = String(value || '')
  return id && id.length <= 512 && !/[\0\r\n]/.test(id) ? id : ''
}

/** @param {string} id */
function messageName(id) { return `mailbox-message:v1:${id}` }

/** @param {string} messageID @param {string} attachmentID */
function attachmentName(messageID, attachmentID) { return `mailbox-attachment:v1:${messageID}:${attachmentID}` }

/** @param {Record<string, any>} message */
export function offlineMailboxSummary(message) {
  const copy = boundedObject(message)
  if (!copy) return null
  for (const field of ['body', 'htmlBody', 'plainText', 'textBody', 'rawRFC822Base64', 'calendarInvite']) delete copy[field]
  return copy
}

/** @param {Record<string, any>} message */
function searchableText(message) {
  const addresses = [message.sender, message.from, message.to, message.cc, message.bcc]
    .flatMap(value => Array.isArray(value) ? value : [value])
    .map(value => typeof value === 'string' ? value : `${value?.name || ''} ${value?.address || value?.email || ''}`)
  const attachments = Array.isArray(message.attachments) ? message.attachments.map(item => item?.filename || '') : []
  return [message.subject, message.snippet, message.body, message.plainText, message.textBody, ...addresses, ...attachments]
    .map(value => String(value || '').toLowerCase()).join('\n').slice(0, 2_000_000)
}

/** @returns {{version:number,updatedAt:number,messages:Array<Record<string,any>>}} */
function emptyIndex() { return { version: 1, updatedAt: Date.now(), messages: [] } }

/** @param {unknown} value */
export function normalizeOfflineMailboxIndex(value) {
  if (!value || typeof value !== 'object') return emptyIndex()
  const messages = Array.isArray(/** @type {any} */ (value).messages)
    ? /** @type {any} */ (value).messages.filter(item => safeID(item?.id) && item?.message && typeof item.message === 'object').slice(0, MAX_MESSAGES)
    : []
  return { version: 1, updatedAt: Number(/** @type {any} */ (value).updatedAt) || Date.now(), messages }
}

/** @param {string} owner */
async function loadIndex(owner) {
  return normalizeOfflineMailboxIndex(await loadOfflineSnapshot(owner, INDEX_NAME))
}

/** @param {string} owner @param {{version:number,updatedAt:number,messages:Array<Record<string,any>>}} index */
async function saveIndex(owner, index) {
  index.updatedAt = Date.now()
  await saveOfflineSnapshot(owner, INDEX_NAME, index)
}

/** @param {Record<string,any>} message @param {Record<string,any>|undefined} previous */
function indexEntry(message, previous) {
  const summary = offlineMailboxSummary(message)
  const id = safeID(summary?.id)
  if (!summary || !id) return null
  return {
    id,
    message: summary,
    searchText: previous?.detailAvailable ? previous.searchText : searchableText(summary),
    detailAvailable: previous?.detailAvailable === true,
    attachmentIds: Array.isArray(previous?.attachmentIds) ? previous.attachmentIds.filter(safeID) : [],
    receivedAt: String(summary.receivedAt || summary.date || ''),
    updatedAt: Date.now(),
  }
}

/**
 * Merge authoritative list summaries into the encrypted local index.
 * @param {string} owner
 * @param {Array<Record<string,any>>} messages
 * @param {{replace?:boolean}} [options]
 */
export async function cacheOfflineMailboxSummaries(owner, messages, { replace = false } = {}) {
  if (!owner || !Array.isArray(messages)) return 0
  const index = await loadIndex(owner)
  const previous = new Map(index.messages.map(item => [item.id, item]))
  /** @type {Array<Record<string,any>>} */
  const entries = []
  for (const message of messages) {
    const entry = indexEntry(message, previous.get(String(message?.id || '')))
    if (entry) entries.push(entry)
  }
  const merged = replace ? entries : [...entries, ...index.messages.filter(item => !entries.some(entry => entry.id === item.id))]
  merged.sort((left, right) => String(right.receivedAt).localeCompare(String(left.receivedAt)) || left.id.localeCompare(right.id))
  const retained = merged.slice(0, MAX_MESSAGES)
  const retainedIDs = new Set(retained.map(item => item.id))
  await saveIndex(owner, { version: 1, updatedAt: Date.now(), messages: retained })
  await Promise.all(index.messages.filter(item => !retainedIDs.has(item.id)).map(item => removeOfflineMessage(owner, item)))
  return retained.length
}

/** @param {string} owner @param {Record<string,any>} message */
export async function cacheOfflineMailboxMessage(owner, message) {
  const id = safeID(message?.id)
  const detail = boundedObject(message)
  if (!owner || !id || !detail) return false
  await saveOfflineSnapshot(owner, messageName(id), detail)
  const index = await loadIndex(owner)
  const previous = index.messages.find(item => item.id === id)
  const entry = indexEntry(detail, previous)
  if (!entry) return false
  entry.detailAvailable = true
  entry.searchText = searchableText(detail)
  entry.attachmentIds = Array.isArray(detail.attachments) ? detail.attachments.map(item => safeID(item?.id)).filter(Boolean) : []
  index.messages = [entry, ...index.messages.filter(item => item.id !== id)].slice(0, MAX_MESSAGES)
  await saveIndex(owner, index)
  return true
}

/** @param {string} owner @param {string} id */
export async function loadOfflineMailboxMessage(owner, id) {
  id = safeID(id)
  if (!owner || !id) return null
  return boundedObject(await loadOfflineSnapshot(owner, messageName(id)))
}

/** @param {string} owner @param {string} messageID @param {Record<string,any>} attachment */
export async function cacheOfflineAttachment(owner, messageID, attachment) {
  messageID = safeID(messageID)
  const attachmentID = safeID(attachment?.id)
  const payload = boundedObject(attachment)
  if (!owner || !messageID || !attachmentID || !payload || typeof payload.contentBase64 !== 'string') return false
  if (Math.ceil(payload.contentBase64.length * 0.75) > MAX_ATTACHMENT_BYTES) return false
  await saveOfflineSnapshot(owner, attachmentName(messageID, attachmentID), payload)
  return true
}

/** @param {string} owner @param {string} messageID @param {string} attachmentID */
export async function loadOfflineAttachment(owner, messageID, attachmentID) {
  messageID = safeID(messageID); attachmentID = safeID(attachmentID)
  if (!owner || !messageID || !attachmentID) return null
  return boundedObject(await loadOfflineSnapshot(owner, attachmentName(messageID, attachmentID)))
}

/** @param {string} owner @param {Record<string,any>} entry */
async function removeOfflineMessage(owner, entry) {
  await removeOfflineSnapshot(owner, messageName(entry.id))
  await Promise.all((entry.attachmentIds || []).map(id => removeOfflineSnapshot(owner, attachmentName(entry.id, id))))
}

/** @param {string} query */
function queryTokens(query) {
  return String(query || '').match(/(?:[^\s"]+|"[^"]*")+/g)?.map(value => value.replace(/^"|"$/g, '').toLowerCase()).filter(Boolean) || []
}

/** @param {Record<string,any>} entry @param {string} token */
function matchesToken(entry, token) {
  const split = token.indexOf(':')
  if (split < 1) return entry.searchText.includes(token)
  const field = token.slice(0, split); const value = token.slice(split + 1)
  const message = entry.message
  if (field === 'subject') return String(message.subject || '').toLowerCase().includes(value)
  if (field === 'from') return searchableText({ sender: message.sender, from: message.from }).includes(value)
  if (field === 'to') return searchableText({ to: message.to, cc: message.cc, bcc: message.bcc }).includes(value)
  if (field === 'is') return matchesState(message, value)
  if (field === 'has' && value === 'attachment') return Boolean(message.hasAttachments || message.attachments?.length)
  if (field === 'before' || field === 'after') return matchesDate(message, field, value)
  return entry.searchText.includes(token)
}

/** @param {Record<string,any>} message @param {string} value */
function matchesState(message, value) {
  if (value === 'read') return message.read === true
  if (value === 'unread') return message.read !== true
  if (value === 'starred') return message.starred === true
  if (value === 'unstarred') return message.starred !== true
  return false
}

/** @param {Record<string,any>} message @param {string} field @param {string} value */
function matchesDate(message, field, value) {
  const actual = Date.parse(message.receivedAt || message.date || '')
  const expected = Date.parse(value)
  return Number.isFinite(actual) && Number.isFinite(expected) && (field === 'before' ? actual < expected : actual > expected)
}

/** @param {Record<string,any>} message @param {Record<string,any>} filter */
function matchesScope(message, filter) {
  const folder = String(message.systemFolder || message.folder || 'inbox').toLowerCase()
  if (filter.folder && folder !== String(filter.folder).toLowerCase()) return false
  if (filter.domain && String(message.domain || '').toLowerCase() !== String(filter.domain).toLowerCase()) return false
  if (filter.account && String(message.providerAccountId || '') !== String(filter.account)) return false
  if (filter.read != null && message.read !== filter.read) return false
  if (filter.starred != null && message.starred !== filter.starred) return false
  return true
}

/** @param {string} owner @param {{query?:string,folder?:string,domain?:string,account?:string,read?:boolean,starred?:boolean,limit?:number}} [filter] */
export async function searchOfflineMailbox(owner, filter = {}) {
	return searchOfflineMailboxIndex(await loadIndex(owner), filter)
}

/** @param {unknown} value @param {{query?:string,folder?:string,domain?:string,account?:string,read?:boolean,starred?:boolean,limit?:number}} [filter] */
export function searchOfflineMailboxIndex(value, filter = {}) {
	const tokens = queryTokens(filter.query || '')
	const index = normalizeOfflineMailboxIndex(value)
	const limit = Math.max(1, Math.min(MAX_MESSAGES, Number(filter.limit) || 100))
	return index.messages
    .filter(entry => matchesScope(entry.message, filter) && tokens.every(token => matchesToken(entry, token)))
    .slice(0, limit)
		.map(entry => safeClone(entry.message))
}

/**
 * Hydrate a bounded slice of missing details and attachment bytes. The caller
 * invokes additional slices from idle callbacks; tokens remain in page memory.
 * @param {string} owner
 * @param {(path:string)=>Promise<Response|null>} fetcher
 * @param {{maxMessages?:number}} [options]
 */
export async function hydrateOfflineMailbox(owner, fetcher, { maxMessages = 25 } = {}) {
  const index = await loadIndex(owner)
  const pending = index.messages.filter(item => !item.detailAvailable).slice(0, Math.max(1, Math.min(50, maxMessages)))
  let completed = 0; let attachmentBytes = 0
  for (const entry of pending) {
    let response
    try { response = await fetcher(`/inbox/${encodeURIComponent(entry.id)}`) } catch { break }
    if (!response?.ok) continue
    const detail = await response.json().catch(() => null)
    if (!detail?.id || detail.id !== entry.id || !await cacheOfflineMailboxMessage(owner, detail)) continue
    completed++
    for (const attachment of detail.attachments || []) {
      if (attachmentBytes >= MAX_SYNC_ATTACHMENT_BYTES) break
      const size = Number(attachment?.size) || 0
      if (!safeID(attachment?.id) || size > MAX_ATTACHMENT_BYTES || attachmentBytes + size > MAX_SYNC_ATTACHMENT_BYTES) continue
      let attachmentResponse
      try { attachmentResponse = await fetcher(`/inbox/${encodeURIComponent(entry.id)}/attachments/${encodeURIComponent(attachment.id)}`) } catch { break }
      if (!attachmentResponse?.ok) continue
      const payload = await attachmentResponse.json().catch(() => null)
      if (await cacheOfflineAttachment(owner, entry.id, payload)) attachmentBytes += size
    }
  }
  return { completed, remaining: Math.max(0, index.messages.filter(item => !item.detailAvailable).length - completed), attachmentBytes }
}
