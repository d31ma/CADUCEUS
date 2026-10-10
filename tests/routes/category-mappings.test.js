import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

const Categories = Routes.CategoriesController
const Preferences = Routes.UsersMePreferencesController
const Inbox = Routes.InboxController
const Detail = Routes.InboxIdController
const Drafts = Routes.DraftsController
const Draft = Routes.DraftsIdController

const native = { inbox: 'inbox', sent: 'sent', drafts: 'drafts', spam: 'spam', archive: 'archive', trash: 'trash' }
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })
let app

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'bob@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.emails.put(email({ id: 'mapped-priority', ownerEmail: 'alice@example.com', folder: 'priority' }))
  await app.emails.put(email({ id: 'alice-native', ownerEmail: 'alice@example.com', folder: 'inbox' }))
  await app.emails.put(email({ id: 'bob-secret-category', ownerEmail: 'bob@example.com', recipient: 'bob@example.com', folder: 'bob-secret' }))
}, 300_000)
afterAll(teardownRoutes)

describe('exact-domain category role mappings', () => {
  it('lists native and discovered folders for only the exact mailbox owner', async () => {
    expect((await call(Categories, 'GET', req())).unauthorized).toBe(true)
    const result = await call(Categories, 'GET', req({ token: alice() }))
    expect(result.roles).toEqual(['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'])
    expect(result.accounts).toHaveLength(1)
    expect(result.accounts[0].mappings).toEqual(native)
    expect(result.accounts[0].capabilities).toEqual({ folders: true, labels: true, create: true })
    expect(result.accounts[0].categories.map(category => category.path)).toContain('priority')
    expect(result.accounts[0].categories.map(category => category.path)).not.toContain('bob-secret')
  })

  it('creates durable nested folders and labels for only the requested owned mailbox', async () => {
    const folder = await call(Categories, 'POST', req({
      token: alice(), body: { domain: 'example.com', kind: 'folder', path: 'Projects/Launch/Review' },
    }))
    expect(folder).toMatchObject({
      category: {
        path: 'projects/launch/review', kind: 'folder', depth: 2,
        parentPath: 'projects/launch', displayName: 'Review',
      },
    })

    const label = await call(Categories, 'POST', req({
      token: alice(), body: { domain: 'example.com', kind: 'label', path: 'Customers/Enterprise' },
    }))
    expect(label).toMatchObject({
      category: {
        path: 'customers/enterprise', kind: 'label', depth: 1,
        parentPath: 'customers', displayName: 'Enterprise',
      },
    })

    const replay = await call(Categories, 'POST', req({
      token: alice(), body: { domain: 'example.com', kind: 'label', path: 'customers/enterprise' },
    }))
    expect(replay.category).toEqual(label.category)

    const listed = await call(Categories, 'GET', req({ token: alice() }))
    expect(listed.accounts[0].categories).toEqual(expect.arrayContaining([folder.category, label.category]))
    expect((await call(Categories, 'POST', req({
      token: alice(), body: { domain: 'other.test', kind: 'label', path: 'secret' },
    }))).forbidden).toBe(true)
    expect((await call(Categories, 'POST', req({
      token: alice(), body: { domain: 'example.com', kind: 'label', path: '../secret' },
    }))).error).toContain('valid category path')
  })

  it('rejects foreign domains, incomplete roles, duplicate paths, and unsafe paths', async () => {
    const foreign = await call(Preferences, 'PUT', req({
      token: alice(),
      body: { categoryMappings: { 'other.com': native } },
    }))
    expect(foreign.forbidden).toBe(true)

    const incomplete = await call(Preferences, 'PUT', req({
      token: alice(), body: { categoryMappings: { 'example.com': { inbox: 'priority' } } },
    }))
    expect(incomplete.error).toContain('sent')

    const duplicate = await call(Preferences, 'PUT', req({
      token: alice(), body: { categoryMappings: { 'example.com': { ...native, inbox: 'priority', archive: 'priority' } } },
    }))
    expect(duplicate.error).toContain('different folder')

    const traversal = await call(Preferences, 'PUT', req({
      token: alice(), body: { categoryMappings: { 'example.com': { ...native, inbox: '../priority' } } },
    }))
    expect(traversal.error).toContain('valid folder path')
  })

  it('persists mappings and resolves logical list and move operations', async () => {
    const mappings = { 'example.com': { ...native, inbox: 'priority', archive: 'stored/archive' } }
    const saved = await call(Preferences, 'PUT', req({ token: alice(), body: { categoryMappings: mappings } }))
    expect(saved.categoryMappings).toEqual(mappings)

    const inbox = await call(Inbox, 'GET', req({ token: alice(), query: { folder: 'inbox' } }))
    expect(inbox.map(message => message.id)).toEqual(['mapped-priority'])
    expect(inbox[0]).toMatchObject({ folder: 'priority', systemFolder: 'inbox' })

    const moved = await call(Detail, 'PUT', req({
      token: alice(), paths: { id: 'mapped-priority' }, body: { folder: 'archive' },
    }))
    expect(moved).toMatchObject({ folder: 'stored/archive', systemFolder: 'archive' })
    expect((await app.emails.findById('mapped-priority'))[1].folder).toBe('stored/archive')
  })

  it('uses the mapped Drafts category while preserving exact-owner access', async () => {
    const mappings = { 'example.com': { ...native, drafts: 'working-drafts', inbox: 'priority', archive: 'stored/archive' } }
    await call(Preferences, 'PUT', req({ token: alice(), body: { categoryMappings: mappings } }))
    const created = await call(Drafts, 'POST', req({
      token: alice(),
      body: {
        clientId: 'category-draft-client', from: 'alice@example.com', replyTo: [],
        to: ['recipient@other.test'], cc: [], bcc: [], subject: 'Mapped draft',
        text: 'Draft body', html: '', attachments: [], references: [],
      },
    }))
    expect(created.folder).toBe('working-drafts')
    expect((await call(Draft, 'GET', req({ token: bob(), paths: { id: created.id } }))).notFound).toBe(true)
  })
})
