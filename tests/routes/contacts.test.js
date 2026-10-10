import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { ContactsController as Contacts } from '@/controllers/ContactsController.js'
import { ContactsIdController as Contact } from '@/controllers/ContactsIdController.js'
import { ContactsGroupsController as Groups } from '@/controllers/ContactsGroupsController.js'
import { ContactsGroupsIdController as Group } from '@/controllers/ContactsGroupsIdController.js'
import { ContactsImportController as ImportContacts } from '@/controllers/ContactsImportController.js'
import { ContactsExportController as ExportContacts } from '@/controllers/ContactsExportController.js'
import { ContactsSuggestionsController as Suggestions } from '@/controllers/ContactsSuggestionsController.js'
import { SendController as Send } from '@/controllers/SendController.js'
import { call } from '../support/controller-call.mjs'

let app
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(async () => { app = await setupRoutes() }, 300_000)
afterAll(teardownRoutes)

describe('mailbox-owned contacts', () => {
  let groupId
  let contactId

  it('creates groups and structured personal contacts', async () => {
    const group = await call(Groups, 'POST', req({ token: alice(), body: { name: 'Vendors' } }))
    groupId = group.id
    expect(groupId).toMatch(/^group-/)
    expect((await call(Groups, 'POST', req({ token: alice(), body: { name: 'vendors' } }))).error).toMatch(/exists/)

    const contact = await call(Contacts, 'POST', req({ token: alice(), body: {
      name: 'Grace Hopper', givenName: 'Grace', familyName: 'Hopper', company: 'Navy',
      emails: [{ value: 'GRACE@EXAMPLE.NET', type: 'work' }],
      phones: [{ value: '+1 555 0110', type: 'cell' }],
      addresses: [{ city: 'Arlington', country: 'US', type: 'work' }],
      urls: [{ value: 'https://example.net/' }], nicknames: ['Amazing Grace'],
      birthday: '1906-12-09', notes: 'Compiler pioneer', groupIds: [groupId],
    } }))
    contactId = contact.id
    expect(contact).toMatchObject({ source: 'manual', emails: [{ value: 'grace@example.net', type: 'work' }] })
    expect((await call(Contacts, 'GET', req({ token: alice(), query: { q: 'compiler' } })))[0].id).toBe(contactId)
    expect((await call(Contacts, 'GET', req({ token: alice(), query: { groupId } })))[0].id).toBe(contactId)
  })

  it('never exposes a personal contact to another user in the same domain', async () => {
    expect(await call(Contacts, 'GET', req({ token: bob() }))).toEqual([])
    expect((await call(Contact, 'GET', req({ token: bob(), paths: { id: contactId } }))).notFound).toBe(true)
    expect((await call(Contact, 'PUT', req({ token: bob(), paths: { id: contactId }, body: { name: 'stolen' } }))).notFound).toBe(true)
    expect((await call(Contact, 'DELETE', req({ token: bob(), paths: { id: contactId } }))).notFound).toBe(true)
  })

  it('updates contacts and removes deleted group membership', async () => {
    const updated = await call(Contact, 'PUT', req({ token: alice(), paths: { id: contactId }, body: { title: 'Rear Admiral' } }))
    expect(updated.title).toBe('Rear Admiral')
    expect(updated.source).toBe('manual')
    expect((await call(Group, 'PUT', req({ token: alice(), paths: { id: groupId }, body: { name: 'Partners' } }))).name).toBe('Partners')
    expect((await call(Group, 'DELETE', req({ token: alice(), paths: { id: groupId } }))).deleted).toBe(true)
    expect((await call(Contact, 'GET', req({ token: alice(), paths: { id: contactId } }))).groupIds).toEqual([])
  })

  it('imports and exports multi-contact vCard files without crossing owners', async () => {
    const content = [
      'BEGIN:VCARD', 'VERSION:4.0', 'FN:Ada Lovelace', 'N:Lovelace;Ada;;;', 'EMAIL:ada@example.net', 'END:VCARD',
      'BEGIN:VCARD', 'VERSION:3.0', 'FN:Alan Turing', 'EMAIL;TYPE=work:alan@example.net', 'END:VCARD',
    ].join('\r\n')
    const imported = await call(ImportContacts, 'POST', req({ token: alice(), body: { content } }))
    expect(imported.imported).toBe(2)
    expect(imported.skipped).toBe(0)

    const exported = await call(ExportContacts, 'POST', req({ token: alice(), body: { ids: imported.ids } }))
    expect(exported.count).toBe(2)
    const decoded = Buffer.from(exported.contentBase64, 'base64').toString('utf8')
    expect(decoded.match(/BEGIN:VCARD/g)).toHaveLength(2)
    expect(decoded).toContain('FN:Ada Lovelace')
    expect((await call(Contacts, 'GET', req({ token: alice(), query: { source: 'vcard' } })))).toHaveLength(2)
    expect((await call(ExportContacts, 'POST', req({ token: bob(), body: { ids: imported.ids } }))).count).toBe(0)
  })

  it('learns found-in-mail recipients from accepted sends and ranks suggestions', async () => {
    const sent = await call(Send, 'POST', req({ token: alice(), body: {
      to: ['new.recipient@example.net'], subject: 'Introductions', text: 'Hello',
    } }))
    expect(sent.sent ?? sent.messageId ?? sent.id).toBeTruthy()
    const found = await call(Contacts, 'GET', req({ token: alice(), query: { source: 'mail', q: 'new.recipient' } }))
    expect(found).toHaveLength(1)
    expect(found[0]).toMatchObject({ source: 'mail', interactionCount: 1 })
    const suggestions = await call(Suggestions, 'GET', req({ token: alice(), query: { q: 'recipient' } }))
    expect(suggestions[0]).toMatchObject({ email: 'new.recipient@example.net', source: 'mail' })
  })

  it('does not learn found-in-mail contacts for a disabled hosted domain', async () => {
    await app.users.put({
      email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin',
      mailPreferences: { findInMailDisabledDomains: ['example.com'] },
    })
    await app.contacts.observeParticipants('alice@example.com', ['not-learned@example.net'], 'example.com')
    expect(await call(Contacts, 'GET', req({ token: alice(), query: { source: 'mail', q: 'not-learned' } }))).toEqual([])
  })

  it('deletes only the owning contact', async () => {
    expect((await call(Contact, 'DELETE', req({ token: alice(), paths: { id: contactId } }))).deleted).toBe(true)
    expect((await call(Contact, 'GET', req({ token: alice(), paths: { id: contactId } }))).notFound).toBe(true)
    expect((await call(Contacts, 'POST', req({ token: alice(), body: { emails: [{ value: 'bad' }] } }))).error).toMatch(/invalid contact email/)
    expect((await call(Contacts, 'POST', req({ token: alice(), body: { name: 'Unsafe', urls: [{ value: 'javascript:alert(1)' }] } }))).error).toMatch(/URL/)
    expect(app.contactRecords).toBeTruthy()
  })
})
