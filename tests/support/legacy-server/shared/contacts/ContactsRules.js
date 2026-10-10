import { randomUUID } from 'node:crypto'
import { Security } from '../security/Security.js'
import { VCard } from './VCard.js'

export class ContactsRules {
  static MAX_VALUES = 50

  static normalizeContact(ownerEmail, input, metadata) {
    const emails = ContactsRules.normalizeValues(input.emails, value => {
      const email = Security.normalizeEmailAddress(value)
      if (!email) throw new Error('invalid contact email')
      return email
    })
    const phones = ContactsRules.normalizeValues(input.phones, value => ContactsRules.validateText(value, 'phone', 80, true))
    const urls = ContactsRules.normalizeValues(input.urls, value => {
      let url
      try { url = new URL(value) } catch { throw new Error('invalid contact URL') }
      if (!['http:', 'https:'].includes(url.protocol)) throw new Error('invalid contact URL')
      return url.href
    })
    const addresses = Array.isArray(input.addresses) ? input.addresses.slice(0, ContactsRules.MAX_VALUES).map(address => ({
      streetAddress: ContactsRules.validateText(address?.streetAddress, 'streetAddress', 300),
      extendedAddress: ContactsRules.validateText(address?.extendedAddress, 'extendedAddress', 300),
      city: ContactsRules.validateText(address?.city, 'city', 120), region: ContactsRules.validateText(address?.region, 'region', 120),
      postalCode: ContactsRules.validateText(address?.postalCode, 'postalCode', 40), country: ContactsRules.validateText(address?.country, 'country', 120),
      ...(address?.type ? { type: ContactsRules.validateType(address.type) } : {}),
    })) : []
    const name = ContactsRules.validateText(input.name, 'name', 200)
    if (!name && emails.length === 0 && phones.length === 0) throw new Error('name, email, or phone is required')
    return {
      id: metadata.id, ownerEmail: ownerEmail.toLowerCase(), domain: ContactsRules.ownerDomain(ownerEmail),
      source: metadata.source, name,
      givenName: ContactsRules.validateText(input.givenName, 'givenName', 100), familyName: ContactsRules.validateText(input.familyName, 'familyName', 100),
      company: ContactsRules.validateText(input.company, 'company', 200), title: ContactsRules.validateText(input.title, 'title', 200),
      emails, phones, addresses, urls,
      nicknames: ContactsRules.uniqueStrings(input.nicknames, 100, 50),
      birthday: ContactsRules.validateText(input.birthday, 'birthday', 32), notes: ContactsRules.validateText(input.notes, 'notes', 10000),
      groupIds: ContactsRules.uniqueStrings(input.groupIds, 140, 100),
      ...(Number.isInteger(metadata.interactionCount) ? { interactionCount: Number(metadata.interactionCount) } : {}),
      ...(metadata.lastContactedAt ? { lastContactedAt: String(metadata.lastContactedAt) } : {}),
      createdAt: metadata.createdAt, updatedAt: metadata.updatedAt,
    }
  }

  static normalizeProviderContact(ownerEmail, input) {
    const safeArray = value => Array.isArray(value) ? value : []
    return {
      ...input,
      ownerEmail: ownerEmail.toLowerCase(), domain: ContactsRules.ownerDomain(ownerEmail), source: 'carddav', readOnly: false,
      name: String(input?.name || ''), givenName: String(input?.givenName || ''), familyName: String(input?.familyName || ''),
      company: String(input?.company || ''), title: String(input?.title || ''), birthday: String(input?.birthday || ''), notes: String(input?.notes || ''),
      emails: safeArray(input?.emails), phones: safeArray(input?.phones), addresses: safeArray(input?.addresses), urls: safeArray(input?.urls),
      nicknames: safeArray(input?.nicknames), groupIds: [],
    }
  }

  static normalizeValues(input, transform) {
    if (!Array.isArray(input)) return []
    const seen = new Set()
    return input.slice(0, ContactsRules.MAX_VALUES).flatMap(entry => {
      const raw = typeof entry === 'string' ? entry : entry?.value
      if (typeof raw !== 'string' || !raw.trim()) return []
      const value = transform(raw.trim())
      const key = value.toLowerCase()
      if (seen.has(key)) return []
      seen.add(key)
      const type = typeof entry === 'object' && entry?.type ? ContactsRules.validateType(entry.type) : ''
      return [{ value, ...(type ? { type } : {}) }]
    })
  }

  static validateText(value, field, max, required = false) {
    const text = typeof value === 'string' ? value.trim() : ''
    if (required && !text) throw new Error(`${field} is required`)
    if (text.length > max || /[\0\r]/.test(text)) throw new Error(`${field} is invalid`)
    return text
  }

  static validateType(value) { return ContactsRules.validateText(value, 'type', 40).toLowerCase().replace(/[^a-z0-9-]/g, '') }

  static uniqueStrings(input, maxLength, maxItems) { return [...new Set((Array.isArray(input) ? input : []).map(value => ContactsRules.validateText(value, 'value', maxLength)).filter(Boolean))].slice(0, maxItems) }

  static ownerDomain(email) { const domain = email.toLowerCase().split('@')[1]; if (!domain) throw new Error('invalid contact owner'); return domain }

  static searchable(contact) {
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

  static stripDocId(value) { const { docId: _docId, ...record } = value; return record }
}
