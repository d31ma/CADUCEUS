import { Collections, collect } from '../../../platform/persistence/Fylo.js'
import { FyloAccessContext } from '../../../platform/persistence/FyloAccessContext.js'

/** Personal contacts and groups, always isolated by exact mailbox owner. */
export class ContactsRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @returns {Promise<Array<import('../../../types').Contact & {docId: string}>>} */
  async list(ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await collect(this.fylo.findDocs(Collections.CONTACTS, {
      $ops: [{ ownerEmail: { $eq: owner } }],
    }).collect())
    return Object.entries(docs)
      .map(([docId, raw]) => ({ docId, ...hydrate(raw) }))
      .filter(contact => contact.ownerEmail === owner)
  }

  async findById(id, ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await collect(this.fylo.findDocs(Collections.CONTACTS, {
      $ops: [{ id: { $eq: id } }],
    }).collect())
    for (const [docId, raw] of Object.entries(docs)) {
      const contact = hydrate(raw)
      if (contact.ownerEmail === owner) return [docId, contact]
    }
    return [null, null]
  }

  async findByEmail(ownerEmail, address) {
    const needle = address.toLowerCase()
    return (await this.list(ownerEmail)).find(contact =>
      contact.emails.some(entry => entry.value.toLowerCase() === needle),
    ) || null
  }

  /** @param {import('../../../types').Contact} contact */
  async put(contact) {
    return await FyloAccessContext.runForDomain(contact.domain, () =>
      this.fylo.putData(Collections.CONTACTS, serialize(contact)))
  }

  /** @param {string} docId @param {import('../../../types').Contact} contact */
  async replace(docId, contact) {
    await FyloAccessContext.runForDomain(contact.domain, () =>
      this.fylo.patchDoc(Collections.CONTACTS, { [docId]: serialize(contact) }))
  }

  async delete(docId, domain) {
    await FyloAccessContext.runForDomain(domain, () => this.fylo.delDoc(Collections.CONTACTS, docId))
  }

  /** @returns {Promise<Array<import('../../../types').ContactGroup & {docId: string}>>} */
  async listGroups(ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await collect(this.fylo.findDocs(Collections.CONTACT_GROUPS, {
      $ops: [{ ownerEmail: { $eq: owner } }],
    }).collect())
    return Object.entries(docs)
      .map(([docId, group]) => ({
        docId,
        .../** @type {import('../../../types').ContactGroup} */ (group),
      }))
      .filter(group => group.ownerEmail === owner)
  }

  async findGroupById(id, ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const groups = await this.listGroups(owner)
    const group = groups.find(item => item.id === id)
    return group ? [group.docId, stripDocId(group)] : [null, null]
  }

  /** @param {import('../../../types').ContactGroup} group */
  async putGroup(group) {
    return await FyloAccessContext.runForDomain(group.domain, () =>
      this.fylo.putData(Collections.CONTACT_GROUPS, group))
  }

  /** @param {string} docId @param {import('../../../types').ContactGroup} group */
  async replaceGroup(docId, group) {
    await FyloAccessContext.runForDomain(group.domain, () =>
      this.fylo.patchDoc(Collections.CONTACT_GROUPS, { [docId]: group }))
  }

  async deleteGroup(docId, group) {
    await FyloAccessContext.runForDomain(group.domain, async () => {
      await this.fylo.delDoc(Collections.CONTACT_GROUPS, docId)
      for (const contact of await this.list(group.ownerEmail)) {
        if (!contact.groupIds.includes(group.id)) continue
        await this.replace(contact.docId, {
          ...stripDocId(contact),
          groupIds: contact.groupIds.filter(id => id !== group.id),
          updatedAt: new Date().toISOString(),
        })
      }
    })
  }
}

function serialize(contact) {
  const { docId: _docId, emails, phones, addresses, urls, nicknames, groupIds, ...record } = contact
  return {
    ...record,
    emailsJson: JSON.stringify(emails || []),
    phonesJson: JSON.stringify(phones || []),
    addressesJson: JSON.stringify(addresses || []),
    urlsJson: JSON.stringify(urls || []),
    nicknamesJson: JSON.stringify(nicknames || []),
    groupIdsJson: JSON.stringify(groupIds || []),
  }
}

function hydrate(raw) {
  const contact = { ...raw }
  for (const [field, stored] of [
    ['emails', 'emailsJson'], ['phones', 'phonesJson'], ['addresses', 'addressesJson'],
    ['urls', 'urlsJson'], ['nicknames', 'nicknamesJson'], ['groupIds', 'groupIdsJson'],
  ]) {
    try { contact[field] = JSON.parse(String(contact[stored] || '[]')) } catch { contact[field] = [] }
    delete contact[stored]
  }
  return contact
}

function stripDocId(value) {
  const { docId: _docId, ...record } = value
  return record
}
