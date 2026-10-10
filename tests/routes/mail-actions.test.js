import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { Handler as MailActions } from '@/routes/inbox/actions/yon.js'
import { Handler as UndoMailAction } from '@/routes/inbox/actions/_id/undo/yon.js'
import { Handler as RedoMailAction } from '@/routes/inbox/actions/_id/redo/yon.js'

let app
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'bob@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('durable reversible mailbox actions', () => {
  it('validates a bounded non-empty action before touching mail', async () => {
    expect(await MailActions.POST(req({ token: alice(), body: {} }))).toMatchObject({ error: expect.any(String) })
    expect(await MailActions.POST(req({ token: alice(), body: { ids: [], patch: { read: true } } }))).toMatchObject({ error: expect.any(String) })
    expect(await MailActions.POST(req({ token: alice(), body: { ids: ['one'], patch: { read: 'yes' } } }))).toMatchObject({ error: expect.any(String) })
    expect(await MailActions.POST(req({ token: alice(), body: { ids: Array.from({ length: 101 }, (_, index) => `over-${index}`), patch: { read: true } } }))).toMatchObject({ error: expect.any(String) })
  })

  it('authorizes the complete set before applying an all-or-nothing bulk action', async () => {
    const owned = email({ id: 'bulk-owned', ownerEmail: 'alice@example.com' })
    const foreign = email({ id: 'bulk-foreign', ownerEmail: 'bob@example.com', recipient: 'bob@example.com' })
    await app.emails.put(owned)
    await app.emails.put(foreign)

    const denied = await MailActions.POST(req({
      token: alice(),
      body: { ids: [owned.id, foreign.id], patch: { folder: 'archive' } },
    }))
    expect(denied.notFound).toBe(true)
    expect((await app.emails.findById(owned.id))[1].folder).toBe('inbox')
    expect((await app.emails.findById(foreign.id))[1].folder).toBe('inbox')
  })

  it('applies, undoes, and redoes one journaled bulk mutation for the exact owner', async () => {
    const first = email({ id: 'bulk-first', ownerEmail: 'alice@example.com', read: false })
    const second = email({ id: 'bulk-second', ownerEmail: 'alice@example.com', read: false })
    await app.emails.put(first)
    await app.emails.put(second)

    const applied = await MailActions.POST(req({
      token: alice(),
      body: { ids: [first.id, second.id], patch: { folder: 'archive', read: true } },
    }))
    const actionId = applied.actionId
    expect(applied).toMatchObject({
      actionId: expect.stringMatching(/^[A-Za-z0-9_-]+$/),
      status: 'applied',
      count: 2,
      kind: 'bulk',
      emails: expect.arrayContaining([
        expect.objectContaining({ id: first.id, folder: 'archive', read: true }),
        expect.objectContaining({ id: second.id, folder: 'archive', read: true }),
      ]),
    })
    expect((await UndoMailAction.POST(req({ token: bob(), paths: { id: actionId } }))).notFound).toBe(true)
    expect((await app.emails.findById(first.id))[1]).toMatchObject({ folder: 'archive', read: true })

    const undone = await UndoMailAction.POST(req({ token: alice(), paths: { id: actionId } }))
    expect(undone).toMatchObject({ status: 'undone', count: 2 })
    expect((await app.emails.findById(first.id))[1]).toMatchObject({ folder: 'inbox', read: false })
    expect((await app.emails.findById(second.id))[1]).toMatchObject({ folder: 'inbox', read: false })

    const redone = await RedoMailAction.POST(req({ token: alice(), paths: { id: actionId } }))
    expect(redone).toMatchObject({ status: 'applied', count: 2 })
    expect((await app.emails.findById(first.id))[1]).toMatchObject({ folder: 'archive', read: true })
    expect((await app.emails.findById(second.id))[1]).toMatchObject({ folder: 'archive', read: true })
  })

  it('adds and removes nested labels across a mixed selection with durable undo and redo', async () => {
    const first = email({
      id: 'labels-first', ownerEmail: 'alice@example.com', labels: ['customers/enterprise'],
    })
    const second = email({ id: 'labels-second', ownerEmail: 'alice@example.com', labels: [] })
    await app.emails.put(first)
    await app.emails.put(second)

    const applied = await MailActions.POST(req({
      token: alice(),
      body: { ids: [first.id, second.id], patch: { labels: { add: ['Projects/Launch'], remove: [] } } },
    }))
    expect(applied).toMatchObject({
      status: 'applied', kind: 'bulk', count: 2,
      emails: expect.arrayContaining([
        expect.objectContaining({ id: first.id, labels: ['customers/enterprise', 'projects/launch'] }),
        expect.objectContaining({ id: second.id, labels: ['projects/launch'] }),
      ]),
    })

    const removed = await MailActions.POST(req({
      token: alice(),
      body: { ids: [first.id, second.id], patch: { labels: { remove: ['projects/launch'] } } },
    }))
    expect((await app.emails.findById(first.id))[1].labels).toEqual(['customers/enterprise'])
    expect((await app.emails.findById(second.id))[1].labels).toEqual([])

    expect(await UndoMailAction.POST(req({ token: alice(), paths: { id: removed.actionId } }))).toMatchObject({
      status: 'undone',
    })
    expect((await app.emails.findById(second.id))[1].labels).toEqual(['projects/launch'])
    expect(await RedoMailAction.POST(req({ token: alice(), paths: { id: removed.actionId } }))).toMatchObject({
      status: 'applied',
    })
    expect((await app.emails.findById(second.id))[1].labels).toEqual([])
  })

  it('rejects malformed, ambiguous, and traversal label operations atomically', async () => {
    const message = email({ id: 'labels-invalid', ownerEmail: 'alice@example.com', labels: [] })
    await app.emails.put(message)
    for (const labels of [
      { add: ['../secret'] },
      { add: ['team'], remove: ['TEAM'] },
      { add: Array.from({ length: 101 }, (_, index) => `label-${index}`) },
    ]) {
      expect(await MailActions.POST(req({
        token: alice(), body: { ids: [message.id], patch: { labels } },
      }))).toMatchObject({ error: expect.any(String) })
    }
    expect((await app.emails.findById(message.id))[1].labels).toEqual([])
  })

  it('replays a client request idempotently and rejects key reuse for another mutation', async () => {
    const message = email({ id: 'action-idempotent', ownerEmail: 'alice@example.com', read: false })
    await app.emails.put(message)
    const clientRequestId = '11111111-1111-4111-8111-111111111111'
    const body = { ids: [message.id], patch: { read: true }, clientRequestId }
    const first = await MailActions.POST(req({ token: alice(), body }))
    const replay = await MailActions.POST(req({ token: alice(), body }))
    expect(replay).toMatchObject({ actionId: first.actionId, status: 'applied', count: 1 })
    expect((await app.mailActionRecords.listForOwner('alice@example.com')).filter(([, item]) => item.clientRequestId === clientRequestId)).toHaveLength(1)

    const collision = await MailActions.POST(req({
      token: alice(), body: { ids: [message.id], patch: { starred: true }, clientRequestId },
    }))
    expect(collision.conflict).toBe(true)
  })

  it('preserves unrelated later edits but refuses to overwrite a conflicting field', async () => {
    const message = email({ id: 'action-conflict', ownerEmail: 'alice@example.com', read: false, starred: false })
    await app.emails.put(message)

    const marked = await MailActions.POST(req({
      token: alice(), body: { ids: [message.id], patch: { read: true } },
    }))
    const [docId] = await app.emails.findById(message.id)
    await app.emails.update(docId, { starred: true })
    expect(await UndoMailAction.POST(req({ token: alice(), paths: { id: marked.actionId } }))).toMatchObject({ status: 'undone' })
    expect((await app.emails.findById(message.id))[1]).toMatchObject({ read: false, starred: true })

    const moved = await MailActions.POST(req({
      token: alice(), body: { ids: [message.id], patch: { folder: 'archive' } },
    }))
    await app.emails.update(docId, { folder: 'trash' })
    const conflicted = await UndoMailAction.POST(req({ token: alice(), paths: { id: moved.actionId } }))
    expect(conflicted.conflict).toBe(true)
    expect((await app.emails.findById(message.id))[1].folder).toBe('trash')
  })

  it('finishes an interrupted transition before serving the owner retry', async () => {
    const message = email({ id: 'action-recovery', ownerEmail: 'alice@example.com', read: false })
    await app.emails.put(message)
    const applied = await MailActions.POST(req({
      token: alice(), body: { ids: [message.id], patch: { read: true } },
    }))
    const [actionDocId] = await app.mailActionRecords.findById(applied.actionId)
    await app.mailActionRecords.update(actionDocId, { status: 'undoing' })

    const recovered = await UndoMailAction.POST(req({ token: alice(), paths: { id: applied.actionId } }))
    expect(recovered).toMatchObject({ actionId: applied.actionId, status: 'undone', count: 1 })
    expect((await app.emails.findById(message.id))[1].read).toBe(false)
    expect((await app.mailActionRecords.findById(applied.actionId))[1].status).toBe('undone')
  })

  it('fails closed after the bounded undo window expires', async () => {
    const message = email({ id: 'action-expired', ownerEmail: 'alice@example.com' })
    await app.emails.put(message)
    const applied = await MailActions.POST(req({
      token: alice(), body: { ids: [message.id], patch: { starred: true } },
    }))
    const [actionDocId] = await app.mailActionRecords.findById(applied.actionId)
    await app.mailActionRecords.update(actionDocId, { expiresAt: Date.now() - 1 })

    const expired = await UndoMailAction.POST(req({ token: alice(), paths: { id: applied.actionId } }))
    expect(expired.conflict).toBe(true)
    expect((await app.emails.findById(message.id))[1].starred).toBe(true)
  })
})
