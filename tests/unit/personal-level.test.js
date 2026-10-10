import { describe, expect, it } from 'bun:test'
import {
  messageParticipants,
  parseAddressList,
  personalLevelForThread,
  recipientsFromHeaders,
} from '@/shared/mailbox/personal-level.js'

const message = (overrides = {}) => ({
  id: 'message-1',
  domain: 'example.com',
  recipient: 'alice@example.com',
  sender: 'sender@outside.test',
  subject: 'Hello',
  body: 'Hello',
  folder: 'inbox',
  read: false,
  starred: false,
  receivedAt: '2026-08-14T12:00:00.000Z',
  processed: true,
  ...overrides,
})

describe('personal level indicators', () => {
  it('normalizes RFC display names and repeated addresses', () => {
    expect(parseAddressList('Alice <ALICE@example.com>, "Alice again" <alice@example.com>, Bob <bob@example.net>'))
      .toEqual(['alice@example.com', 'bob@example.net'])
  })

  it('preserves visible recipients and only falls back to the envelope when headers are absent', () => {
    expect(recipientsFromHeaders({
      to: 'Team List <staff@example.net>',
      cc: 'Observer <observer@example.org>',
    }, 'alice@example.com')).toEqual({
      to: ['staff@example.net'],
      cc: ['observer@example.org'],
      bcc: [],
    })
    expect(recipientsFromHeaders({}, 'ALICE@example.com')).toEqual({
      to: ['alice@example.com'], cc: [], bcc: [],
    })
  })

  it('uses explicit recipient arrays instead of an envelope recipient for list mail', () => {
    expect(messageParticipants(message({
      recipient: 'alice@example.com',
      to: ['staff@example.net'],
    }))).toEqual(['sender@outside.test', 'staff@example.net'])
  })

  it('returns level two for a two-person conversation', () => {
    expect(personalLevelForThread([
      message({ to: ['alice@example.com'] }),
      message({ sender: 'alice@example.com', to: ['sender@outside.test'] }),
    ], 'ALICE@example.com')).toBe(2)
  })

  it('returns level one when the mailbox is one of several participants', () => {
    expect(personalLevelForThread([
      message({ to: ['alice@example.com', 'bob@example.net'] }),
    ], 'alice@example.com')).toBe(1)
  })

  it('returns no indicator for mailing-list delivery where the mailbox is not a visible participant', () => {
    expect(personalLevelForThread([
      message({ recipient: 'alice@example.com', to: ['staff@example.net'] }),
    ], 'alice@example.com')).toBe(0)
  })
})
