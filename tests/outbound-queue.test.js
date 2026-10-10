import { test, expect } from 'bun:test'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { Providers } from '../scripts/support/Providers.mjs'
import OutboundQueueService from '../server/services/OutboundQueueService.js'

/** @param {Error|null} err */
function mockSmtp(err) {
  return { async sendEmail() { if (err) throw err; return { messageId: 'ok' } } }
}
/** @param {number} code @param {string} msg */
function smtpErr(code, msg) { return Object.assign(new Error(msg), { responseCode: code }) }

const EMAIL = { to: ['a@dest.test'], subject: 'Hi', text: 'body' }
async function freshApp() { return Providers.container(mkdtempSync(join(tmpdir(), 'caduceus-q-'))) }

test('delivers a due message and clears it from the queue', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(null), app.emails, app.scheduled, app.attachments)
  await q.enqueue('me@x.test', EMAIL, { availableAt: Date.now() - 1 })
  const r = await q.processDue()
  expect(r).toEqual({ delivered: 1, retried: 0, bounced: 0 })
  expect(Object.keys(await app.outbox.findDue()).length).toBe(0)
})

test('retries a transient (4xx) failure with backoff, keeping it queued', async () => {
  const app = await freshApp()
  const now = Date.now()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(smtpErr(451, 'try later')), app.emails, app.scheduled, app.attachments)
  const id = await q.enqueue('me@x.test', EMAIL, { availableAt: now - 1 })
  const r = await q.processDue(now)
  expect(r.retried).toBe(1)
  const [, entry] = await app.outbox.findById(id)
  expect(entry.attempts).toBe(1)
  expect(entry.nextAttempt).toBeGreaterThan(now) // rescheduled into the future
  expect(Object.keys(await app.outbox.findDue(now)).length).toBe(0) // no longer due
})

test('bounces a permanent (5xx) failure into the sender mailbox', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(smtpErr(550, 'no such user')), app.emails, app.scheduled, app.attachments)
  await q.enqueue('me@x.test', EMAIL, { availableAt: Date.now() - 1 })
  const r = await q.processDue()
  expect(r.bounced).toBe(1)
  expect(Object.keys(await app.outbox.findDue()).length).toBe(0)
  const inbox = await app.emails.list(['x.test'], {})
  const bounce = inbox.find(e => e.subject.startsWith('Undelivered:'))
  expect(bounce).toBeTruthy()
  expect(bounce.recipient).toBe('me@x.test')
  expect(bounce.body).toContain('no such user')
})

test('bounces once transient retries are exhausted', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(smtpErr(451, 'try later')), app.emails, app.scheduled, app.attachments)
  const id = await q.enqueue('me@x.test', EMAIL, { availableAt: 0 })
  const [docId] = await app.outbox.findById(id)
  await app.outbox.update(docId, { attempts: q.maxAttempts - 1, nextAttempt: 0, status: 'queued' })
  const r = await q.processDue(Date.now())
  expect(r.bounced).toBe(1)
  const inbox = await app.emails.list(['x.test'], {})
  expect(inbox.some(e => e.subject.startsWith('Undelivered:'))).toBe(true)
})

test('keeps alias delivery separate from mailbox ownership', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(smtpErr(550, 'alias rejected')), app.emails, app.scheduled, app.attachments)
  await q.enqueue('sales@x.test', EMAIL, { owner: 'me@x.test', availableAt: Date.now() - 1 })
  await q.processDue()
  const inbox = await app.emails.list(['x.test'], {})
  expect(inbox.find(email => email.subject.startsWith('Undelivered:'))?.recipient).toBe('me@x.test')
})

test('delivers linked scheduled work and removes its management record', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(null), app.emails, app.scheduled, app.attachments)
  const id = 'scheduled-worker-test'
  await app.scheduled.put({ id, email: EMAIL, sender: 'me@x.test', owner: 'me@x.test', sendAt: 0, createdAt: 0, queueId: id })
  await q.enqueue('me@x.test', EMAIL, { id, scheduledId: id, availableAt: 0 })
  expect((await q.processDue()).delivered).toBe(1)
  expect((await app.scheduled.findById(id))[1]).toBeNull()
})

test('reconciles and delivers a metadata-only schedule from an older release', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(null), app.emails, app.scheduled, app.attachments)
  const id = 'scheduled-legacy-test'
  await app.scheduled.put({ id, email: EMAIL, sender: 'me@x.test', owner: 'me@x.test', sendAt: 0, createdAt: 0 })
  expect((await app.outbox.findById(id))[1]).toBeNull()
  expect((await q.processDue()).delivered).toBe(1)
  expect((await app.scheduled.findById(id))[1]).toBeNull()
})

test('reclaims a stale delivery lease after a worker restart', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(null), app.emails, app.scheduled, app.attachments)
  const id = await q.enqueue('me@x.test', EMAIL, { availableAt: 0 })
  const [docId] = await app.outbox.findById(id)
  await app.outbox.update(docId, { status: 'sending', claimedAt: Date.now() - 10 * 60_000 })
  expect((await q.processDue()).delivered).toBe(1)
})

test('serializes overlapping delivery ticks in the persistent writer', async () => {
  const app = await freshApp()
  let sends = 0
  const smtp = { async sendEmail() { sends++; await new Promise(resolve => setTimeout(resolve, 10)) } }
  const q = Providers.create(OutboundQueueService, app.outbox, smtp, app.emails, app.scheduled, app.attachments)
  await q.enqueue('me@x.test', EMAIL, { availableAt: 0 })
  await Promise.all([q.processDue(), q.processDue()])
  expect(sends).toBe(1)
})

test('reports bounded secret-free queue health for operations', async () => {
  const app = await freshApp()
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(null), app.emails, app.scheduled, app.attachments)
  await q.enqueue('me@x.test', EMAIL, { availableAt: 500 })
  const secondID = await q.enqueue('me@x.test', EMAIL, { availableAt: 2000 })
  const [secondDocID] = await app.outbox.findById(secondID)
  await app.outbox.update(secondDocID, { attempts: 2, status: 'sending', claimedAt: 1000 })
  expect(await q.status(1000)).toEqual({ depth: 2, queued: 1, sending: 1, due: 1, oldestAgeMs: 0, attempts: 2 })
})

test('never retries recipients already accepted during a partial multi-domain delivery', async () => {
  const app = await freshApp()
  const error = Object.assign(new Error('partial delivery'), {
    deliveryResults: [
      { deliveredRecipients: ['ok@good.test'], failedRecipients: [], responseCode: 250 },
      { deliveredRecipients: [], failedRecipients: ['later@grey.test'], responseCode: 451 },
    ],
  })
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(error), app.emails, app.scheduled, app.attachments)
  const id = await q.enqueue('me@x.test', { to: ['ok@good.test'], cc: ['later@grey.test'], subject: 'Partial', text: 'body' }, { availableAt: 0 })
  expect(await q.processDue(1)).toEqual({ delivered: 0, retried: 1, bounced: 0 })
  const [, queued] = await app.outbox.findById(id)
  expect(queued.email.envelopeRecipients).toEqual(['later@grey.test'])
  expect(queued.email.to).toEqual(['ok@good.test'])
})

test('retries recipients omitted from malformed delivery diagnostics', async () => {
  const app = await freshApp()
  const error = Object.assign(new Error('incomplete diagnostics'), {
    deliveryResults: [{ deliveredRecipients: ['ok@good.test'], failedRecipients: [], responseCode: 250 }],
  })
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(error), app.emails, app.scheduled, app.attachments)
  const id = await q.enqueue('me@x.test', { to: ['ok@good.test', 'unknown@good.test'], subject: 'Partial', text: 'body' }, { availableAt: 0 })
  expect((await q.processDue(1)).retried).toBe(1)
  const [, queued] = await app.outbox.findById(id)
  expect(queued.email.envelopeRecipients).toEqual(['unknown@good.test'])
})

test('bounces and suppresses only permanent recipients while retaining transient work', async () => {
  const app = await freshApp()
  const error = Object.assign(new Error('mixed\r\nremote response'), {
    deliveryResults: [
      { deliveredRecipients: [], failedRecipients: ['gone@bad.test'], responseCode: 550 },
      { deliveredRecipients: [], failedRecipients: ['later@grey.test'], responseCode: 451 },
    ],
  })
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(error), app.emails, app.scheduled, app.attachments, app.categoryMappings, app.suppressed)
  const id = await q.enqueue('me@x.test', { to: ['gone@bad.test', 'later@grey.test'], subject: 'Mixed', text: 'body' }, { availableAt: 0 })
  expect(await q.processDue(1)).toEqual({ delivered: 0, retried: 1, bounced: 1 })
  const [, queued] = await app.outbox.findById(id)
  expect(queued.email.envelopeRecipients).toEqual(['later@grey.test'])
  expect(queued.lastError).not.toContain('\r')
  const inbox = await app.emails.list(['x.test'], {})
  const bounce = inbox.find(email => email.bounce)
  expect(bounce.body).toContain('gone@bad.test')
  expect(bounce.body).not.toContain('later@grey.test')
  expect((await app.suppressed.list()).map(item => item.address)).toContain('gone@bad.test')
})

test('uses recipient-scoped reply codes for mixed failures from one MX', async () => {
  const app = await freshApp()
  const error = Object.assign(new Error('mixed recipient replies'), {
    deliveryResults: [{
      deliveredRecipients: ['ok@mixed.test'],
      failedRecipients: ['later@mixed.test', 'gone@mixed.test'],
      responseCode: 451,
      recipientFailures: [
        { recipient: 'later@mixed.test', responseCode: 451 },
        { recipient: 'gone@mixed.test', responseCode: 550 },
      ],
    }],
  })
  const q = Providers.create(OutboundQueueService, app.outbox, mockSmtp(error), app.emails, app.scheduled, app.attachments, app.categoryMappings, app.suppressed)
  const id = await q.enqueue('me@x.test', {
    to: ['ok@mixed.test', 'later@mixed.test', 'gone@mixed.test'], subject: 'Same MX', text: 'body',
  }, { availableAt: 0 })

  expect(await q.processDue(1)).toEqual({ delivered: 0, retried: 1, bounced: 1 })
  expect((await app.outbox.findById(id))[1].email.envelopeRecipients).toEqual(['later@mixed.test'])
  expect((await app.suppressed.list()).map(item => item.address)).toContain('gone@mixed.test')
})
