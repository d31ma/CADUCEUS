import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { Handler as Snooze } from '@/routes/snooze/yon.js'
import { Handler as SnoozeItem } from '@/routes/snooze/_id/yon.js'

let app
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('thread-level snooze lifecycle', () => {
  let root
  let reply
  let sent
  let snoozeId

  it('moves every Inbox message in the RFC conversation and labels the due time', async () => {
    root = email({ id: 'snooze-root', rfcMessageId: '<snooze-root@other.test>', subject: 'Project status', read: true })
    reply = email({
      id: 'snooze-reply', rfcMessageId: '<snooze-reply@example.com>',
      inReplyTo: '<snooze-root@other.test>', references: ['<snooze-root@other.test>'],
      subject: 'Re: Project status', read: true,
    })
    sent = email({
      id: 'snooze-sent', folder: 'sent', sender: 'alice@example.com', recipient: 'bob@other.com',
      rfcMessageId: '<snooze-sent@example.com>', inReplyTo: '<snooze-reply@example.com>',
      references: ['<snooze-root@other.test>', '<snooze-reply@example.com>'], subject: 'Re: Project status',
    })
    await app.emails.put(root); await app.emails.put(reply); await app.emails.put(sent)
    const until = new Date(Date.now() + 60_000).toISOString()
    const request = { emailId: root.id, until, label: 'Later Today', clientRequestId: 'snooze-client-request-1' }
    const result = await Snooze.POST(req({ token: alice(), body: request }))
    expect(result).toMatchObject({ snoozed: true, count: 2, until })
    expect(await Snooze.POST(req({ token: alice(), body: request }))).toEqual(result)
    snoozeId = result.snoozeId
    expect((await app.emails.findById(root.id))[1]).toMatchObject({ folder: 'snoozed', snoozeId, snoozeLabel: 'Later Today', snoozedUntil: until })
    expect((await app.emails.findById(reply.id))[1].folder).toBe('snoozed')
    expect((await app.emails.findById(sent.id))[1].folder).toBe('sent')
    expect(await Snooze.GET(req({ token: bob() }))).toEqual([])
  })

  it('provides exact-owner undo without forcing the conversation unread', async () => {
    expect((await SnoozeItem.DELETE(req({ token: bob(), paths: { id: snoozeId } }))).notFound).toBe(true)
    expect(await SnoozeItem.DELETE(req({ token: alice(), paths: { id: snoozeId } }))).toMatchObject({ snoozed: false, count: 2 })
    expect((await app.emails.findById(root.id))[1]).toMatchObject({ folder: 'inbox', read: true })
    expect((await app.emails.findById(reply.id))[1]).toMatchObject({ folder: 'inbox', read: true })
    expect((await SnoozeItem.DELETE(req({ token: alice(), paths: { id: snoozeId } }))).notFound).toBe(true)
  })

  it('returns due conversations unread exactly once and notifies the owner', async () => {
    const result = await Snooze.POST(req({
      token: alice(), body: { emailId: root.id, until: new Date(Date.now() + 60_000).toISOString(), label: 'Custom' },
    }))
    snoozeId = result.snoozeId
    const [docId] = await app.snooze.findById(snoozeId)
    await app.snooze.update(docId, { until: Date.now() - 1 })
    const notifications = []
    app.push.notifyUnsnoozed = async message => { notifications.push(message.id) }
    const releasedAt = Date.now()
    expect(await app.snoozeService.processDue(releasedAt)).toEqual({ due: 1, returned: 2, failed: 0, notified: 1 })
    expect((await app.emails.findById(root.id))[1]).toMatchObject({ folder: 'inbox', read: false })
    expect((await app.emails.findById(reply.id))[1]).toMatchObject({ folder: 'inbox', read: false })
    expect((await app.emails.findById(root.id))[1].receivedAt).toBe(new Date(releasedAt).toISOString())
    expect(notifications).toHaveLength(1)
    expect(await app.snoozeService.processDue()).toEqual({ due: 0, returned: 0, failed: 0, notified: 0 })
  })

  it('preserves message chronology when return-to-top is disabled', async () => {
    const [userDocId, user] = await app.users.findByEmail('alice@example.com')
    await app.users.updatePreferences(userDocId, {
      ...user,
      mailPreferences: { ...user.mailPreferences, unsnoozeToTop: false },
    })
    const originalReceivedAt = (await app.emails.findById(root.id))[1].receivedAt
    const result = await Snooze.POST(req({
      token: alice(), body: { emailId: root.id, until: new Date(Date.now() + 60_000).toISOString(), label: 'Keep position' },
    }))
    const [docId] = await app.snooze.findById(result.snoozeId)
    await app.snooze.update(docId, { until: Date.now() - 1 })
    await app.snoozeService.processDue(Date.now())
    expect((await app.emails.findById(root.id))[1].receivedAt).toBe(originalReceivedAt)
  })

  it('does not pull a manually moved message back into Inbox when due', async () => {
    const result = await Snooze.POST(req({
      token: alice(), body: { emailId: root.id, until: new Date(Date.now() + 60_000).toISOString(), label: 'Tomorrow' },
    }))
    const [, movedReply] = await app.emails.findById(reply.id)
    const replyDoc = (await app.emails.findById(reply.id))[0]
    expect(movedReply.folder).toBe('snoozed')
    await app.emails.update(replyDoc, { folder: 'archive' })
    const [docId] = await app.snooze.findById(result.snoozeId)
    await app.snooze.update(docId, { until: Date.now() - 1 })
    expect(await app.snoozeService.processDue()).toMatchObject({ due: 1, returned: 1, failed: 0 })
    expect((await app.emails.findById(reply.id))[1].folder).toBe('archive')
  })
})
