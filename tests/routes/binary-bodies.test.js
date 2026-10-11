import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { createHash } from 'node:crypto'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { call, respond } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

// Every byte value, so a body that passes through UTF-8 text anywhere on the
// way (the server, the frame encoder, the HTTP layer) cannot stay equal.
const EVERY_BYTE = Buffer.from(Array.from({ length: 256 }, (_, index) => index))
const PNG_HEADER = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0xff, 0xfe, 0xc3, 0x28])

const Detail = Routes.InboxIdController
const Attachment = Routes.InboxIdAttachmentsAttachmentIdController
const Source = Routes.InboxIdSourceController
const ExportSources = Routes.InboxExportRfc822Controller

let app
const auth = () => token()

/** A Latin-1, 8bit message: its body bytes are not valid UTF-8. */
function latinSource(subject) {
  return Buffer.concat([
    Buffer.from(`From: Bob <bob@other.com>\r\nSubject: ${subject}\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: 8bit\r\n\r\n`, 'ascii'),
    Buffer.from([0x43, 0x61, 0x66, 0xe9, 0x20, 0xa9, 0x20, 0xff, 0x80, 0x0d, 0x0a]),
    EVERY_BYTE,
    Buffer.from('\r\n', 'ascii'),
  ])
}

/** Splits an rfc822-frames export into its metadata and exact sources. */
function frames(bytes) {
  const magic = Buffer.from('CADUCEUS-RFC822-FRAMES/1\n', 'ascii')
  expect(bytes.subarray(0, magic.length).equals(magic)).toBe(true)
  const parsed = []
  let offset = magic.length
  while (offset < bytes.length) {
    const metadataLength = bytes.readUInt32BE(offset)
    const sourceLength = bytes.readUInt32BE(offset + 4)
    offset += 8
    const metadata = JSON.parse(bytes.subarray(offset, offset + metadataLength).toString('utf8'))
    offset += metadataLength
    parsed.push({ metadata, source: Buffer.from(bytes.subarray(offset, offset + sourceLength)) })
    offset += sourceLength
  }
  expect(offset).toBe(bytes.length)
  return parsed
}

beforeAll(async () => {
  app = await setupRoutes()
}, 300_000)
afterAll(teardownRoutes)

describe('binary bodies are byte-exact', () => {
  it('downloads an attachment holding every byte value', async () => {
    await app.emails.put(email({ id: 'binary-attachment' }))
    const [saved] = await app.attachments.saveForEmail('binary-attachment', 'example.com', [{
      filename: 'every-byte.bin', contentType: 'application/octet-stream', content: EVERY_BYTE,
    }])
    const result = await call(Attachment, 'GET', req({ token: auth(), paths: { id: 'binary-attachment', attachment_id: saved.id } }))
    expect(result.filename).toBe('every-byte.bin')
    expect(Buffer.from(result.contentBase64, 'base64').equals(EVERY_BYTE)).toBe(true)
  })

  it('serves an inline image by its Content-ID with its exact bytes', async () => {
    const image = Buffer.concat([PNG_HEADER, EVERY_BYTE])
    await app.emails.put(email({ id: 'binary-inline', htmlBody: '<p>Logo</p><img src="cid:logo@caduceus.test">' }))
    const [saved] = await app.attachments.saveForEmail('binary-inline', 'example.com', [{
      filename: 'logo.png', contentType: 'image/png', content: image, disposition: 'inline', contentId: 'logo@caduceus.test',
    }])
    const detail = await call(Detail, 'GET', req({ token: auth(), paths: { id: 'binary-inline' } }))
    const listed = [...(detail.attachments ?? []), ...(detail.inlineAttachments ?? [])].find(item => item.id === saved.id)
    expect(listed).toMatchObject({ filename: 'logo.png', contentType: 'image/png', size: image.length })
    const result = await call(Attachment, 'GET', req({ token: auth(), paths: { id: 'binary-inline', attachment_id: saved.id } }))
    expect(result.contentType).toBe('image/png')
    expect(Buffer.from(result.contentBase64, 'base64').equals(image)).toBe(true)
  })

  it('answers the raw source of an 8bit Latin-1 message byte for byte', async () => {
    const source = latinSource('Latin source')
    await app.emails.put(email({ id: 'binary-source', subject: 'Latin source' }))
    await app.rawMessages.saveForEmail('binary-source', 'example.com', source)
    const result = await call(Source, 'GET', req({ token: auth(), paths: { id: 'binary-source' } }))
    expect(result.contentType).toBe('message/rfc822')
    const bytes = Buffer.from(result.contentBase64, 'base64')
    expect(bytes.equals(source)).toBe(true)
    expect(result.sha256).toBe(createHash('sha256').update(source).digest('hex'))
  })

  it('exports non-UTF-8 sources as rfc822 frames byte for byte', async () => {
    const first = latinSource('Frame one')
    const second = Buffer.concat([Buffer.from('From: Two <two@sender.test>\r\nSubject: Frame two\r\n\r\n', 'ascii'), EVERY_BYTE.subarray(128), EVERY_BYTE.subarray(0, 128)])
    await app.emails.put(email({ id: 'binary-frame-one', folder: 'binary-export', receivedAt: '2026-08-15T12:00:00Z', subject: 'Frame one' }))
    await app.emails.put(email({ id: 'binary-frame-two', folder: 'binary-export', receivedAt: '2026-08-15T12:01:00Z', subject: 'Frame two' }))
    await app.rawMessages.saveForEmail('binary-frame-one', 'example.com', first)
    await app.rawMessages.saveForEmail('binary-frame-two', 'example.com', second)

    const response = await respond(ExportSources, 'GET', req({ token: auth(), query: { folder: 'binary-export', domain: 'example.com' } }))
    expect(response.status).toBe(200)
    expect(response.headers.get('content-type')).toContain('application/vnd.caduceus.rfc822-frames')
    const bytes = Buffer.from(await response.arrayBuffer())
    expect(Number(response.headers.get('content-length') ?? bytes.length)).toBe(bytes.length)
    const exported = frames(bytes)
    const byId = Object.fromEntries(exported.map(frame => [frame.metadata.id, frame.source]))
    expect(Object.keys(byId).sort()).toEqual(['binary-frame-one', 'binary-frame-two'])
    expect(byId['binary-frame-one'].equals(first)).toBe(true)
    expect(byId['binary-frame-two'].equals(second)).toBe(true)
  })
})
