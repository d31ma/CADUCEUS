import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

const Drafts = Routes.DraftsController
const Draft = Routes.DraftsIdController
const Inbox = Routes.InboxController

let app
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })
const base = (over = {}) => ({
  clientId: 'client-draft-1',
  to: ['recipient@other.test'],
  cc: [], bcc: [], replyTo: [],
  subject: 'Durable draft',
  text: 'first body',
  html: '<p>first body</p>',
  references: [],
  attachments: [],
  ...over,
})

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: ['sales@example.com'], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('durable drafts API', () => {
  let draftId

  it('validates and creates a versioned draft idempotently', async () => {
    expect((await call(Drafts, 'POST', req({ token: alice(), body: {} }))).error).toMatch(/clientId/)
    const created = await call(Drafts, 'POST', req({ token: alice(), body: base() }))
    draftId = created.id
    expect(draftId).toMatch(/^draft-/)
    expect(created.draftVersion).toBe(1)
    expect(created.draftOwner).toBe('alice@example.com')
    expect(created.recovered).toBe(false)

    const retried = await call(Drafts, 'POST', req({ token: alice(), body: base({ text: 'response was lost' }) }))
    expect(retried.id).toBe(draftId)
    expect(retried.draftVersion).toBe(1)
    expect(retried.recovered).toBe(true)
    expect((await app.emails.list(['example.com'], { folder: 'drafts' }))).toHaveLength(1)
  })

  it('keeps draft bodies private from another user in the same domain', async () => {
    expect((await call(Draft, 'GET', req({ token: bob(), paths: { id: draftId } }))).notFound).toBe(true)
    const list = await call(Inbox, 'GET', req({ token: bob(), query: { folder: 'drafts' } }))
    expect(list.some(item => item.id === draftId)).toBe(false)
  })

  it('persists an exact owned From alias and rejects same-domain impersonation', async () => {
    const denied = await call(Drafts, 'POST', req({ token: alice(), body: base({ clientId: 'foreign-from', from: 'bob@example.com' }) }))
    expect(denied.forbidden).toBe(true)
    const aliasDraft = await call(Drafts, 'POST', req({ token: alice(), body: base({ clientId: 'alias-from', from: 'sales@example.com' }) }))
    expect(aliasDraft.sender).toBe('sales@example.com')
    expect((await call(Draft, 'DELETE', req({ token: alice(), paths: { id: aliasDraft.id } }))).discarded).toBe(aliasDraft.id)
  })

  it('compare-and-swaps content and the complete attachment set', async () => {
    const updated = await call(Draft, 'PUT', req({
      token: alice(), paths: { id: draftId },
      body: {
        ...base({
          text: 'second body', html: '<p>second body</p>',
          inReplyTo: '<parent@other.test>', references: ['<root@other.test>'], sourceEmailId: 'source-message-1',
          attachments: [{
            filename: 'notes.txt', contentType: 'text/plain',
            contentBase64: Buffer.from('draft attachment').toString('base64'),
          }, {
            filename: 'inline.png', contentType: 'image/png', disposition: 'inline', contentId: 'draft-logo@example.test',
            contentBase64: Buffer.from('inline image').toString('base64'),
          }],
        }),
        version: 1,
      },
    }))
    expect(updated.draftVersion).toBe(2)
    expect(updated.body).toBe('second body')
    expect(updated.inReplyTo).toBe('<parent@other.test>')
    expect(updated.sourceEmailId).toBe('source-message-1')
    expect(updated.attachments).toHaveLength(2)
    expect(Buffer.from(updated.attachments[0].contentBase64, 'base64').toString()).toBe('draft attachment')
    expect(updated.attachments[1]).toMatchObject({ disposition: 'inline', contentId: 'draft-logo@example.test' })

    const stale = await call(Draft, 'PUT', req({
      token: alice(), paths: { id: draftId }, body: { ...base(), version: 1 },
    }))
    expect(stale.conflict).toBe(true)
    expect(stale.currentVersion).toBe(2)
  })

  it('loads the latest version and permanently discards its files', async () => {
    const loaded = await call(Draft, 'GET', req({ token: alice(), paths: { id: draftId } }))
    expect(loaded.draftVersion).toBe(2)
    expect(loaded.attachments).toHaveLength(2)
    expect(loaded.attachments[1]).toMatchObject({ disposition: 'inline', contentId: 'draft-logo@example.test' })
    expect((await call(Draft, 'DELETE', req({ token: bob(), paths: { id: draftId } }))).notFound).toBe(true)
    expect((await call(Draft, 'DELETE', req({ token: alice(), paths: { id: draftId } }))).discarded).toBe(draftId)
    expect((await call(Draft, 'GET', req({ token: alice(), paths: { id: draftId } }))).notFound).toBe(true)
    expect(await app.attachments.list(draftId)).toEqual([])
  })
})
