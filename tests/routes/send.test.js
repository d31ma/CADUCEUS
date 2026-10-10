import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { SendController as Send } from '@/controllers/SendController.js'
import { SendScheduledController as Scheduled } from '@/controllers/SendScheduledController.js'
import { SendScheduledIdController as ScheduledItem } from '@/controllers/SendScheduledIdController.js'
import { SendUndoController as Undo } from '@/controllers/SendUndoController.js'
import { RemindersController as Reminders } from '@/controllers/RemindersController.js'
import { RemindersTickController as ReminderTick } from '@/controllers/RemindersTickController.js'
import { SendFlushController as Flush } from '@/controllers/SendFlushController.js'
import { simpleParser } from 'mailparser'
import { createHmac } from 'node:crypto'
import { call } from '../support/controller-call.mjs'

let app
const auth = () => token()
const base = { to: ['bob@other.com'], subject: 'Hello', text: 'hi' }
const future = () => new Date(Date.now() + 3_600_000).toISOString()

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: ['sales@example.com'], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('POST /send', () => {
  it('requires auth', async () => {
    expect((await call(Send, 'POST', req({ body: base }))).unauthorized).toBe(true)
  })

  it('400s on an invalid request', async () => {
    expect((await call(Send, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/Subject is required/)
    expect((await call(Send, 'POST', req({ token: auth(), body: { subject: 'x', to: [] } }))).error).toMatch(/Missing to field/)
  })

  it('400s on invalid attachments', async () => {
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, attachments: 'nope' } }))
    expect(res.error).toMatch(/attachments must be an array/)
  })

  it('422s when a recipient is suppressed', async () => {
    await app.suppressed.suppress('blocked@other.com', 'bounce')
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, to: ['blocked@other.com'] } }))
    expect(res.blocked).toEqual(['blocked@other.com'])
  })

  it('sends immediately', async () => {
    const res = await call(Send, 'POST', req({ token: auth(), body: base }))
    expect(res.sent ?? res.messageId ?? res.id).toBeTruthy()
  })

  it('replays one accepted send idempotently and rejects key reuse', async () => {
    const body = {
      ...base,
      subject: 'Exactly once acceptance',
      delayMs: 60_000,
      idempotencyKey: 'offline-send-11111111-1111-4111-8111-111111111111',
    }
    const first = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.61', body }))
    const replay = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.61', body: { ...body } }))
    expect(replay).toEqual(first)
    expect((await app.emails.list(['example.com'], { folder: 'sent', ownerEmail: 'alice@example.com' }))
      .filter(email => email.subject === body.subject)).toHaveLength(1)
    expect((await app.outbox.findById(first.undoId))[1]).toBeTruthy()
    expect((await app.sendRequestRecords.find('alice@example.com', body.idempotencyKey))[1]).toMatchObject({ state: 'accepted' })

    const secondBody = {
      ...body,
      subject: 'A separate exactly-once acceptance',
      idempotencyKey: 'offline-send-22222222-2222-4222-8222-222222222222',
    }
    const second = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.61', body: secondBody }))
    expect(second).toMatchObject({ sent: true, undoId: expect.any(String) })
    expect(second.undoId).not.toBe(first.undoId)
    expect((await app.sendRequestRecords.find('alice@example.com', secondBody.idempotencyKey))[1])
      .toMatchObject({ state: 'accepted' })

    const reused = await call(Send, 'POST', req({
      token: auth(), ip: '10.0.0.61', body: { ...body, subject: 'Different message' },
    }))
    expect(reused.conflict).toBe(true)
    expect(reused.error).toMatch(/different message/)
  })

  it('keeps tracking off by default and validates explicit controls', async () => {
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, openTracking: 'yes' } }))).error).toMatch(/boolean/)
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, linkTracking: true } }))).error).toMatch(/HTML/)
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, subject: 'Untracked default' }, ip: '10.0.0.31' }))
    const [, sent] = await app.emails.findById(res.messageId)
    expect(sent.trackingId).toBeUndefined()
    expect(sent.openTracking).toBeUndefined()
    expect(sent.linkTracking).toBeUndefined()
  })

  it('stores clean tracked metadata while leaving capabilities only in SMTP delivery bodies', async () => {
    const html = '<p>Review <a href="https://example.org/proposal">the proposal</a>.</p>'
    const res = await call(Send, 'POST', req({
      token: auth(),
      ip: '10.0.0.32',
      body: { to: ['bob@other.com'], subject: 'Tracked send', html, openTracking: true, linkTracking: true },
    }))
    const [, sent] = await app.emails.findById(res.messageId)
    expect(sent.ownerEmail).toBe('alice@example.com')
    expect(sent.trackingId).toMatch(/^[A-Za-z0-9_-]{16,128}$/)
    expect(sent.openTracking).toBe(true)
    expect(sent.linkTracking).toBe(true)
    expect(sent.trackedLinks).toEqual([{ index: 0, url: 'https://example.org/proposal', title: 'the proposal' }])
    expect(sent.opens).toEqual([])
    expect(sent.clicks).toEqual([])
    expect(sent.htmlBody).toBe(html)
    expect(sent.htmlBody).not.toContain('/track/')
    const source = await app.rawMessages.findForEmail(res.messageId)
    expect(Buffer.from(await app.rawMessages.readContent(source)).toString('utf8')).not.toContain('/track/')
  })

  it('retains a standards-compliant EML source for the Sent copy', async () => {
    const message = {
      to: ['bob@other.com'],
      cc: ['team@other.com'],
      bcc: ['audit@other.com'],
      replyTo: ['replies@example.net'],
      subject: 'Retained MIME source — café',
      text: 'Plain-text body',
      html: '<p>HTML body</p>',
      inReplyTo: '<parent@other.com>',
      references: ['<root@other.com>', '<parent@other.com>'],
      attachments: [{
        filename: 'notes.txt',
        contentType: 'text/plain',
        contentBase64: Buffer.from('attachment bytes').toString('base64'),
      }],
    }
    const res = await call(Send, 'POST', req({ token: auth(), body: message, ip: '10.0.0.12' }))
    const source = await app.rawMessages.findForEmail(res.messageId)
    expect(source).toBeTruthy()

    const bytes = await app.rawMessages.readContent(source)
    const parsed = await simpleParser(Buffer.from(bytes))
    expect(parsed.subject).toBe(message.subject)
    expect(parsed.text).toContain(message.text)
    expect(parsed.html).toContain(message.html)
    expect(parsed.replyTo?.value[0]?.address).toBe('replies@example.net')
    expect(parsed.inReplyTo).toBe('<parent@other.com>')
    expect(parsed.references).toEqual(message.references)
    expect(parsed.attachments[0]?.content.toString()).toBe('attachment bytes')
    const [, sentCopy] = await app.emails.findById(res.messageId)
    expect(sentCopy.inReplyTo).toBe(message.inReplyTo)
    expect(sentCopy.references).toEqual(message.references)
    expect(sentCopy.to).toEqual(message.to)
    expect(sentCopy.cc).toEqual(message.cc)
    expect(sentCopy.bcc).toEqual(message.bcc)
    expect(sentCopy.threadId).toMatch(/^[a-f0-9]{64}$/)
  })

  it('accepts an attachment', async () => {
    const res = await call(Send, 'POST', req({
      token: auth(),
      body: { ...base, attachments: [{ filename: 'a.txt', contentType: 'text/plain', contentBase64: Buffer.from('hi').toString('base64') }] },
    }))
    expect(res.error).toBeUndefined()
  })

  it('validates, sends, and stores an explicit Reply-To mailbox', async () => {
    const invalid = await call(Send, 'POST', req({ token: auth(), body: { ...base, replyTo: ['not-an-address'] } }))
    expect(invalid.error).toMatch(/replyTo/)

    const res = await call(Send, 'POST', req({
      token: auth(),
      body: { ...base, replyTo: ['Replies@Example.net'] },
      ip: '10.0.0.11',
    }))
    const [, sent] = await app.emails.findById(res.messageId)
    expect(sent.replyTo).toEqual(['replies@example.net'])
  })

  it('sends from an exact owned alias and rejects same-domain impersonation', async () => {
    const denied = await call(Send, 'POST', req({
      token: auth(), ip: '10.0.0.21', body: { ...base, from: 'bob@example.com' },
    }))
    expect(denied.forbidden).toBe(true)

    const res = await call(Send, 'POST', req({
      token: auth(), ip: '10.0.0.22', body: { ...base, from: 'Sales@Example.com', subject: 'Alias sender' },
    }))
    const [, sent] = await app.emails.findById(res.messageId)
    expect(sent.sender).toBe('sales@example.com')
    expect(sent.domain).toBe('example.com')
    const source = await app.rawMessages.findForEmail(res.messageId)
    const parsed = await simpleParser(Buffer.from(await app.rawMessages.readContent(source)))
    expect(parsed.from?.value[0]?.address).toBe('sales@example.com')
  })

  it('archives the authorized source conversation and restores it on undo', async () => {
    const root = {
      id: 'archive-source-root', domain: 'example.com', ownerEmail: 'alice@example.com', recipient: 'alice@example.com',
      sender: 'bob@other.com', subject: 'Archive source', body: 'root', folder: 'inbox', read: true, starred: false,
      receivedAt: '2026-08-14T10:00:00.000Z', processed: true, rfcMessageId: '<archive-root@other.com>',
    }
    const reply = {
      ...root, id: 'archive-source-reply', body: 'reply', receivedAt: '2026-08-14T10:01:00.000Z',
      rfcMessageId: '<archive-reply@other.com>', inReplyTo: '<archive-root@other.com>', references: ['<archive-root@other.com>'],
    }
    await app.emails.put(root); await app.emails.put(reply)
    const sent = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.23', body: {
      ...base, subject: 'Re: Archive source', inReplyTo: reply.rfcMessageId,
      references: [root.rfcMessageId, reply.rfcMessageId], delayMs: 60_000,
      archive: true, archiveSourceId: reply.id,
    } }))
    expect(sent).toMatchObject({ archived: true, archivedCount: 2 })
    expect((await app.emails.findById(root.id))[1].folder).toBe('archive')
    expect((await app.emails.findById(reply.id))[1].folder).toBe('archive')
    expect((await app.outbox.findById(sent.undoId))[1].archiveStateJson).toContain(root.id)

    const [replyDocId] = await app.emails.findById(reply.id)
    await app.emails.update(replyDocId, { folder: 'trash' })
    const undone = await call(Undo, 'POST', req({ token: auth(), body: { undoId: sent.undoId } }))
    expect(undone).toMatchObject({ undone: true, archiveRestored: 1 })
    expect((await app.emails.findById(root.id))[1].folder).toBe('inbox')
    expect((await app.emails.findById(reply.id))[1].folder).toBe('trash')
  })

  it('requires an authorized source for Send & Archive', async () => {
    expect((await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.24', body: { ...base, archive: true } }))).error)
      .toMatch(/archiveSourceId/)
    const foreign = await call(Send, 'POST', req({
      token: token({ email: 'carol@other.com' }), ip: '10.0.0.25',
      body: { ...base, to: ['recipient@example.com'], archive: true, archiveSourceId: 'archive-source-root' },
    }))
    expect(foreign.notFound).toBe(true)
  })

  it('400s on an unusable sendAt', async () => {
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, sendAt: 'nope' } }))).error).toMatch(/valid ISO/)
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, sendAt: '2000-01-01T00:00:00.000Z' } }))).error).toMatch(/in the future/)
  })

  it('schedules a future send', async () => {
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, sendAt: future() } }))
    expect(res.error).toBeUndefined()
    const list = await call(Scheduled, 'GET', req({ token: auth() }))
    const item = list.find(entry => entry.id === res.id)
    expect(item.status).toBe('queued')
    expect(item.nextAttempt).toBeTruthy()
    expect((await app.outbox.findById(res.id))[1]?.scheduledId).toBe(res.id)
  })

  it('loads and edits a scheduled message without changing its canonical id', async () => {
    const created = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.40', body: {
      ...base,
      subject: 'Before edit',
      sendAt: future(),
      attachments: [{ filename: 'before.txt', contentType: 'text/plain', contentBase64: Buffer.from('before').toString('base64') }],
    } }))
    const detail = await call(ScheduledItem, 'GET', req({ token: auth(), paths: { id: created.id } }))
    expect(detail.attachments[0].contentBase64).toBeTruthy()
    const later = new Date(Date.now() + 7_200_000).toISOString()
    const updated = await call(ScheduledItem, 'PUT', req({ token: auth(), paths: { id: created.id }, body: {
      subject: 'After edit', text: 'updated body', html: '<p>updated body</p>', sendAt: later, attachments: [],
    } }))
    expect(updated.id).toBe(created.id)
    const after = await call(ScheduledItem, 'GET', req({ token: auth(), paths: { id: created.id } }))
    expect(after.subject).toBe('After edit')
    expect(after.sendAt).toBe(later)
    expect(after.attachments).toEqual([])
    const [, queue] = await app.outbox.findById(created.id)
    expect(queue.email.subject).toBe('After edit')
    expect(queue.attempts).toBe(0)
  })

  it('protects scheduled detail and edits by exact mailbox owner', async () => {
    const created = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.41', body: { ...base, subject: 'Private schedule', sendAt: future() } }))
    const other = token({ email: 'bob@example.com' })
    expect((await call(ScheduledItem, 'GET', req({ token: other, paths: { id: created.id } }))).notFound).toBe(true)
    expect((await call(ScheduledItem, 'PUT', req({ token: other, paths: { id: created.id }, body: { subject: 'Stolen' } }))).notFound).toBe(true)
  })

  it('validates and schedules a no-reply follow-up reminder', async () => {
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, remindAt: 'nope' } }))).error).toMatch(/valid ISO/)
    const remindAt = future()
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, remindAt } }))
    expect(res.reminderId).toMatch(/^reminder-/)
    const list = await call(Reminders, 'GET', req({ token: auth() }))
    expect(list.some(item => item.id === res.reminderId && item.remindAt === remindAt)).toBe(true)
  })

  it('requires a reminder for scheduled mail to occur after delivery', async () => {
    const sendAt = new Date(Date.now() + 7_200_000).toISOString()
    const remindAt = new Date(Date.now() + 3_600_000).toISOString()
    expect((await call(Send, 'POST', req({ token: auth(), body: { ...base, sendAt, remindAt } }))).error)
      .toMatch(/after sendAt/)
  })

  it('queues a delayed send that can be undone', async () => {
    const res = await call(Send, 'POST', req({ token: auth(), body: { ...base, delayMs: 60_000 } }))
    expect(res.undoId ?? res.id).toBeTruthy()
  })

  it('429s once the hourly limit is exhausted', async () => {
    let res
    for (let i = 0; i < 62; i++) res = await call(Send, 'POST', req({ token: auth(), body: base, ip: '10.0.0.5' }))
    expect(res.rateLimited).toBe(true)
  })
})

describe('DELETE /send/scheduled', () => {
  it('400s without an id', async () => {
    expect((await call(Scheduled, 'DELETE', req({ token: auth(), body: {} }))).error).toMatch(/id is required/)
  })

  it('404s for an unknown id', async () => {
    expect((await call(Scheduled, 'DELETE', req({ token: auth(), body: { id: 'ghost' } }))).notFound).toBe(true)
  })

  it('404s for another sender scheduled mail', async () => {
    const [entry] = await call(Scheduled, 'GET', req({ token: auth() }))
    const res = await call(Scheduled, 'DELETE', req({ token: token({ email: 'bob@example.com' }), body: { id: entry.id } }))
    expect(res.notFound).toBe(true)
  })

  it('cancels the scheduled send', async () => {
    const [entry] = await call(Scheduled, 'GET', req({ token: auth() }))
    expect((await call(Scheduled, 'DELETE', req({ token: auth(), body: { id: entry.id } }))).cancelled).toBe(true)
    expect((await app.outbox.findById(entry.id))[1]).toBeNull()
  })

  it('refuses cancellation during an active delivery lease', async () => {
    const created = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.42', body: { ...base, subject: 'Claimed', sendAt: future() } }))
    const [queueDocId] = await app.outbox.findById(created.id)
    await app.outbox.update(queueDocId, { status: 'sending', claimedAt: Date.now() })
    expect((await call(ScheduledItem, 'DELETE', req({ token: auth(), paths: { id: created.id } }))).conflict).toBe(true)
  })
})

describe('POST /send/undo', () => {
  it('400s without an undoId', async () => {
    expect((await call(Undo, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/undoId is required/)
  })

  it('404s for an unknown undoId', async () => {
    expect((await call(Undo, 'POST', req({ token: auth(), body: { undoId: 'ghost' } }))).notFound).toBe(true)
  })

  it('cancels a delayed send', async () => {
    const queued = await call(Send, 'POST', req({ token: auth(), body: { ...base, delayMs: 60_000 }, ip: '10.0.0.7' }))
    const undoId = queued.undoId ?? queued.id
    expect((await call(Undo, 'POST', req({ token: auth(), body: { undoId } }))).undone).toBe(true)
  })

  it('cannot undo another mailbox send and dismisses its own reminder on undo', async () => {
    const queued = await call(Send, 'POST', req({
      token: auth(), ip: '10.0.0.8',
      body: { ...base, delayMs: 60_000, remindAt: future() },
    }))
    expect((await call(Undo, 'POST', req({ token: token({ email: 'bob@example.com' }), body: { undoId: queued.undoId } }))).notFound).toBe(true)
    expect((await call(Undo, 'POST', req({ token: auth(), body: { undoId: queued.undoId } }))).undone).toBe(true)
    expect((await call(Reminders, 'GET', req({ token: auth() }))).some(item => item.id === queued.reminderId)).toBe(false)
  })
})

describe('POST /send/flush', () => {
  it('requires a system signature and returns queue health around a recovery pass', async () => {
    await app.outboundQueue.enqueue('alice@example.com', base, { availableAt: 0 })
    expect((await call(Flush, 'POST', req({ token: auth(), body: {} }))).unauthorized).toBe(true)

    const secret = process.env.INBOUND_WEBHOOK_SECRET || ''
    const signature = createHmac('sha256', secret).update('{}').digest('hex')
    const result = await call(Flush, 'POST', req({ body: {}, headers: { 'X-Caduceus-Signature': signature } }))
    expect(result.before.depth).toBeGreaterThan(0)
    expect(result.delivered).toBeGreaterThan(0)
    expect(result.queue.due).toBe(0)
  })
})

describe('/reminders', () => {
  it('sets a reminder on an existing sent message and protects mailbox ownership', async () => {
    const sent = await call(Send, 'POST', req({ token: auth(), ip: '10.0.0.9', body: base }))
    const remindAt = future()
    expect((await call(Reminders, 'POST', req({
      token: token({ email: 'bob@example.com' }), body: { emailId: sent.messageId, remindAt },
    }))).notFound).toBe(true)
    const created = await call(Reminders, 'POST', req({ token: auth(), body: { emailId: sent.messageId, remindAt } }))
    expect(created.scheduled).toBe(true)
    expect((await call(Reminders, 'DELETE', req({ token: auth(), body: { id: created.id } }))).dismissed).toBe(true)
  })

  it('runs a signed all-mailbox due pass without an inbox read', async () => {
    const due = await app.reminders.schedule({
      ownerEmail: 'alice@example.com', emailId: 'worker-sent', recipient: 'bob@other.com',
      subject: 'Worker follow-up', remindAt: Date.now() - 1,
    })
    expect((await call(ReminderTick, 'POST', req({ body: {} }))).unauthorized).toBe(true)
    const secret = process.env.INBOUND_WEBHOOK_SECRET || ''
    const signature = createHmac('sha256', secret).update('{}').digest('hex')
    expect(await call(ReminderTick, 'POST', req({ body: {}, headers: { 'X-Caduceus-Signature': signature } })))
      .toEqual({ due: 1, surfaced: 1, reconciled: 0, failed: 0 })
    expect((await app.emails.findById(`follow-up-${due.id}`))[1]).toMatchObject({
      ownerEmail: 'alice@example.com', folder: 'inbox', read: false,
    })
  })
})
