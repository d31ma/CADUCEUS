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

/**
 * Write one little-endian ZIP record from its [offset, bits, value] fields.
 * @param {number} size
 * @param {Array<[number, 16 | 32 | 64, number | bigint]>} fields
 */
function record(size, fields) {
  const bytes = new Uint8Array(size)
  const view = new DataView(bytes.buffer)
  for (const [offset, bits, value] of fields) {
    if (bits === 64) view.setBigUint64(offset, BigInt(value), true)
    else if (bits === 32) view.setUint32(offset, Number(value), true)
    else view.setUint16(offset, Number(value), true)
  }
  return bytes
}

/** @param {{ name: Uint8Array, crc: number, size: bigint, time: number, date: number }} entry */
function localHeader(entry) {
  return record(30, [
    [0, 32, 0x04034b50], [4, 16, ZIP64_VERSION], [6, 16, UTF8_FLAG], [8, 16, 0], [10, 16, entry.time], [12, 16, entry.date], [14, 32, entry.crc], [18, 32, UINT32_MAX], [22, 32, UINT32_MAX], [26, 16, entry.name.byteLength], [28, 16, 20],
  ])
}

/** @param {{ size: bigint }} entry */
function localZip64Extra(entry) {
  return record(20, [
    [0, 16, 0x0001], [2, 16, 16], [4, 64, entry.size], [12, 64, entry.size],
  ])
}

/** @param {{ name: Uint8Array, crc: number, size: bigint, offset: bigint, time: number, date: number }} entry */
function centralHeader(entry) {
  return record(46, [
    [0, 32, 0x02014b50], [4, 16, (3 << 8) | ZIP64_VERSION], [6, 16, ZIP64_VERSION], [8, 16, UTF8_FLAG], [10, 16, 0], [12, 16, entry.time], [14, 16, entry.date], [16, 32, entry.crc], [20, 32, UINT32_MAX], [24, 32, UINT32_MAX], [28, 16, entry.name.byteLength], [30, 16, 28], [32, 16, 0], [34, 16, 0], [36, 16, 0], [38, 32, 0], [42, 32, UINT32_MAX],
  ])
}

/** @param {{ size: bigint, offset: bigint }} entry */
function centralZip64Extra(entry) {
  return record(28, [
    [0, 16, 0x0001], [2, 16, 24], [4, 64, entry.size], [12, 64, entry.size], [20, 64, entry.offset],
  ])
}

/** @param {number} count @param {bigint} centralSize @param {bigint} centralOffset */
function zip64End(count, centralSize, centralOffset) {
  return record(56, [
    [0, 32, 0x06064b50], [4, 64, 44n], [12, 16, (3 << 8) | ZIP64_VERSION], [14, 16, ZIP64_VERSION], [16, 32, 0], [20, 32, 0], [24, 64, BigInt(count)], [32, 64, BigInt(count)], [40, 64, centralSize], [48, 64, centralOffset],
  ])
}

/** @param {bigint} zip64Offset */
function zip64Locator(zip64Offset) {
  return record(20, [
    [0, 32, 0x07064b50], [4, 32, 0], [8, 64, zip64Offset], [16, 32, 1],
  ])
}

function classicEnd() {
  return record(22, [
    [0, 32, 0x06054b50], [4, 16, 0], [6, 16, 0], [8, 16, UINT16_MAX], [10, 16, UINT16_MAX], [12, 32, UINT32_MAX], [16, 32, UINT32_MAX], [20, 16, 0],
  ])
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
