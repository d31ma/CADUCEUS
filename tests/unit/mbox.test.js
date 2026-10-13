import { describe, expect, it } from 'bun:test'
import * as MboxUtils from '../../client/shared/scripts/mbox.js'

const formatMboxRecord = (...args) => MboxUtils.formatMboxRecord(...args)
const mboxFilename = (...args) => MboxUtils.mboxFilename(...args)
const writeMboxFromFrameStream = (...args) => MboxUtils.writeMboxFromFrameStream(...args)

const encoder = new TextEncoder()
const decoder = new TextDecoder('latin1')
const MAGIC = encoder.encode('CADUCEUS-RFC822-FRAMES/1\n')

describe('mboxrd formatting', () => {
  it('uses Return-Path and Date, normalizes newlines, and quotes every From-line depth', () => {
    const source = encoder.encode([
      'Return-Path: <bounce@example.test>',
      'From: Display <author@example.test>',
      'Date: Sun, 07 Jan 2024 06:05:04 +0000',
      'Subject: Portable',
      '',
      'ordinary',
      'From escaped once',
      '>From escaped twice',
      '',
    ].join('\r\n'))

    const output = decoder.decode(formatMboxRecord({ sender: 'fallback@invalid.test' }, source))
    expect(output).toStartWith('From bounce@example.test Sun Jan  7 06:05:04 2024\n')
    expect(output).toContain('\n>From escaped once\n>>From escaped twice\n\n')
    expect(output).not.toContain('\r')
  })

  it('preserves arbitrary 8-bit body bytes without decoding them', () => {
    const prefix = encoder.encode('From: sender@example.test\n\nBody: ')
    const source = new Uint8Array(prefix.byteLength + 2)
    source.set(prefix)
    source.set([0x80, 0xff], prefix.byteLength)
    const output = formatMboxRecord({ receivedAt: '2026-08-14T12:00:00Z' }, source)
    expect(output.includes(0x80)).toBe(true)
    expect(output.includes(0xff)).toBe(true)
    expect(output.slice(-2)).toEqual(new Uint8Array([0x0a, 0x0a]))
  })

  it('sanitizes portable filenames', () => {
    expect(mboxFilename('Projects/Q3', 'example.com')).toBe('example.com-Projects-Q3.mbox')
    expect(mboxFilename('', '')).toBe('mailbox-folder.mbox')
  })
})

describe('RFC 822 frame stream', () => {
  it('parses arbitrarily fragmented frames and reports durable write progress', async () => {
    const first = encoder.encode('From: first@example.test\r\n\r\nOne')
    const second = encoder.encode('From: second@example.test\r\n\r\nTwo')
    const streamBytes = concat([
      MAGIC,
      frame({ id: 'one', receivedAt: '2026-08-14T12:00:00Z' }, first),
      frame({ id: 'two', receivedAt: '2026-08-14T12:01:00Z' }, second),
    ])
    const body = chunks(streamBytes, [1, 3, 2, 17, 5, 31, 7])
    const written = []
    const progress = []
    const result = await writeMboxFromFrameStream(body, chunk => written.push(chunk), {
      onProgress: value => progress.push({ ...value }),
    })

    expect(result.messages).toBe(2)
    expect(result.sourceBytes).toBe(first.byteLength + second.byteLength)
    expect(written).toHaveLength(2)
    expect(progress.map(value => value.messages)).toEqual([1, 2])
    expect(decoder.decode(concat(written)).match(/^From /gm)).toHaveLength(2)
  })

  it('rejects an invalid or truncated frame instead of installing partial output', async () => {
    const invalid = chunks(encoder.encode('not-a-caduceus-stream'), [2, 1])
    await expect(writeMboxFromFrameStream(invalid, () => {})).rejects.toThrow(/invalid header|unexpectedly/)

    const header = new Uint8Array(8)
    new DataView(header.buffer).setUint32(0, 2)
    new DataView(header.buffer).setUint32(4, 100)
    const truncated = chunks(concat([MAGIC, header, encoder.encode('{}tiny')]), [4, 9])
    await expect(writeMboxFromFrameStream(truncated, () => {})).rejects.toThrow(/ended unexpectedly/)
  })

  it('cancels a stalled stream without writing a partial mailbox', async () => {
    const controller = new AbortController()
    let streamController
    const stalled = new ReadableStream({
      start(value) {
        streamController = value
        value.enqueue(MAGIC)
      },
      cancel() {},
    })
    const written = []
    const pending = writeMboxFromFrameStream(stalled, chunk => written.push(chunk), {
      signal: controller.signal,
    })
    await Promise.resolve()
    controller.abort(new DOMException('Canceled by test', 'AbortError'))
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    expect(written).toEqual([])
    void streamController
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
