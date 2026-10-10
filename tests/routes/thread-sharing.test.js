import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { Handler as Shares } from '@/routes/thread-shares/yon.js'
import { Handler as Share } from '@/routes/thread-shares/_id/yon.js'
import { Handler as PublicShare } from '@/routes/thread-shares/public/_token/yon.js'
import { Handler as PublicAttachment } from '@/routes/thread-shares/public/_token/attachments/_attachment_id/yon.js'
import { FyloAccessContext } from '@/platform/persistence/fylo-access.js'

let app
let attachment
let unrelatedAttachment
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(async () => {
  app = await setupRoutes()
  await app.emails.put(email({
    id: 'share-root',
    ownerEmail: 'alice@example.com',
    rfcMessageId: '<share-root@other.test>',
    threadId: 'a'.repeat(64),
    subject: 'Launch plan',
    body: 'first private message',
    receivedAt: '2026-08-14T10:00:00.000Z',
    htmlBody: '<p>first <strong>private</strong> message</p><script>alert(1)</script><img src="https://tracker.invalid/pixel">',
    to: ['alice@example.com'],
    cc: ['team@other.test'],
    bcc: ['hidden@other.test'],
    trackingId: 'must-not-leak',
  }))
  await app.emails.put(email({
    id: 'share-reply',
    ownerEmail: 'alice@example.com',
    sender: 'alice@example.com',
    recipient: 'recipient@other.test',
    folder: 'sent',
    rfcMessageId: '<share-reply@example.com>',
    inReplyTo: '<share-root@other.test>',
    references: ['<share-root@other.test>'],
    threadId: 'a'.repeat(64),
    subject: 'Re: Launch plan',
    body: 'second private message',
    receivedAt: '2026-08-14T10:05:00.000Z',
  }))
  await app.emails.put(email({
    id: 'share-bob-copy',
    ownerEmail: 'bob@example.com',
    recipient: 'bob@example.com',
    rfcMessageId: '<share-bob-copy@example.com>',
    inReplyTo: '<share-root@other.test>',
    threadId: 'a'.repeat(64),
    subject: 'Re: Launch plan',
    body: 'Bob mailbox secret',
  }))
  await app.emails.put(email({
    id: 'share-unrelated',
    ownerEmail: 'alice@example.com',
    rfcMessageId: '<unrelated@other.test>',
    threadId: 'b'.repeat(64),
    subject: 'Unrelated',
  }))
  ;[attachment] = await app.attachments.saveForEmail('share-root', 'example.com', [{
    filename: 'launch.txt', contentType: 'text/plain', content: new TextEncoder().encode('attachment secret'),
  }])
  ;[unrelatedAttachment] = await app.attachments.saveForEmail('share-unrelated', 'example.com', [{
    filename: 'unrelated.txt', contentType: 'text/plain', content: new TextEncoder().encode('not in share'),
  }])
}, 300_000)
afterAll(teardownRoutes)

describe('self-hosted expiring thread shares', () => {
  let first
  let second

  it('requires auth, validates expiry, and enforces exact mailbox ownership', async () => {
    expect((await Shares.POST(req({ body: { emailId: 'share-root', expiresInDays: 7 } }))).unauthorized).toBe(true)
    expect((await Shares.POST(req({ token: alice(), body: { emailId: 'share-root', expiresInDays: 2 } }))).error).toContain('1, 7, 30')
    expect((await Shares.POST(req({ token: bob(), body: { emailId: 'share-root', expiresInDays: 7 } }))).notFound).toBe(true)
    expect((await Shares.GET(req({ token: bob(), query: { emailId: 'share-root' } }))).notFound).toBe(true)
  })

  it('creates a hash-only capability and exposes a sanitized immutable conversation', async () => {
    first = await Shares.POST(req({ token: alice(), body: { emailId: 'share-root', expiresInDays: 7 } }))
    expect(first.id).toMatch(/^share-/)
    expect(first.token).toMatch(/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]{43}$/)
    expect(first.url).toBe(`/share/${first.token}`)
    expect(first.messageCount).toBe(2)

    const managed = await Shares.GET(req({ token: alice(), query: { emailId: 'share-root' } }))
    expect(managed).toHaveLength(1)
    expect(managed[0]).not.toHaveProperty('token')
    expect(managed[0]).not.toHaveProperty('tokenHash')

    const stored = await FyloAccessContext.runForDomain('example.com', () => app.threadShareRecords.findById(first.id))
    expect(stored[1].tokenHash).toMatch(/^[a-f0-9]{64}$/)
    expect(JSON.stringify(stored[1])).not.toContain(first.token)

    const publicView = await PublicShare.GET(req({ paths: { token: first.token } }))
    expect(publicView.subject).toBe('Launch plan')
    expect(publicView.messages).toHaveLength(2)
    expect(publicView.messages.map(message => message.body)).toEqual([
      'first private message', 'second private message',
    ])
    expect(JSON.stringify(publicView)).not.toContain('Bob mailbox secret')
    expect(JSON.stringify(publicView)).not.toContain('hidden@other.test')
    expect(JSON.stringify(publicView)).not.toContain('must-not-leak')
    expect(publicView.messages[0]).not.toHaveProperty('ownerEmail')
    expect(publicView.messages[0]).not.toHaveProperty('domain')
  })

  it('re-authorizes attachment membership on every public read', async () => {
    const downloaded = await PublicAttachment.GET(req({ paths: {
      token: first.token, attachment_id: attachment.id,
    } }))
    expect(Buffer.from(downloaded.contentBase64, 'base64').toString()).toBe('attachment secret')

    const unrelated = await PublicAttachment.GET(req({ paths: {
      token: first.token, attachment_id: unrelatedAttachment.id,
    } }))
    expect(unrelated.notFound).toBe(true)

    const altered = `${first.token.slice(0, -1)}${first.token.endsWith('A') ? 'B' : 'A'}`
    expect((await PublicShare.GET(req({ paths: { token: altered } }))).notFound).toBe(true)
  })

  it('rotates links, hides foreign revocation, and revokes immediately', async () => {
    second = await Shares.POST(req({ token: alice(), body: { emailId: 'share-root', expiresInDays: 1 } }))
    expect(second.token).not.toBe(first.token)
    expect((await PublicShare.GET(req({ paths: { token: first.token } }))).notFound).toBe(true)
    expect((await PublicShare.GET(req({ paths: { token: second.token } }))).messages).toHaveLength(2)

    expect((await Share.DELETE(req({ token: bob(), paths: { id: second.id } }))).notFound).toBe(true)
    expect((await Share.DELETE(req({ token: alice(), paths: { id: second.id } }))).revoked).toBe(second.id)
    expect((await PublicShare.GET(req({ paths: { token: second.token } }))).notFound).toBe(true)
    expect((await PublicAttachment.GET(req({ paths: {
      token: second.token, attachment_id: attachment.id,
    } }))).notFound).toBe(true)
  })

  it('fails closed after expiry without revealing why', async () => {
    const expiring = await Shares.POST(req({ token: alice(), body: { emailId: 'share-root', expiresInDays: 1 } }))
    const [docId] = await FyloAccessContext.runForDomain('example.com', () => app.threadShareRecords.findById(expiring.id))
    await FyloAccessContext.runForDomain('example.com', () => app.threadShareRecords.update(docId, {
      expiresAt: new Date(Date.now() - 1000).toISOString(),
    }))
    const result = await PublicShare.GET(req({ paths: { token: expiring.token } }))
    expect(result).toEqual({ error: 'Shared conversation not found', notFound: true })
  })
})
