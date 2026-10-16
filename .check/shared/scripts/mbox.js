// @ts-check

const MAGIC = new TextEncoder().encode('CADUCEUS-RFC822-FRAMES/1\n')
const FRAME_HEADER_BYTES = 8
const MAX_METADATA_BYTES = 16 * 1024
const MAX_SOURCE_BYTES = 512 * 1024 * 1024
const encoder = new TextEncoder()
const decoder = new TextDecoder('utf-8', { fatal: true })
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/**
 * Consume CADUCEUS's bounded RFC 822 frame stream and append an mboxrd record
 * for every message. The caller decides whether `write` targets an atomic
 * File System Access handle or an in-memory download fallback.
 *
 * @param {ReadableStream<Uint8Array>} body
 * @param {(chunk: Uint8Array) => Promise<void> | void} write
 * @param {{ signal?: AbortSignal, onProgress?: (progress: { messages: number, sourceBytes: number, outputBytes: number }) => void }} [options]
 */
export async function writeMboxFromFrameStream(body, write, options = {}) {
  return consumeRfc822FrameStream(body, async (metadata, messageBytes) => {
    const record = formatMboxRecord(metadata, messageBytes)
    await write(record)
    return record.byteLength
  }, options)
}

/**
 * Parse the versioned exact-source stream once and hand each complete frame to
 * a format-specific consumer. The callback is never invoked for a partial
 * frame, and the source reader is canceled on completion, error, or abort.
 *
 * @param {ReadableStream<Uint8Array>} body
 * @param {(metadata: Record<string, unknown>, source: Uint8Array, index: number) => Promise<number | void> | number | void} consume
 * @param {{ signal?: AbortSignal, onProgress?: (progress: { messages: number, sourceBytes: number, outputBytes: number }) => void }} [options]
 */
export async function consumeRfc822FrameStream(body, consume, options = {}) {
  const reader = body.getReader()
  const source = new ExactReader(reader, options.signal)
  let messages = 0
  let sourceBytes = 0
  let outputBytes = 0
  try {
    const magic = await source.take(MAGIC.byteLength, false)
    if (!magic || !equalBytes(magic, MAGIC)) throw new Error('Mailbox export stream has an invalid header')

    for (;;) {
      const header = await source.take(FRAME_HEADER_BYTES, true)
      if (!header) break
      const view = new DataView(header.buffer, header.byteOffset, header.byteLength)
      const metadataLength = view.getUint32(0)
      const sourceLength = view.getUint32(4)
      if (metadataLength === 0 || metadataLength > MAX_METADATA_BYTES) {
        throw new Error('Mailbox export metadata exceeds its limit')
      }
      if (sourceLength > MAX_SOURCE_BYTES) throw new Error('A message exceeds the mailbox export limit')

      const metadataBytes = await source.take(metadataLength, false)
      const messageBytes = await source.take(sourceLength, false)
      if (!metadataBytes || !messageBytes) throw new Error('Mailbox export stream ended inside a message')
      let metadata
      try {
        metadata = JSON.parse(decoder.decode(metadataBytes))
      } catch {
        throw new Error('Mailbox export metadata is malformed')
      }
      const written = await consume(metadata, messageBytes, messages + 1)
      messages += 1
      sourceBytes += messageBytes.byteLength
      outputBytes += Number(written) || 0
      options.onProgress?.({ messages, sourceBytes, outputBytes })
    }
    return { messages, sourceBytes, outputBytes }
  } finally {
    try { await reader.cancel() } catch {}
  }
}

/**
 * Convert one exact RFC 822 byte sequence to one mboxrd record.
 *
 * The byte body is never decoded: CRLF/CR are normalized to LF and a leading
 * `>` is inserted for every line matching `/^>*From /`. This preserves
 * arbitrary 8-bit content and transfer encodings while remaining compatible
 * with Thunderbird, mutt, Apple Mail, and Google Takeout readers.
 *
 * @param {{ sender?: unknown, receivedAt?: unknown }} metadata
 * @param {Uint8Array} source
 */
export function formatMboxRecord(metadata, source) {
  const separator = encoder.encode(
    `From ${envelopeSender(source, metadata?.sender)} ${asctime(messageDate(source, metadata?.receivedAt))}\n`,
  )
  const measurement = measureBody(source)
  const trailing = measurement.last === 0x0a
    ? (measurement.beforeLast === 0x0a ? 0 : 1)
    : 2
  const output = new Uint8Array(
    separator.byteLength + measurement.normalizedBytes + measurement.escapes + trailing,
  )
  output.set(separator, 0)
  let offset = separator.byteLength
  let lineStart = true
  for (let index = 0; index < source.byteLength; index += 1) {
    if (lineStart && isFromLine(source, index)) output[offset++] = 0x3e
    let byte = source[index]
    if (byte === 0x0d) {
      if (source[index + 1] === 0x0a) index += 1
      byte = 0x0a
    }
    output[offset++] = byte
    lineStart = byte === 0x0a
  }
  while (offset < output.byteLength) output[offset++] = 0x0a
  return output
}

/** @param {string} folder @param {string} domain */
export function mboxFilename(folder, domain) {
  const clean = value => String(value || '')
    .replace(/[/\\?%*:|"<>\u0000-\u001f\u007f]/g, '-')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 80)
  const mailbox = clean(domain) || 'mailbox'
  const name = clean(folder) || 'folder'
  return `${mailbox}-${name}.mbox`
}

/** @param {number} bytes */
export function formatExportBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${Math.round(bytes)} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

class ExactReader {
  /** @param {ReadableStreamDefaultReader<Uint8Array>} reader @param {AbortSignal | undefined} signal */
  constructor(reader, signal) {
    this.reader = reader
    this.signal = signal
    /** @type {Uint8Array} */
    this.pending = new Uint8Array(0)
    this.done = false
  }

  /** @param {number} length @param {boolean} allowCleanEof */
  async take(length, allowCleanEof) {
    this.abortIfNeeded()
    if (length === 0) return new Uint8Array(0)
    /** @type {Uint8Array[]} */
    const parts = []
    let available = 0
    if (this.pending.byteLength) {
      parts.push(this.pending)
      available += this.pending.byteLength
      this.pending = new Uint8Array(0)
    }
    while (available < length && !this.done) {
      this.abortIfNeeded()
      const result = await this.read()
      this.abortIfNeeded()
      if (result.done) { this.done = true; break }
      if (result.value?.byteLength) {
        parts.push(result.value)
        available += result.value.byteLength
      }
    }
    if (available === 0 && this.done && allowCleanEof) return null
    if (available < length) throw new Error('Mailbox export stream ended unexpectedly')

    if (parts.length === 1 && parts[0].byteLength === length) return parts[0]
    const output = new Uint8Array(length)
    let offset = 0
    for (const part of parts) {
      const needed = length - offset
      const used = Math.min(needed, part.byteLength)
      output.set(part.subarray(0, used), offset)
      offset += used
      if (used < part.byteLength) {
        this.pending = part.subarray(used)
        const partIndex = parts.indexOf(part)
        if (partIndex + 1 < parts.length) {
          this.pending = concatenate([this.pending, ...parts.slice(partIndex + 1)])
        }
        break
      }
    }
    return output
  }

  abortIfNeeded() {
    if (!this.signal?.aborted) return
    throw this.signal.reason instanceof Error
      ? this.signal.reason
      : new DOMException('Mailbox export canceled', 'AbortError')
  }

  async read() {
    if (!this.signal) return this.reader.read()
    this.abortIfNeeded()
    const signal = this.signal
    return new Promise((resolve, reject) => {
      const aborted = () => {
        cleanup()
        void this.reader.cancel().catch(() => {})
        reject(signal.reason instanceof Error
          ? signal.reason
          : new DOMException('Mailbox export canceled', 'AbortError'))
      }
      const cleanup = () => signal.removeEventListener('abort', aborted)
      signal.addEventListener('abort', aborted, { once: true })
      this.reader.read().then(
        result => { cleanup(); resolve(result) },
        error => { cleanup(); reject(error) },
      )
    })
  }
}

/** @param {Uint8Array[]} chunks */
function concatenate(chunks) {
  const output = new Uint8Array(chunks.reduce((total, chunk) => total + chunk.byteLength, 0))
  let offset = 0
  for (const chunk of chunks) { output.set(chunk, offset); offset += chunk.byteLength }
  return output
}

/** @param {Uint8Array} left @param {Uint8Array} right */
function equalBytes(left, right) {
  if (left.byteLength !== right.byteLength) return false
  for (let index = 0; index < left.byteLength; index += 1) {
    if (left[index] !== right[index]) return false
  }
  return true
}

/** @param {Uint8Array} bytes */
function measureBody(bytes) {
  let normalizedBytes = 0
  let escapes = 0
  let lineStart = true
  let beforeLast = -1
  let last = -1
  for (let index = 0; index < bytes.byteLength; index += 1) {
    if (lineStart && isFromLine(bytes, index)) escapes += 1
    let byte = bytes[index]
    if (byte === 0x0d) {
      if (bytes[index + 1] === 0x0a) index += 1
      byte = 0x0a
    }
    normalizedBytes += 1
    beforeLast = last
    last = byte
    lineStart = byte === 0x0a
  }
  return { normalizedBytes, escapes, beforeLast, last }
}

/** @param {Uint8Array} bytes @param {number} start */
function isFromLine(bytes, start) {
  let index = start
  while (bytes[index] === 0x3e) index += 1
  return bytes[index] === 0x46
    && bytes[index + 1] === 0x72
    && bytes[index + 2] === 0x6f
    && bytes[index + 3] === 0x6d
    && bytes[index + 4] === 0x20
}

/** @param {Uint8Array} source @param {unknown} fallback */
function envelopeSender(source, fallback) {
  const returnPath = headerValue(source, 'Return-Path')
  const from = headerValue(source, 'From')
  return extractAddress(returnPath) || extractAddress(from) || extractAddress(String(fallback || '')) || 'MAILER-DAEMON'
}

/** @param {Uint8Array} source @param {unknown} fallback */
function messageDate(source, fallback) {
  const header = headerValue(source, 'Date')
  const preferred = header ? new Date(header) : new Date(String(fallback || ''))
  return Number.isNaN(preferred.getTime()) ? new Date(0) : preferred
}

/** @param {Uint8Array} source @param {string} name */
function headerValue(source, name) {
  const limit = Math.min(source.byteLength, 256 * 1024)
  let end = limit
  for (let index = 0; index + 1 < limit; index += 1) {
    if (source[index] === 0x0a && source[index + 1] === 0x0a) { end = index; break }
    if (
      index + 3 < limit
      && source[index] === 0x0d && source[index + 1] === 0x0a
      && source[index + 2] === 0x0d && source[index + 3] === 0x0a
    ) { end = index; break }
  }
  let headers = ''
  for (let index = 0; index < end; index += 1) headers += String.fromCharCode(source[index])
  headers = headers.replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/\n[ \t]+/g, ' ')
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`^${escaped}:[ \\t]*(.*)$`, 'im').exec(headers)?.[1]?.trim() || ''
}

/** @param {string} value */
function extractAddress(value) {
  const clean = String(value || '').replace(/[\u0000-\u001f\u007f]/g, '').trim()
  if (!clean || clean === '<>') return ''
  const bracketed = /<([^<>]+)>/.exec(clean)?.[1]
  const candidate = bracketed || /[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9.-]+/i.exec(clean)?.[0] || ''
  return candidate.replace(/\s/g, '').slice(0, 320)
}

/** @param {Date} date */
function asctime(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${WEEKDAYS[date.getUTCDay()]} ${MONTHS[date.getUTCMonth()]} ${String(date.getUTCDate()).padStart(2, ' ')} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())} ${date.getUTCFullYear()}`
}
