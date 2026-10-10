import { describe, expect, it } from 'bun:test'
import OfflineMailboxIndexService from '../../client/services/OfflineMailboxIndexService.js'
import { OfflineMailboxIndexUtils } from '../../client/shared/utils/OfflineMailboxIndexUtils.js'

const snapshots = new Map()
// The offline write vault is now an injected service; give the index an in-memory fake.
const offlineMailboxIndex = Object.assign(new OfflineMailboxIndexService(), {
  offlineWriteVault: {
    loadOfflineSnapshot: async (owner, name) => structuredClone(snapshots.get(`${owner}:${name}`) ?? null),
    saveOfflineSnapshot: async (owner, name, value) => { snapshots.set(`${owner}:${name}`, structuredClone(value)) },
    removeOfflineSnapshot: async (owner, name) => { snapshots.delete(`${owner}:${name}`) },
  },
})

const cacheOfflineAttachment = (...args) => offlineMailboxIndex.cacheOfflineAttachment(...args)
const cacheOfflineMailboxMessage = (...args) => offlineMailboxIndex.cacheOfflineMailboxMessage(...args)
const cacheOfflineMailboxSummaries = (...args) => offlineMailboxIndex.cacheOfflineMailboxSummaries(...args)
const hydrateOfflineMailbox = (...args) => offlineMailboxIndex.hydrateOfflineMailbox(...args)
const loadOfflineAttachment = (...args) => offlineMailboxIndex.loadOfflineAttachment(...args)
const loadOfflineMailboxMessage = (...args) => offlineMailboxIndex.loadOfflineMailboxMessage(...args)
const searchOfflineMailbox = (...args) => offlineMailboxIndex.searchOfflineMailbox(...args)
const normalizeOfflineMailboxIndex = (...args) => OfflineMailboxIndexUtils.normalizeOfflineMailboxIndex(...args)
const offlineMailboxSummary = (...args) => OfflineMailboxIndexUtils.offlineMailboxSummary(...args)
const searchOfflineMailboxIndex = (...args) => OfflineMailboxIndexUtils.searchOfflineMailboxIndex(...args)

function entry(overrides = {}) {
  const message = {
    id: overrides.id || 'mail-1',
    subject: overrides.subject || 'Quarterly planning',
    sender: overrides.sender || 'Alice <alice@example.net>',
    to: overrides.to || ['owner@example.test'],
    folder: overrides.folder || 'inbox',
    domain: overrides.domain || 'example.test',
    providerAccountId: overrides.providerAccountId || '',
    read: overrides.read ?? false,
    starred: overrides.starred ?? true,
    receivedAt: overrides.receivedAt || '2026-08-20T12:00:00.000Z',
    hasAttachments: overrides.hasAttachments ?? true,
  }
  return {
    id: message.id,
    message,
    searchText: `${message.subject} ${message.sender} ${overrides.body || 'confidential roadmap'}`.toLowerCase(),
    detailAvailable: true,
    attachmentIds: ['attachment-1'],
    receivedAt: message.receivedAt,
    updatedAt: Date.now(),
  }
}

describe('encrypted offline mailbox index', () => {
  it('keeps bodies and raw source out of list projections', () => {
    const summary = offlineMailboxSummary({
      id: 'mail-1', subject: 'Safe summary', snippet: 'bounded',
      body: 'full secret body', htmlBody: '<p>secret</p>', rawRFC822Base64: 'cmF3',
    })
    expect(summary).toEqual({ id: 'mail-1', subject: 'Safe summary', snippet: 'bounded' })
  })

  it('normalizes malformed state and caps the encrypted mirror', () => {
    expect(normalizeOfflineMailboxIndex(null).messages).toEqual([])
    const messages = Array.from({ length: 550 }, (_, index) => entry({ id: `mail-${index}` }))
    expect(normalizeOfflineMailboxIndex({ messages }).messages).toHaveLength(500)
  })

  it('searches full cached text with structured state, address, date, and mailbox filters', () => {
    const value = { messages: [
      entry(),
      entry({ id: 'mail-2', subject: 'Other', sender: 'Bob <bob@example.net>', folder: 'archive', read: true, starred: false, receivedAt: '2026-08-01T12:00:00.000Z', body: 'unrelated' }),
      entry({ id: 'mail-3', subject: 'Quarterly planning', providerAccountId: 'ext_AAAAAAAAAAAAAAAAAAAA' }),
    ] }
    expect(searchOfflineMailboxIndex(value, { query: 'roadmap from:alice is:unread has:attachment after:2026-08-10', folder: 'inbox', domain: 'example.test' }).map(item => item.id)).toEqual(['mail-1', 'mail-3'])
    expect(searchOfflineMailboxIndex(value, { query: 'subject:other before:2026-08-10 is:read', folder: 'archive' }).map(item => item.id)).toEqual(['mail-2'])
    expect(searchOfflineMailboxIndex(value, { account: 'ext_AAAAAAAAAAAAAAAAAAAA' }).map(item => item.id)).toEqual(['mail-3'])
  })

  it('persists one bounded detail and attachment behind an encrypted-snapshot boundary', async () => {
    snapshots.clear()
    const owner = 'owner@example.test'
    await cacheOfflineMailboxSummaries(owner, [entry().message], { replace: true })
    expect(await cacheOfflineMailboxMessage(owner, { ...entry().message, body: 'confidential roadmap', attachments: [{ id: 'attachment-1', size: 4 }] })).toBe(true)
    expect(await cacheOfflineAttachment(owner, 'mail-1', { id: 'attachment-1', filename: 'proof.txt', contentBase64: 'cHJvb2Y=' })).toBe(true)
    expect((await loadOfflineMailboxMessage(owner, 'mail-1')).body).toBe('confidential roadmap')
    expect((await loadOfflineAttachment(owner, 'mail-1', 'attachment-1')).filename).toBe('proof.txt')
    expect((await searchOfflineMailbox(owner, { query: 'roadmap' })).map(message => message.id)).toEqual(['mail-1'])
    expect(await loadOfflineMailboxMessage(owner, '../unsafe\n')).toBeNull()
  })

  it('hydrates missing details and bounded attachments from authenticated responses', async () => {
    snapshots.clear()
    const owner = 'owner@example.test'
    await cacheOfflineMailboxSummaries(owner, [entry().message], { replace: true })
    const paths = []
    const result = await hydrateOfflineMailbox(owner, async path => {
      paths.push(path)
      if (path.includes('/attachments/')) return Response.json({ id: 'attachment-1', filename: 'proof.txt', contentBase64: 'cHJvb2Y=' })
      return Response.json({ ...entry().message, body: 'hydrated body', attachments: [{ id: 'attachment-1', size: 5 }] })
    })
    expect(result).toEqual({ completed: 1, remaining: 0, attachmentBytes: 5 })
    expect(paths).toEqual(['/inbox/mail-1', '/inbox/mail-1/attachments/attachment-1'])
    expect((await loadOfflineMailboxMessage(owner, 'mail-1')).body).toBe('hydrated body')
  })
})
