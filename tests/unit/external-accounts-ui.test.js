import { describe, expect, it } from 'bun:test'
import { ExternalAccountsUiUtils } from '../../client/shared/utils/ExternalAccountsUiUtils.js'

const accountActions = (...args) => ExternalAccountsUiUtils.accountActions(...args)
const accountProgressLabel = (...args) => ExternalAccountsUiUtils.accountProgressLabel(...args)
const connectionDraft = (...args) => ExternalAccountsUiUtils.connectionDraft(...args)
const externalSenderIdentities = (...args) => ExternalAccountsUiUtils.externalSenderIdentities(...args)
const normalizeExternalAccounts = (...args) => ExternalAccountsUiUtils.normalizeExternalAccounts(...args)
const normalizeProviderCatalog = (...args) => ExternalAccountsUiUtils.normalizeProviderCatalog(...args)

describe('external account settings model', () => {
  it('keeps only supported, complete provider choices', () => {
    expect(normalizeProviderCatalog([
      { id: 'imap', name: 'IMAP / SMTP', provider: 'imap', authentication: 'password', guidance: 'none', configured: true, preset: null },
      { id: 'broken', name: '', provider: 'imap', authentication: 'password' },
      { id: 'future', name: 'Future', provider: 'future', authentication: 'password' },
    ])).toEqual([
      { id: 'imap', name: 'IMAP / SMTP', provider: 'imap', authentication: 'password', guidance: 'none', configured: true, preset: null },
    ])
  })

  it('builds an editable password connection without retaining a password', () => {
    expect(connectionDraft({
      choiceId: 'fastmail', source: 'preset',
      connection: {
        imap: { host: 'imap.fastmail.com', port: 993, security: 'tls', username: 'owner@example.com', allowInsecureTLS: false },
        smtp: { host: 'smtp.fastmail.com', port: 465, security: 'tls', username: 'owner@example.com', allowInsecureTLS: false },
        carddav: { url: 'https://carddav.fastmail.com/dav/', username: 'dav-owner' },
        caldav: { url: 'https://caldav.fastmail.com/dav/', username: 'calendar-owner' },
      },
    }, 'owner@example.com')).toEqual({
      choiceId: 'fastmail', source: 'preset', mailboxAddress: 'owner@example.com',
      label: 'owner@example.com',
      username: 'owner@example.com', imapHost: 'imap.fastmail.com', imapPort: '993', imapSecurity: 'tls',
      smtpHost: 'smtp.fastmail.com', smtpPort: '465', smtpSecurity: 'tls', containerFolder: '',
      cardDAVURL: 'https://carddav.fastmail.com/dav/', cardDAVUsername: 'dav-owner',
      calDAVURL: 'https://caldav.fastmail.com/dav/', calDAVUsername: 'calendar-owner',
    })
  })

  it('sorts redacted accounts and exposes state-safe actions and progress', () => {
    const accounts = normalizeExternalAccounts([
      { id: 'ext_bbbbbbbbbbbbbbbbbbbb', mailboxAddress: 'b@example.com', label: 'B', provider: 'imap', color: '#000000', order: 2, state: 'paused', revision: 3 },
      { id: 'ext_aaaaaaaaaaaaaaaaaaaa', mailboxAddress: 'a@example.com', label: 'A', provider: 'imap', color: '#ffffff', order: 1, state: 'syncing', revision: 7, progress: { phase: 'headers', completed: 25, total: 100 } },
      { id: 'bad', mailboxAddress: 'secret', credentials: { password: 'must-drop' } },
    ])
    expect(accounts.map(account => account.id)).toEqual(['ext_aaaaaaaaaaaaaaaaaaaa', 'ext_bbbbbbbbbbbbbbbbbbbb'])
    expect(accountActions(accounts[0])).toEqual({ canPause: true, canResume: false, canReconnect: false, canRetry: false, canRemove: true })
    expect(accountActions(accounts[1])).toEqual({ canPause: false, canResume: true, canReconnect: true, canRetry: false, canRemove: true })
    expect(accountProgressLabel(accounts[0])).toBe('Syncing headers · 25 of 100')
  })

  it('retains only redacted Microsoft delegation metadata and availability', () => {
    expect(normalizeProviderCatalog([
      { id: 'office365', name: 'Microsoft 365', provider: 'microsoft', authentication: 'oauth', configured: true },
    ])[0]).toMatchObject({ id: 'office365', configured: true })
    expect(normalizeExternalAccounts([{
      id: 'ext_mmmmmmmmmmmmmmmmmmmm', mailboxAddress: 'support@example.com', provider: 'microsoft',
      authIdentity: 'delegate@example.com', sharedMailboxAddress: 'support@example.com', label: 'Support',
      color: '#0078D4', order: 1, state: 'ready', revision: 2, credentials: { accessToken: 'must-drop' },
    }])[0]).toMatchObject({ authIdentity: 'delegate@example.com', sharedMailboxAddress: 'support@example.com' })
    expect(normalizeExternalAccounts([{
      id: 'ext_mmmmmmmmmmmmmmmmmmmm', mailboxAddress: 'support@example.com', provider: 'microsoft',
      label: 'Support', color: '#0078D4', order: 1, state: 'ready', revision: 2, credentials: { accessToken: 'must-drop' },
    }])[0]).not.toHaveProperty('credentials')
  })

  it('offers only unique, currently sendable external composer identities', () => {
    expect(externalSenderIdentities([
      { id: 'ext_aaaaaaaaaaaaaaaaaaaa', mailboxAddress: 'work@example.net', label: 'Work', provider: 'imap', color: '#336699', order: 1, state: 'idle', revision: 2, capabilities: { send: true } },
      { id: 'ext_bbbbbbbbbbbbbbbbbbbb', mailboxAddress: 'paused@example.net', label: 'Paused', provider: 'imap', color: '#000000', order: 2, state: 'paused', revision: 1, capabilities: { send: true } },
      { id: 'ext_cccccccccccccccccccc', mailboxAddress: 'readonly@example.net', label: 'Read only', provider: 'imap', color: '#000000', order: 3, state: 'idle', revision: 1, capabilities: { send: false } },
    ])).toEqual([{ address: 'work@example.net', label: 'Work', accountId: 'ext_aaaaaaaaaaaaaaaaaaaa', provider: 'imap', color: '#336699' }])
  })
})
