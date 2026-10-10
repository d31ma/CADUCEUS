import { describe, expect, it } from 'bun:test'
import {
  createMessageId,
  normalizeMessageId,
  parseMessageIdList,
  threadMetadataFromHeaders,
  threadMetadataFromOutbound,
} from '@/shared/mailbox/threading.js'
import { groupMessagesIntoThreads } from '@/features/mailbox/repositories/ThreadsRepository.js'

describe('RFC message threading metadata', () => {
  it('normalizes one Message-ID and rejects unsafe or ambiguous values', () => {
    expect(normalizeMessageId(' root@example.test ')).toBe('<root@example.test>')
    expect(normalizeMessageId('<root@example.test>')).toBe('<root@example.test>')
    expect(normalizeMessageId('root@example.test\r\nBcc: victim@test')).toBeUndefined()
    expect(normalizeMessageId('<one@test> <two@test>')).toBeUndefined()
    expect(normalizeMessageId('not-an-id')).toBeUndefined()
  })

  it('extracts ordered unique references and caps pathological chains', () => {
    expect(parseMessageIdList('<root@test> <parent@test> <root@test>'))
      .toEqual(['<root@test>', '<parent@test>'])
    expect(parseMessageIdList(Array.from({ length: 120 }, (_, i) => `<m${i}@test>`))).toHaveLength(100)
  })

  it('derives a stable tenant-scoped thread identity from the reference root', () => {
    const root = threadMetadataFromHeaders({ 'message-id': '<root@outside.test>' }, 'fallback', 'example.com')
    const reply = threadMetadataFromHeaders({
      'message-id': '<reply@outside.test>',
      'in-reply-to': '<root@outside.test>',
      references: '<root@outside.test>',
    }, 'fallback', 'example.com')
    const otherTenant = threadMetadataFromHeaders({
      'message-id': '<reply@outside.test>', references: '<root@outside.test>',
    }, 'fallback', 'other.com')

    expect(reply.inReplyTo).toBe('<root@outside.test>')
    expect(reply.references).toEqual(['<root@outside.test>'])
    expect(reply.threadId).toBe(root.threadId)
    expect(otherTenant.threadId).not.toBe(root.threadId)
  })

  it('creates outbound IDs and extends a missing parent into the lineage', () => {
    const messageId = createMessageId('Example.COM')
    expect(messageId).toMatch(/^<[a-f0-9.]+@example\.com>$/)
    const metadata = threadMetadataFromOutbound({
      messageId,
      inReplyTo: '<parent@outside.test>',
      references: ['<root@outside.test>'],
    }, 'fallback', 'example.com')
    expect(metadata.references).toEqual(['<root@outside.test>', '<parent@outside.test>'])
  })
})

describe('unified mailbox thread grouping', () => {
  const external = (overrides = {}) => ({
    id: 'external-1',
    external: true,
    providerAccountId: 'ext_aaaaaaaaaaaaaaaaaaaa',
    mailboxAddress: 'alice@remote.example',
    domain: 'remote.example',
    sender: 'sender@example.net',
    recipient: 'alice@remote.example',
    to: ['alice@remote.example'],
    cc: [],
    bcc: [],
    subject: 'Provider conversation',
    rfcMessageId: '<shared@example.net>',
    inReplyTo: '',
    references: [],
    receivedAt: '2026-01-01T10:00:00.000Z',
    folder: 'inbox',
    ...overrides,
  })

  it('connects RFC references within one provider account', () => {
    const threads = groupMessagesIntoThreads([
      external(),
      external({
        id: 'external-2',
        rfcMessageId: '<reply@example.net>',
        inReplyTo: '<shared@example.net>',
        references: ['<shared@example.net>'],
        subject: 'Re: Provider conversation',
        receivedAt: '2026-01-01T11:00:00.000Z',
      }),
    ], 'alice@example.com')

    expect(threads).toHaveLength(1)
    expect(threads[0].emails.map(message => message.id)).toEqual(['external-1', 'external-2'])
    expect(threads[0].personalLevel).toBe(2)
  })

  it('never merges equal RFC identities from separate provider accounts on the same domain', () => {
    const threads = groupMessagesIntoThreads([
      external(),
      external({
        id: 'external-other-account',
        providerAccountId: 'ext_bbbbbbbbbbbbbbbbbbbb',
        receivedAt: '2026-01-01T11:00:00.000Z',
      }),
    ], 'alice@remote.example')

    expect(threads).toHaveLength(2)
    expect(threads.map(thread => thread.emails.map(message => message.id))).toEqual([
      ['external-other-account'],
      ['external-1'],
    ])
  })
})
