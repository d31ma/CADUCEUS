// @ts-check

import { formatExportBytes, mboxFilename, writeMboxFromFrameStream } from './mbox.js'
import { emlArchiveFilename, writeEmlArchiveFromFrameStream } from './eml-archive.js'
import { cacheOfflineMailboxSummaries, searchOfflineMailbox } from './offline-mailbox-index.js'

/**
 * The byte-moving half of the mailbox archive tools (MailboxArchiveService).
 *
 * What to export or import and how to word it is decided in Go
 * (MailboxTransferService). These pipelines stay in the browser because an
 * HTTPClient cannot carry them: the export is a binary RFC 822 frame stream
 * with no size bound, written through the File System Access API or a Blob
 * download, and an import uploads files of up to 50 MiB. A Tac service may not
 * call fetch (TY1361), so the requests are made here, to the page's own
 * origin, with the session's bearer token, as the browser transport does.
 * The encrypted offline index (offline-mailbox-index.js) is reached here too.
 */

const FRAMES = 'application/vnd.caduceus.rfc822-frames; version=1'
const MAX_MEMORY_EXPORT_BYTES = 256 * 1024 * 1024
const MAX_EML_IMPORT_BYTES = 50 * 1024 * 1024
const OFFLINE_SLICE = 25

/**
 * Stream the folder into the chosen file. Chromium's File System Access API
 * receives one record at a time and only commits on close(); other browsers
 * keep the chunks until a download can be offered. Cancellation and failure
 * discard the incomplete output.
 *
 * @param {ExportRequested} request
 * @param {AbortSignal} signal
 * @param {(messages: number, bytesLabel: string) => void} progress
 * @param {HTMLAnchorElement | null} link
 * @returns {Promise<ExportFinished>}
 */
export async function exportFolder(request, signal, progress, link) {
  const eml = request.format === 'eml'
  const filename = eml ? emlArchiveFilename(request.folder, request.filenameDomain) : mboxFilename(request.folder, request.filenameDomain)
  /** @type {any} */
  let writable = null
  /** @type {Uint8Array[]} */
  const chunks = []
  let memoryBytes = 0
  let messages = 0
  let sourceBytes = 0
  let outputBytes = 0
  try {
    const picker = /** @type {any} */ (window).showSaveFilePicker
    if (typeof picker === 'function') {
      const handle = await picker({
        suggestedName: filename,
        types: eml
          ? [{ description: 'EML message archive', accept: { 'application/zip': ['.zip'] } }]
          : [{ description: 'Mailbox file', accept: { 'application/mbox': ['.mbox'] } }],
      })
      writable = await handle.createWritable()
    }
    for (const scope of request.scopes) {
      signal.throwIfAborted()
      const body = await openExportStream(request.folder, scope, request.authorization, signal)
      const consume = eml ? writeEmlArchiveFromFrameStream : writeMboxFromFrameStream
      const result = await consume(body, chunk => {
        if (writable) return writable.write(chunk)
        if (memoryBytes + chunk.byteLength > MAX_MEMORY_EXPORT_BYTES) {
          throw new Error(`This browser limits in-memory mailbox exports to ${formatExportBytes(MAX_MEMORY_EXPORT_BYTES)}. Use a browser with the File System Access API.`)
        }
        chunks.push(chunk)
        memoryBytes += chunk.byteLength
      }, {
        signal,
        onProgress: step => progress(messages + step.messages, formatExportBytes(outputBytes + step.outputBytes)),
      })
      messages += result.messages
      sourceBytes += result.sourceBytes
      outputBytes += result.outputBytes
    }
    if (messages === 0) {
      if (writable) await writable.abort()
      return { messages: 0, sourceBytesLabel: formatExportBytes(0), canceled: false, failure: '' }
    }
    if (writable) await writable.close()
    else download(chunks, filename, eml ? 'application/zip' : 'application/mbox', link)
    return { messages, sourceBytesLabel: formatExportBytes(sourceBytes), canceled: false, failure: '' }
  } catch (error) {
    if (writable) {
      try { await writable.abort() } catch {}
    }
    const canceled = isAbort(error, signal)
    return { messages, sourceBytesLabel: formatExportBytes(sourceBytes), canceled, failure: canceled ? '' : message(error, 'Mailbox export failed.') }
  }
}

/** @param {string} folder @param {{ domain: string, account: string }} scope @param {string} authorization @param {AbortSignal} signal */
async function openExportStream(folder, scope, authorization, signal) {
  const params = new URLSearchParams(scope.account ? { folder, account: scope.account } : { folder, domain: scope.domain })
  const response = await fetch(`/inbox/export/rfc822?${params}`, {
    headers: { Accept: FRAMES, Authorization: authorization }, credentials: 'omit', signal,
  })
  if (!response.ok || !response.body) {
    let text = `Mailbox export failed (${response.status}).`
    try { text = (await response.clone().json())?.error || text } catch {}
    throw new Error(text)
  }
  return response.body
}

/** @param {Uint8Array[]} chunks @param {string} filename @param {string} contentType @param {HTMLAnchorElement | null} link */
function download(chunks, filename, contentType, link) {
  if (!link) throw new Error('Mailbox export cannot be saved from this page.')
  const blob = new Blob(chunks.map(chunk => Uint8Array.from(chunk).buffer), { type: contentType })
  const url = URL.createObjectURL(blob)
  link.href = url
  link.download = filename
  link.click()
  link.removeAttribute('href')
  setTimeout(() => URL.revokeObjectURL(url), 0)
}

/**
 * Import the chosen EML files one after another, so progress, cancellation
 * and per-file atomicity stay predictable for large mailboxes.
 *
 * @param {File[]} files
 * @param {ImportPickRequested} target
 * @param {AbortSignal} signal
 * @param {(imported: number, total: number, current: string) => void} progress
 * @returns {Promise<ImportFinished>}
 */
export async function importFiles(files, target, signal, progress) {
  let imported = 0
  /** @type {string[]} */
  const failures = []
  try {
    for (const file of files) {
      signal.throwIfAborted()
      progress(imported, files.length, file.name)
      if (file.size === 0 || file.size > MAX_EML_IMPORT_BYTES) {
        failures.push(`${file.name}: file must be between 1 byte and 50 MiB`)
        continue
      }
      try {
        const contentBase64 = await readBase64(file, signal)
        await uploadEml({ mailbox: target.mailbox, folder: target.folder, contentBase64 }, target.authorization, signal)
        imported += 1
      } catch (error) {
        if (signal.aborted) throw error
        failures.push(`${file.name}: ${message(error, 'import failed')}`)
      }
    }
    return { imported, failures: failures.length, firstFailure: failures[0] || '', canceled: false, failure: '' }
  } catch (error) {
    const canceled = isAbort(error, signal)
    return { imported, failures: failures.length, firstFailure: failures[0] || '', canceled, failure: canceled ? '' : message(error, 'EML import failed.') }
  }
}

/** @param {{ mailbox: string, folder: string, contentBase64: string }} body @param {string} authorization @param {AbortSignal} signal */
async function uploadEml(body, authorization, signal) {
  const response = await fetch('/inbox/import/eml', {
    method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: authorization },
    body: JSON.stringify(body), credentials: 'omit', signal,
  })
  const result = await response.json().catch(() => ({}))
  if (!response.ok || !result?.imported) throw new Error(result?.error || `Import failed (${response.status || 0})`)
}

/** @param {File} file @param {AbortSignal} signal @returns {Promise<string>} */
function readBase64(file, signal) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const abort = () => reader.abort()
    signal.addEventListener('abort', abort, { once: true })
    reader.onerror = () => {
      signal.removeEventListener('abort', abort)
      reject(reader.error || new Error('Could not read EML file'))
    }
    reader.onabort = () => {
      signal.removeEventListener('abort', abort)
      reject(signal.reason || new DOMException('EML import canceled', 'AbortError'))
    }
    reader.onload = () => {
      signal.removeEventListener('abort', abort)
      const result = String(reader.result || '')
      const comma = result.indexOf(',')
      if (comma < 0) reject(new Error('Could not encode EML file'))
      else resolve(result.slice(comma + 1))
    }
    reader.readAsDataURL(file)
  })
}

/** @param {unknown} error @param {AbortSignal} signal */
function isAbort(error, signal) {
  return signal.aborted || (error instanceof DOMException && error.name === 'AbortError')
}

/** @param {unknown} error @param {string} fallback */
function message(error, fallback) {
  return error instanceof Error && error.message ? error.message : fallback
}

/**
 * Keep loaded summaries in the encrypted offline index.
 *
 * @param {string} owner
 * @param {Array<Record<string, any>>} emails
 */
export function rememberSummaries(owner, emails) {
  return cacheOfflineMailboxSummaries(owner, emails).catch(() => 0)
}

/**
 * Search the offline index, in slices that each fit one event.
 *
 * @param {{ owner: string, query: string, folder: string, domain: string, account: string, read: string, starred: string, limit: number }} request
 * @returns {Promise<Array<Array<Record<string, any>>>>}
 */
export async function searchOffline(request) {
  /** @type {Array<Record<string, any>>} */
  let found = []
  try {
    found = await searchOfflineMailbox(request.owner, {
      query: request.query || undefined, folder: request.query ? undefined : request.folder || undefined,
      domain: request.domain, account: request.account,
      read: request.read === '' ? undefined : request.read === 'true',
      starred: request.starred === '' ? undefined : request.starred === 'true',
      limit: request.limit,
    })
  } catch {}
  /** @type {Array<Array<Record<string, any>>>} */
  const slices = []
  for (let start = 0; start < found.length; start += OFFLINE_SLICE) slices.push(found.slice(start, start + OFFLINE_SLICE).map(summary))
  return slices.length ? slices : [[]]
}

const SUMMARY_FIELDS = ['id', 'domain', 'folder', 'systemFolder', 'sender', 'subject', 'snippet', 'read', 'starred', 'labels',
  'receivedAt', 'hasAttachments', 'spamScore', 'spamReasons', 'snoozedUntil', 'providerAccountId', 'threadId', 'to', 'cc']
const NULLABLE = ['spamScore', 'spamReasons', 'labels', 'to', 'cc']

/**
 * The fields of an Email model (client/models/Email.go) and nothing else: a
 * Go listener refuses an event with a key its model does not have, or a null
 * where its field is not a pointer.
 * @param {Record<string, any>} email
 */
function summary(email) {
  return Object.fromEntries(SUMMARY_FIELDS
    .filter(field => email[field] !== undefined && (email[field] !== null || NULLABLE.includes(field)))
    .map(field => [field, email[field]]))
}
