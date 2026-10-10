import { Fylo } from '../persistence/Fylo.js'
import { FyloAccessContext } from '../persistence/FyloAccessContext.js'

export class ContactsRecords {
  static serialize(contact) {
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

  static hydrate(raw) {
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

  static stripDocId(value) {
    const { docId: _docId, ...record } = value
    return record
  }
}
