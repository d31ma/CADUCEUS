import { describe, expect, it } from 'bun:test'
import { canAccessEmail } from '@/features/mailbox/repositories/EmailsRepository.js'

const message = over => ({
  id: 'message-1',
  domain: 'example.com',
  recipient: 'alice@example.com',
  sender: 'sender@outside.test',
  subject: 'Private',
  body: 'body',
  folder: 'inbox',
  read: false,
  starred: false,
  receivedAt: '2026-08-14T12:00:00.000Z',
  processed: true,
  ...over,
})

describe('email mailbox ownership', () => {
  it('treats an explicit owner as authoritative within a shared domain', () => {
    const email = message({ ownerEmail: 'alice@example.com', to: ['bob@example.com'] })
    expect(canAccessEmail(email, 'alice@example.com')).toBe(true)
    expect(canAccessEmail(email, 'bob@example.com')).toBe(false)
  })

  it('supports narrow exact-address fallbacks for historical rows', () => {
    expect(canAccessEmail(message({ ownerEmail: undefined }), 'alice@example.com')).toBe(true)
    expect(canAccessEmail(message({ ownerEmail: undefined }), 'bob@example.com')).toBe(false)
    expect(canAccessEmail(message({
      ownerEmail: undefined,
      folder: 'sent',
      sender: 'alice@example.com',
      recipient: 'bob@example.com',
    }), 'alice@example.com')).toBe(true)
    expect(canAccessEmail(message({
      ownerEmail: undefined,
      folder: 'sent',
      sender: 'alice@example.com',
      recipient: 'bob@example.com',
    }), 'bob@example.com')).toBe(false)
  })

  it('fails closed for malformed or ambiguous ownership', () => {
    expect(canAccessEmail(message({ ownerEmail: undefined, recipient: 'not-an-address' }), 'alice@example.com')).toBe(false)
    expect(canAccessEmail(message({ ownerEmail: 'alice@example.com, bob@example.com' }), 'alice@example.com')).toBe(false)
    expect(canAccessEmail(message({ ownerEmail: 'alice@example.com' }), '')).toBe(false)
  })
})
