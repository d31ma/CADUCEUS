import { randomUUID } from 'node:crypto'
import { Security } from '../../../platform/security/Security.js'
import { VCard } from './VCard.js'

const MAX_VALUES = 50

/** Contact lifecycle, found-in-mail learning, groups, and vCard interchange. */
export class ContactsService {
  /**
   * @param {import('../repositories/ContactsRepository.js').ContactsRepository} repository
   * @param {import('../../users/repositories/UsersRepository.js').UsersRepository} [users]
   */
  constructor(repository, users) { this.repository = repository; this.users = users }

  async list(ownerEmail, filters = {}, providerContacts = []) {
    let contacts = [
      ...await this.repository.list(ownerEmail),
      ...(Array.isArray(providerContacts) ? providerContacts.map(contact => normalizeProviderContact(ownerEmail, contact)) : []),
    ]
    if (filters.source) contacts = contacts.filter(contact => contact.source === filters.source)
    if (filters.groupId) contacts = contacts.filter(contact => contact.groupIds.includes(filters.groupId))
    if (filters.q) {
      const needle = String(filters.q).trim().toLowerCase()
      contacts = contacts.filter(contact => searchable(contact).includes(needle))
    }
    return contacts.map(stripDocId).sort((a, b) => (a.name || a.emails[0]?.value || '').localeCompare(b.name || b.emails[0]?.value || ''))
  }

  async get(id, ownerEmail) {
    const [, contact] = await this.repository.findById(id, ownerEmail)
    return contact
  }

  async create(ownerEmail, input, source = 'manual') {
    const now = new Date().toISOString()
    const contact = normalizeContact(ownerEmail, input, {
      id: `contact-${randomUUID()}`, source, createdAt: now, updatedAt: now,
      ...(source === 'mail' ? {
        interactionCount: Number.isInteger(input.interactionCount) ? input.interactionCount : 1,
        lastContactedAt: input.lastContactedAt || now,
      } : {}),
    })
    await this.assertGroups(ownerEmail, contact.groupIds)
    await this.repository.put(contact)
    return contact
  }

  async update(id, ownerEmail, input) {
    const [docId, current] = await this.repository.findById(id, ownerEmail)
    if (!docId || !current) return null
    const next = normalizeContact(ownerEmail, { ...current, ...input }, {
      id: current.id, source: current.source, createdAt: current.createdAt,
      updatedAt: new Date().toISOString(), interactionCount: current.interactionCount,
      lastContactedAt: current.lastContactedAt,
    })
    await this.assertGroups(ownerEmail, next.groupIds)
    await this.repository.replace(docId, next)
    return next
  }

  async delete(id, ownerEmail) {
    const [docId, contact] = await this.repository.findById(id, ownerEmail)
    if (!docId || !contact) return false
    await this.repository.delete(docId, contact.domain)
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
    if (this.users && domain) {
      const [, owner] = await this.users.findByEmail(ownerEmail)
      if (owner?.mailPreferences?.findInMailDisabledDomains?.includes(domain)) return
    }
    for (const raw of [...new Set(addresses || [])]) {
      const email = Security.normalizeEmailAddress(raw)
      if (!email || email === ownerEmail.toLowerCase()) continue
      try {
        const existing = await this.repository.findByEmail(ownerEmail, email)
        const now = new Date().toISOString()
        if (existing) {
          await this.repository.replace(existing.docId, {
            ...stripDocId(existing), interactionCount: Number(existing.interactionCount || 0) + 1,
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
    return (await this.repository.listGroups(ownerEmail)).map(stripDocId).sort((a, b) => a.name.localeCompare(b.name))
  }

  async createGroup(ownerEmail, name) {
    const normalizedName = validateText(name, 'name', 120, true)
    const groups = await this.listGroups(ownerEmail)
    if (groups.some(group => group.name.toLowerCase() === normalizedName.toLowerCase())) throw new Error('group name already exists')
    const now = new Date().toISOString()
    const group = { id: `group-${randomUUID()}`, ownerEmail: ownerEmail.toLowerCase(), domain: ownerDomain(ownerEmail), name: normalizedName, createdAt: now, updatedAt: now }
    await this.repository.putGroup(group)
    return group
  }

  async updateGroup(id, ownerEmail, name) {
    const [docId, current] = await this.repository.findGroupById(id, ownerEmail)
    if (!docId || !current) return null
    const next = { ...current, name: validateText(name, 'name', 120, true), updatedAt: new Date().toISOString() }
    await this.repository.replaceGroup(docId, next)
    return next
  }

  async deleteGroup(id, ownerEmail) {
    const [docId, group] = await this.repository.findGroupById(id, ownerEmail)
    if (!docId || !group) return false
    await this.repository.deleteGroup(docId, group)
    return true
  }

  async assertGroups(ownerEmail, groupIds) {
    if (!groupIds.length) return
    const owned = new Set((await this.listGroups(ownerEmail)).map(group => group.id))
    if (groupIds.some(id => !owned.has(id))) throw new Error('contact group not found')
  }
}

function normalizeContact(ownerEmail, input, metadata) {
  const emails = normalizeValues(input.emails, value => {
    const email = Security.normalizeEmailAddress(value)
    if (!email) throw new Error('invalid contact email')
    return email
  })
  const phones = normalizeValues(input.phones, value => validateText(value, 'phone', 80, true))
  const urls = normalizeValues(input.urls, value => {
    let url
    try { url = new URL(value) } catch { throw new Error('invalid contact URL') }
    if (!['http:', 'https:'].includes(url.protocol)) throw new Error('invalid contact URL')
    return url.href
  })
  const addresses = Array.isArray(input.addresses) ? input.addresses.slice(0, MAX_VALUES).map(address => ({
    streetAddress: validateText(address?.streetAddress, 'streetAddress', 300),
    extendedAddress: validateText(address?.extendedAddress, 'extendedAddress', 300),
    city: validateText(address?.city, 'city', 120), region: validateText(address?.region, 'region', 120),
    postalCode: validateText(address?.postalCode, 'postalCode', 40), country: validateText(address?.country, 'country', 120),
    ...(address?.type ? { type: validateType(address.type) } : {}),
  })) : []
  const name = validateText(input.name, 'name', 200)
  if (!name && emails.length === 0 && phones.length === 0) throw new Error('name, email, or phone is required')
  return {
    id: metadata.id, ownerEmail: ownerEmail.toLowerCase(), domain: ownerDomain(ownerEmail),
    source: metadata.source, name,
    givenName: validateText(input.givenName, 'givenName', 100), familyName: validateText(input.familyName, 'familyName', 100),
    company: validateText(input.company, 'company', 200), title: validateText(input.title, 'title', 200),
    emails, phones, addresses, urls,
    nicknames: uniqueStrings(input.nicknames, 100, 50),
    birthday: validateText(input.birthday, 'birthday', 32), notes: validateText(input.notes, 'notes', 10000),
    groupIds: uniqueStrings(input.groupIds, 140, 100),
    ...(Number.isInteger(metadata.interactionCount) ? { interactionCount: Number(metadata.interactionCount) } : {}),
    ...(metadata.lastContactedAt ? { lastContactedAt: String(metadata.lastContactedAt) } : {}),
    createdAt: metadata.createdAt, updatedAt: metadata.updatedAt,
  }
}

function normalizeProviderContact(ownerEmail, input) {
  const safeArray = value => Array.isArray(value) ? value : []
  return {
    ...input,
    ownerEmail: ownerEmail.toLowerCase(), domain: ownerDomain(ownerEmail), source: 'carddav', readOnly: false,
    name: String(input?.name || ''), givenName: String(input?.givenName || ''), familyName: String(input?.familyName || ''),
    company: String(input?.company || ''), title: String(input?.title || ''), birthday: String(input?.birthday || ''), notes: String(input?.notes || ''),
    emails: safeArray(input?.emails), phones: safeArray(input?.phones), addresses: safeArray(input?.addresses), urls: safeArray(input?.urls),
    nicknames: safeArray(input?.nicknames), groupIds: [],
  }
}

function normalizeValues(input, transform) {
  if (!Array.isArray(input)) return []
  const seen = new Set()
  return input.slice(0, MAX_VALUES).flatMap(entry => {
    const raw = typeof entry === 'string' ? entry : entry?.value
    if (typeof raw !== 'string' || !raw.trim()) return []
    const value = transform(raw.trim())
    const key = value.toLowerCase()
    if (seen.has(key)) return []
    seen.add(key)
    const type = typeof entry === 'object' && entry?.type ? validateType(entry.type) : ''
    return [{ value, ...(type ? { type } : {}) }]
  })
}

function validateText(value, field, max, required = false) {
  const text = typeof value === 'string' ? value.trim() : ''
  if (required && !text) throw new Error(`${field} is required`)
  if (text.length > max || /[\0\r]/.test(text)) throw new Error(`${field} is invalid`)
  return text
}

function validateType(value) { return validateText(value, 'type', 40).toLowerCase().replace(/[^a-z0-9-]/g, '') }
function uniqueStrings(input, maxLength, maxItems) { return [...new Set((Array.isArray(input) ? input : []).map(value => validateText(value, 'value', maxLength)).filter(Boolean))].slice(0, maxItems) }
function ownerDomain(email) { const domain = email.toLowerCase().split('@')[1]; if (!domain) throw new Error('invalid contact owner'); return domain }
function searchable(contact) {
  return [
    contact.name, contact.givenName, contact.familyName, contact.company,
    contact.title, contact.notes, contact.birthday,
    ...contact.nicknames,
    ...contact.emails.map(item => item.value),
    ...contact.phones.map(item => item.value),
    ...contact.urls.map(item => item.value),
    ...contact.addresses.flatMap(address => [
      address.streetAddress, address.extendedAddress, address.city,
      address.region, address.postalCode, address.country,
    ]),
  ].join('\n').toLowerCase()
}
function stripDocId(value) { const { docId: _docId, ...record } = value; return record }
