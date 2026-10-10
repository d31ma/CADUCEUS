import { Fylo } from '../../../shared/persistence/Fylo.js'
import { FyloAccessContext } from '../../../shared/persistence/FyloAccessContext.js'
import { ContactsRecords } from '../../../shared/contacts/ContactsRecords.js'

/** Personal contacts and groups, always isolated by exact mailbox owner. */
export class ContactsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @returns {Promise<Array<import('../../../types').Contact & {docId: string}>>} */
  async list(ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.CONTACTS, {
      $ops: [{ ownerEmail: { $eq: owner } }],
    }).collect())
    return Object.entries(docs)
      .map(([docId, raw]) => ({ docId, ...ContactsRecords.hydrate(raw) }))
      .filter(contact => contact.ownerEmail === owner)
  }

  async findById(id, ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.CONTACTS, {
      $ops: [{ id: { $eq: id } }],
    }).collect())
    for (const [docId, raw] of Object.entries(docs)) {
      const contact = ContactsRecords.hydrate(raw)
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
      this.fylo.putData(Fylo.Collections.CONTACTS, ContactsRecords.serialize(contact)))
  }

  /** @param {string} docId @param {import('../../../types').Contact} contact */
  async replace(docId, contact) {
    await FyloAccessContext.runForDomain(contact.domain, () =>
      this.fylo.patchDoc(Fylo.Collections.CONTACTS, { [docId]: ContactsRecords.serialize(contact) }))
  }

  async delete(docId, domain) {
    await FyloAccessContext.runForDomain(domain, () => this.fylo.delDoc(Fylo.Collections.CONTACTS, docId))
  }

  /** @returns {Promise<Array<import('../../../types').ContactGroup & {docId: string}>>} */
  async listGroups(ownerEmail) {
    const owner = ownerEmail.toLowerCase()
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.CONTACT_GROUPS, {
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
    return group ? [group.docId, ContactsRecords.stripDocId(group)] : [null, null]
  }

  /** @param {import('../../../types').ContactGroup} group */
  async putGroup(group) {
    return await FyloAccessContext.runForDomain(group.domain, () =>
      this.fylo.putData(Fylo.Collections.CONTACT_GROUPS, group))
  }

  /** @param {string} docId @param {import('../../../types').ContactGroup} group */
  async replaceGroup(docId, group) {
    await FyloAccessContext.runForDomain(group.domain, () =>
      this.fylo.patchDoc(Fylo.Collections.CONTACT_GROUPS, { [docId]: group }))
  }

  async deleteGroup(docId, group) {
    await FyloAccessContext.runForDomain(group.domain, async () => {
      await this.fylo.delDoc(Fylo.Collections.CONTACT_GROUPS, docId)
      for (const contact of await this.list(group.ownerEmail)) {
        if (!contact.groupIds.includes(group.id)) continue
        await this.replace(contact.docId, {
          ...ContactsRecords.stripDocId(contact),
          groupIds: contact.groupIds.filter(id => id !== group.id),
          updatedAt: new Date().toISOString(),
        })
      }
    })
  }
}

