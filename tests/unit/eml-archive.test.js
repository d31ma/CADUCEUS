import { describe, expect, it } from 'bun:test'
import { EmlArchiveUtils } from '../../client/shared/utils/EmlArchiveUtils.js'

const emlArchiveEntryName = (...args) => EmlArchiveUtils.emlArchiveEntryName(...args)
const emlArchiveFilename = (...args) => EmlArchiveUtils.emlArchiveFilename(...args)
const writeEmlArchiveFromFrameStream = (...args) => EmlArchiveUtils.writeEmlArchiveFromFrameStream(...args)

const encoder = new TextEncoder()
const decoder = new TextDecoder()
const MAGIC = encoder.encode('CADUCEUS-RFC822-FRAMES/1\n')

describe('EML folder archive', () => {
  it('streams a valid store-only ZIP64 archive with exact source bytes', async () => {
    const first = concat([
      encoder.encode('From: first@example.test\r\nSubject: First / message\r\n\r\nBinary: '),
      new Uint8Array([0x00, 0x80, 0xff]),
    ])
    const second = encoder.encode('From: second@example.test\nSubject: Second message\n\nFrom body')
    const input = concat([
      MAGIC,
      frame({ subject: 'First / message', receivedAt: '2026-08-14T12:00:00Z' }, first),
      frame({ subject: 'Second message', receivedAt: '2026-08-14T12:01:00Z' }, second),
    ])
    const written = []
    const progress = []

    const result = await writeEmlArchiveFromFrameStream(chunks(input, [1, 2, 19, 3, 41]), chunk => {
      written.push(Uint8Array.from(chunk))
    }, { onProgress: value => progress.push({ ...value }) })

    const archive = concat(written)
    const entries = localEntries(archive)
    expect([...entries.keys()]).toEqual([
      'messages/000001-2026-08-14-First - message.eml',
      'messages/000002-2026-08-14-Second message.eml',
    ])
    expect(entries.get([...entries.keys()][0])).toEqual(first)
    expect(entries.get([...entries.keys()][1])).toEqual(second)
    expect(result).toEqual({ messages: 2, sourceBytes: first.byteLength + second.byteLength, outputBytes: archive.byteLength })
    expect(progress.map(value => value.messages)).toEqual([1, 2])
    expect(new DataView(archive.buffer, archive.byteOffset).getUint32(archive.byteLength - 22, true)).toBe(0x06054b50)
  })

  it('produces collision-safe portable names and a valid empty archive', async () => {
    expect(emlArchiveEntryName({ subject: '../Q3:*', receivedAt: 'invalid' }, 4))
      .toBe('messages/000004-undated-Q3.eml')
    expect(emlArchiveFilename('Projects/Q3', 'example.com')).toBe('example.com-Projects-Q3-eml.zip')
    expect(emlArchiveFilename('', '')).toBe('mailbox-folder-eml.zip')

    const written = []
    const result = await writeEmlArchiveFromFrameStream(chunks(MAGIC, [2]), chunk => written.push(Uint8Array.from(chunk)))
    const archive = concat(written)
    expect(result.messages).toBe(0)
    expect(localEntries(archive).size).toBe(0)
    expect(archive.byteLength).toBe(98)
  })

  it('aborts a stalled frame stream without completing an archive', async () => {
    const controller = new AbortController()
    const written = []
    const stalled = new ReadableStream({
      start(stream) { stream.enqueue(MAGIC) },
      cancel() {},
    })
    const pending = writeEmlArchiveFromFrameStream(stalled, chunk => written.push(chunk), {
      signal: controller.signal,
    })
    await Promise.resolve()
    controller.abort(new DOMException('Canceled by test', 'AbortError'))
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    expect(written).toEqual([])
  })
})

function frame(metadata, source) {
  const meta = encoder.encode(JSON.stringify(metadata))
  const header = new Uint8Array(8)
  const view = new DataView(header.buffer)
  view.setUint32(0, meta.byteLength)
  view.setUint32(4, source.byteLength)
  return concat([header, meta, source])
}

function localEntries(bytes) {
  const entries = new Map()
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  let offset = 0
  while (offset + 4 <= bytes.byteLength && view.getUint32(offset, true) === 0x04034b50) {
    const nameLength = view.getUint16(offset + 26, true)
    const extraLength = view.getUint16(offset + 28, true)
    const nameStart = offset + 30
    const extraStart = nameStart + nameLength
    expect(view.getUint16(extraStart, true)).toBe(0x0001)
    const size = Number(view.getBigUint64(extraStart + 4, true))
    const dataStart = extraStart + extraLength
    const name = decoder.decode(bytes.subarray(nameStart, extraStart))
    entries.set(name, bytes.slice(dataStart, dataStart + size))
    offset = dataStart + size
  }
  return entries
}

function concat(parts) {
  const output = new Uint8Array(parts.reduce((sum, part) => sum + part.byteLength, 0))
  let offset = 0
  for (const part of parts) { output.set(part, offset); offset += part.byteLength }
  return output
}

function chunks(bytes, sizes) {
  return new ReadableStream({
    start(controller) {
      let offset = 0
      let index = 0
      while (offset < bytes.byteLength) {
        const size = sizes[index++ % sizes.length]
        controller.enqueue(bytes.slice(offset, offset + size))
        offset += size
      }
      controller.close()
    },
  })
}
