import { afterEach, beforeEach, describe, expect, it } from 'bun:test'
import InboxService from '../../client/services/InboxService.js'

const originalSessionStorage = globalThis.sessionStorage

/** The inbox view is now InboxService; the old `window._caduceus` globals come from its injected shell. */
function inboxView() {
  return Object.assign(new InboxService(), {
    shell: {
      auth: { domains: ['example.com'], externalAccounts: [], externalFolders: {} },
      mailboxLabel: domain => domain,
    },
  })
}

beforeEach(() => {
  globalThis.sessionStorage = { getItem: () => null }
})

afterEach(() => {
  globalThis.sessionStorage = originalSessionStorage
})

describe('mailbox account scope', () => {
  const accountId = 'ext_aaaaaaaaaaaaaaaaaaaa'

  it('keeps a connected account distinct from hosted mail on the same domain', () => {
    const view = inboxView()
    view.allEmails = [
      { id: 'hosted', domain: 'example.com' },
      { id: 'external-a', domain: 'example.com', providerAccountId: accountId },
      { id: 'external-b', domain: 'example.com', providerAccountId: 'ext_bbbbbbbbbbbbbbbbbbbb' },
    ]

    view.selectedDomain = 'example.com'
    expect(view.domainEmails.map(message => message.id)).toEqual(['hosted'])

    view.selectedDomain = 'all'
    view.selectedAccount = accountId
    expect(view.domainEmails.map(message => message.id)).toEqual(['external-a'])
  })

  it('emits mutually exclusive backend query scopes', () => {
    const view = inboxView()
    const accountParams = new URLSearchParams()
    view.selectedAccount = accountId
    view.selectedDomain = 'all'
    view.appendMailboxScope(accountParams)
    expect(accountParams.toString()).toBe(`account=${accountId}`)

    const domainParams = new URLSearchParams()
    view.selectedAccount = ''
    view.selectedDomain = 'example.com'
    view.appendMailboxScope(domainParams)
    expect(domainParams.toString()).toBe('domain=example.com')
  })

  it('presents hosted and provider identities as separate selector options', () => {
    const view = inboxView()
    view.externalAccounts = [{ id: accountId, label: 'Remote work', mailboxAddress: 'alice@example.com' }]

    expect(view.mailboxOptions).toEqual([
      expect.objectContaining({ value: 'domain:example.com', kind: 'domain' }),
      expect.objectContaining({ value: `account:${accountId}`, kind: 'account', label: 'Remote work' }),
    ])
  })
})
