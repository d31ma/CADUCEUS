// @ts-check

import { consumeRfc822FrameStream } from './mbox.js'

const encoder = new TextEncoder()
const UTF8_FLAG = 0x0800
const ZIP64_VERSION = 45
const UINT32_MAX = 0xffff_ffff
const UINT16_MAX = 0xffff
const MAX_ENTRY_NAME_BYTES = 240

/**
 * Stream exact RFC 822 frames into a standards-compatible, store-only ZIP64
 * archive. Each source is resident only for its own local entry write; the
 * retained central-directory metadata is bounded to one short filename and
 * fixed-size record per message.
 *
 * @param {ReadableStream<Uint8Array>} body
 * @param {(chunk: Uint8Array) => Promise<void> | void} write
 * @param {{ signal?: AbortSignal, onProgress?: (progress: { messages: number, sourceBytes: number, outputBytes: number }) => void }} [options]
 */
export async function writeEmlArchiveFromFrameStream(body, write, options = {}) {
  /** @type {Array<{ name: Uint8Array, crc: number, size: bigint, offset: bigint, time: number, date: number }>} */
  const entries = []
  let offset = 0n

  /** @param {Uint8Array} chunk */
  const append = async chunk => {
    options.signal?.throwIfAborted()
    await write(chunk)
    offset += BigInt(chunk.byteLength)
  }

  const result = await consumeRfc822FrameStream(body, async (metadata, source, index) => {
    const name = encoder.encode(emlArchiveEntryName(metadata, index))
    if (name.byteLength > MAX_ENTRY_NAME_BYTES) throw new Error('EML archive filename exceeds its limit')
    const timestamp = dosTimestamp(metadata.receivedAt)
    const entry = {
      name,
      crc: crc32(source),
      size: BigInt(source.byteLength),
      offset,
      ...timestamp,
    }
    const header = localHeader(entry)
    const extra = localZip64Extra(entry)
    await append(header)
    await append(name)
    await append(extra)
    await append(source)
    entries.push(entry)
    return header.byteLength + name.byteLength + extra.byteLength + source.byteLength
  }, options)

  const centralOffset = offset
  for (const entry of entries) {
    const header = centralHeader(entry)
    const extra = centralZip64Extra(entry)
    await append(header)
    await append(entry.name)
    await append(extra)
  }
  const centralSize = offset - centralOffset
  const zip64Offset = offset
  await append(zip64End(entries.length, centralSize, centralOffset))
  await append(zip64Locator(zip64Offset))
  await append(classicEnd())

  return { ...result, outputBytes: safeCount(offset) }
}

/** @param {Record<string, unknown>} metadata @param {number} index */
export function emlArchiveEntryName(metadata, index) {
  const sequence = String(Math.max(1, Math.trunc(index) || 1)).padStart(6, '0')
  const date = validDate(metadata.receivedAt)
  const stamp = date ? date.toISOString().slice(0, 10) : 'undated'
  let subject = cleanComponent(metadata.subject) || 'no-subject'
  while (encoder.encode(`messages/${sequence}-${stamp}-${subject}.eml`).byteLength > MAX_ENTRY_NAME_BYTES) {
    subject = subject.slice(0, -1)
  }
  return `messages/${sequence}-${stamp}-${subject || 'message'}.eml`
}

/** @param {string} folder @param {string} domain */
export function emlArchiveFilename(folder, domain) {
  const mailbox = cleanComponent(domain) || 'mailbox'
  const name = cleanComponent(folder) || 'folder'
  return `${mailbox}-${name}-eml.zip`
}

/** @param {{ name: Uint8Array, crc: number, size: bigint, time: number, date: number }} entry */
function localHeader(entry) {
  const bytes = new Uint8Array(30)
  const view = new DataView(bytes.buffer)
  view.setUint32(0, 0x04034b50, true)
  view.setUint16(4, ZIP64_VERSION, true)
  view.setUint16(6, UTF8_FLAG, true)
  view.setUint16(8, 0, true)
  view.setUint16(10, entry.time, true)
  view.setUint16(12, entry.date, true)
  view.setUint32(14, entry.crc, true)
  view.setUint32(18, UINT32_MAX, true)
  view.setUint32(22, UINT32_MAX, true)
  view.setUint16(26, entry.name.byteLength, true)
  view.setUint16(28, 20, true)
  return bytes
}

/** @param {{ size: bigint }} entry */
function localZip64Extra(entry) {
  const bytes = new Uint8Array(20)
  const view = new DataView(bytes.buffer)
  view.setUint16(0, 0x0001, true)
  view.setUint16(2, 16, true)
  view.setBigUint64(4, entry.size, true)
  view.setBigUint64(12, entry.size, true)
  return bytes
}

/** @param {{ name: Uint8Array, crc: number, size: bigint, offset: bigint, time: number, date: number }} entry */
function centralHeader(entry) {
  const bytes = new Uint8Array(46)
  const view = new DataView(bytes.buffer)
  view.setUint32(0, 0x02014b50, true)
  view.setUint16(4, (3 << 8) | ZIP64_VERSION, true)
  view.setUint16(6, ZIP64_VERSION, true)
  view.setUint16(8, UTF8_FLAG, true)
  view.setUint16(10, 0, true)
  view.setUint16(12, entry.time, true)
  view.setUint16(14, entry.date, true)
  view.setUint32(16, entry.crc, true)
  view.setUint32(20, UINT32_MAX, true)
  view.setUint32(24, UINT32_MAX, true)
  view.setUint16(28, entry.name.byteLength, true)
  view.setUint16(30, 28, true)
  view.setUint16(32, 0, true)
  view.setUint16(34, 0, true)
  view.setUint16(36, 0, true)
  view.setUint32(38, 0, true)
  view.setUint32(42, UINT32_MAX, true)
  return bytes
}

/** @param {{ size: bigint, offset: bigint }} entry */
function centralZip64Extra(entry) {
  const bytes = new Uint8Array(28)
  const view = new DataView(bytes.buffer)
  view.setUint16(0, 0x0001, true)
  view.setUint16(2, 24, true)
  view.setBigUint64(4, entry.size, true)
  view.setBigUint64(12, entry.size, true)
  view.setBigUint64(20, entry.offset, true)
  return bytes
}

/** @param {number} count @param {bigint} centralSize @param {bigint} centralOffset */
function zip64End(count, centralSize, centralOffset) {
  const bytes = new Uint8Array(56)
  const view = new DataView(bytes.buffer)
  view.setUint32(0, 0x06064b50, true)
  view.setBigUint64(4, 44n, true)
  view.setUint16(12, (3 << 8) | ZIP64_VERSION, true)
  view.setUint16(14, ZIP64_VERSION, true)
  view.setUint32(16, 0, true)
  view.setUint32(20, 0, true)
  view.setBigUint64(24, BigInt(count), true)
  view.setBigUint64(32, BigInt(count), true)
  view.setBigUint64(40, centralSize, true)
  view.setBigUint64(48, centralOffset, true)
  return bytes
}

/** @param {bigint} zip64Offset */
function zip64Locator(zip64Offset) {
  const bytes = new Uint8Array(20)
  const view = new DataView(bytes.buffer)
  view.setUint32(0, 0x07064b50, true)
  view.setUint32(4, 0, true)
  view.setBigUint64(8, zip64Offset, true)
  view.setUint32(16, 1, true)
  return bytes
}

function classicEnd() {
  const bytes = new Uint8Array(22)
  const view = new DataView(bytes.buffer)
  view.setUint32(0, 0x06054b50, true)
  view.setUint16(4, 0, true)
  view.setUint16(6, 0, true)
  view.setUint16(8, UINT16_MAX, true)
  view.setUint16(10, UINT16_MAX, true)
  view.setUint32(12, UINT32_MAX, true)
  view.setUint32(16, UINT32_MAX, true)
  view.setUint16(20, 0, true)
  return bytes
}

/** @param {Uint8Array} bytes */
function crc32(bytes) {
  let crc = UINT32_MAX
  for (const byte of bytes) crc = CRC_TABLE[(crc ^ byte) & 0xff] ^ (crc >>> 8)
  return (crc ^ UINT32_MAX) >>> 0
}

const CRC_TABLE = (() => {
  const table = new Uint32Array(256)
  for (let index = 0; index < table.length; index += 1) {
    let value = index
    for (let bit = 0; bit < 8; bit += 1) value = (value & 1) ? 0xedb88320 ^ (value >>> 1) : value >>> 1
    table[index] = value >>> 0
  }
  return table
})()

/** @param {unknown} value */
function cleanComponent(value) {
  return String(value || '')
    .normalize('NFKC')
    .replace(/[/\\?%*:|"<>\u0000-\u001f\u007f]/g, '-')
    .replace(/\s+/g, ' ')
    .replace(/^[.\s-]+|[.\s-]+$/g, '')
    .trim()
    .slice(0, 120)
}

/** @param {unknown} value */
function validDate(value) {
  const date = new Date(String(value || ''))
  return Number.isNaN(date.getTime()) ? null : date
}

/** @param {unknown} value */
function dosTimestamp(value) {
  const input = validDate(value) || new Date(0)
  const year = Math.min(2107, Math.max(1980, input.getUTCFullYear()))
  return {
    time: (input.getUTCHours() << 11) | (input.getUTCMinutes() << 5) | Math.floor(input.getUTCSeconds() / 2),
    date: ((year - 1980) << 9) | ((input.getUTCMonth() + 1) << 5) | input.getUTCDate(),
  }
}

/** @param {bigint} value */
function safeCount(value) {
  return value > BigInt(Number.MAX_SAFE_INTEGER) ? Number.MAX_SAFE_INTEGER : Number(value)
}
