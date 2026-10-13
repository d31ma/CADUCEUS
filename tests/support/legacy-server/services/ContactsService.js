import { Security } from '../shared/security/Security.js'
import { VCard } from '../shared/contacts/VCard.js'
import { ContactsRules } from '../shared/contacts/ContactsRules.js'

/** Contact lifecycle, found-in-mail learning, groups, and vCard interchange. */
@Service
export default class ContactsService {
  /** @type {ContactsHelper} */
  @Needs
  contactsHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  async list(ownerEmail, filters = {}, providerContacts = []) {
    let contacts = [
      ...await this.contactsHelper.list(ownerEmail),
      ...(Array.isArray(providerContacts) ? providerContacts.map(contact => ContactsRules.normalizeProviderContact(ownerEmail, contact)) : []),
    ]
    if (filters.source) contacts = contacts.filter(contact => contact.source === filters.source)
    if (filters.groupId) contacts = contacts.filter(contact => contact.groupIds.includes(filters.groupId))
    if (filters.q) {
      const needle = String(filters.q).trim().toLowerCase()
      contacts = contacts.filter(contact => ContactsRules.searchable(contact).includes(needle))
    }
    return contacts.map(ContactsRules.stripDocId).sort((a, b) => (a.name || a.emails[0]?.value || '').localeCompare(b.name || b.emails[0]?.value || ''))
  }

  async get(id, ownerEmail) {
    const [, contact] = await this.contactsHelper.findById(id, ownerEmail)
    return contact
  }

  async create(ownerEmail, input, source = 'manual') {
    const now = new Date().toISOString()
    const contact = ContactsRules.normalizeContact(ownerEmail, input, {
      id: `contact-${crypto.randomUUID()}`, source, createdAt: now, updatedAt: now,
      ...(source === 'mail' ? {
        interactionCount: Number.isInteger(input.interactionCount) ? input.interactionCount : 1,
        lastContactedAt: input.lastContactedAt || now,
      } : {}),
    })
    await this.assertGroups(ownerEmail, contact.groupIds)
    await this.contactsHelper.put(contact)
    return contact
  }

  async update(id, ownerEmail, input) {
    const [docId, current] = await this.contactsHelper.findById(id, ownerEmail)
    if (!docId || !current) return null
    const next = ContactsRules.normalizeContact(ownerEmail, { ...current, ...input }, {
      id: current.id, source: current.source, createdAt: current.createdAt,
      updatedAt: new Date().toISOString(), interactionCount: current.interactionCount,
      lastContactedAt: current.lastContactedAt,
    })
    await this.assertGroups(ownerEmail, next.groupIds)
    await this.contactsHelper.replace(docId, next)
    return next
  }

  async delete(id, ownerEmail) {
    const [docId, contact] = await this.contactsHelper.findById(id, ownerEmail)
    if (!docId || !contact) return false
    await this.contactsHelper.delete(docId, contact.domain)
    return true
  }

  async suggestions(ownerEmail, query, providerContacts = []) {
    const contacts = await this.list(ownerEmail, { q: query }, providerContacts)
    return contacts.flatMap(contact => contact.emails.map(email => ({
      contactId: contact.id, name: contact.name, email: email.value, source: contact.source,
      lastContactedAt: contact.lastContactedAt || '', interactionCount: contact.interactionCount || 0,
    }))).sort((a, b) => b.interactionCount - a.interactionCount || b.lastContactedAt.localeCompare(a.lastContactedAt)).slice(0, 20)
  }

  async observeParticipants(ownerEmail, addresses, mailboxDomain = '') {
    const domain = Security.normalizeDomain(mailboxDomain || ownerEmail.split('@')[1])
    if (this.usersHelper && domain) {
      const [, owner] = await this.usersHelper.findByEmail(ownerEmail)
      if (owner?.mailPreferences?.findInMailDisabledDomains?.includes(domain)) return
    }
    for (const raw of [...new Set(addresses || [])]) {
      const email = Security.normalizeEmailAddress(raw)
      if (!email || email === ownerEmail.toLowerCase()) continue
      try {
        const existing = await this.contactsHelper.findByEmail(ownerEmail, email)
        const now = new Date().toISOString()
        if (existing) {
          await this.contactsHelper.replace(existing.docId, {
            ...ContactsRules.stripDocId(existing), interactionCount: Number(existing.interactionCount || 0) + 1,
            lastContactedAt: now, updatedAt: now,
          })
        } else {
          await this.create(ownerEmail, {
            name: '', emails: [{ value: email }], phones: [], addresses: [], urls: [], nicknames: [], groupIds: [],
            interactionCount: 1, lastContactedAt: now,
          }, 'mail')
        }
      } catch (error) {
        console.warn('[contacts] found-in-mail update failed:', error instanceof Error ? error.message : error)
      }
    }
  }

  async importVCard(ownerEmail, content, groupIds = []) {
    await this.assertGroups(ownerEmail, groupIds)
    const parsed = VCard.parseFile(content)
    const ids = []
    let skipped = 0
    for (const item of parsed) {
      try { ids.push((await this.create(ownerEmail, { ...item, groupIds }, 'vcard')).id) } catch { skipped++ }
    }
    return { imported: ids.length, skipped, ids }
  }

  async exportVCard(ownerEmail, ids = []) {
    const all = await this.list(ownerEmail)
    const allowed = new Set(ids)
    const contacts = ids.length ? all.filter(contact => allowed.has(contact.id)) : all
    return { content: VCard.serializeContacts(contacts), count: contacts.length }
  }

  async listGroups(ownerEmail) {
    return (await this.contactsHelper.listGroups(ownerEmail)).map(ContactsRules.stripDocId).sort((a, b) => a.name.localeCompare(b.name))
  }

  async createGroup(ownerEmail, name) {
    const normalizedName = ContactsRules.validateText(name, 'name', 120, true)
    const groups = await this.listGroups(ownerEmail)
    if (groups.some(group => group.name.toLowerCase() === normalizedName.toLowerCase())) throw new Error('group name already exists')
    const now = new Date().toISOString()
    const group = { id: `group-${crypto.randomUUID()}`, ownerEmail: ownerEmail.toLowerCase(), domain: ContactsRules.ownerDomain(ownerEmail), name: normalizedName, createdAt: now, updatedAt: now }
    await this.contactsHelper.putGroup(group)
    return group
  }

  async updateGroup(id, ownerEmail, name) {
    const [docId, current] = await this.contactsHelper.findGroupById(id, ownerEmail)
    if (!docId || !current) return null
    const next = { ...current, name: ContactsRules.validateText(name, 'name', 120, true), updatedAt: new Date().toISOString() }
    await this.contactsHelper.replaceGroup(docId, next)
    return next
  }

  async deleteGroup(id, ownerEmail) {
    const [docId, group] = await this.contactsHelper.findGroupById(id, ownerEmail)
    if (!docId || !group) return false
    await this.contactsHelper.deleteGroup(docId, group)
    return true
  }

  async assertGroups(ownerEmail, groupIds) {
    if (!groupIds.length) return
    const owned = new Set((await this.listGroups(ownerEmail)).map(group => group.id))
    if (groupIds.some(id => !owned.has(id))) throw new Error('contact group not found')
  }
}

