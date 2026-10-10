import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import {
  FyloAccessContext,
  posixId,
  recordDomain,
} from '../../server/platform/persistence/fylo-access.js'

let previousEnabled

beforeEach(() => {
  previousEnabled = process.env.FYLO_POSIX_ACCESS
  process.env.FYLO_POSIX_ACCESS = 'true'
})

afterEach(() => {
  if (previousEnabled === undefined) delete process.env.FYLO_POSIX_ACCESS
  else process.env.FYLO_POSIX_ACCESS = previousEnabled
})

describe('FyloAccessContext', () => {
  test('derives stable namespace-separated POSIX identities', () => {
    expect(posixId('mailbox-user', 'alice@example.com')).toBe(
      posixId('mailbox-user', 'alice@example.com'),
    )
    expect(posixId('mailbox-user', 'alice@example.com')).not.toBe(
      posixId('mail-domain', 'alice@example.com'),
    )
  })

  test('provides trusted read groups and 0660 ownership for an allowed domain', async () => {
    await FyloAccessContext.run({
      email: 'Alice@Example.com',
      domains: ['EXAMPLE.com', 'work.example.com'],
    }, async () => {
      const read = FyloAccessContext.read()
      expect(read).toEqual({
        uid: posixId('mailbox-user', 'alice@example.com'),
        groups: [
          posixId('mail-domain', 'example.com'),
          posixId('mail-domain', 'work.example.com'),
        ],
      })
      expect(FyloAccessContext.write({ domain: 'example.com' })).toEqual({
        gid: posixId('mail-domain', 'example.com'),
        mode: 0o660,
      })
    })
  })

  test('fails closed when a write targets a domain outside the caller groups', async () => {
    await FyloAccessContext.run({
      email: 'alice@example.com',
      domains: ['example.com'],
    }, () => {
      expect(() => FyloAccessContext.write({ recipient: 'bob@other.com' })).toThrow(
        'Storage access denied for domain other.com',
      )
    })
  })

  test('extracts tenant domains only from recognized record shapes', () => {
    expect(recordDomain({ meta: { domain: 'Example.com' } })).toBe('example.com')
    expect(recordDomain({ sender: 'alice@example.com' })).toBe('example.com')
    expect(recordDomain({ address: 'external@example.net' })).toBeNull()
  })
})
