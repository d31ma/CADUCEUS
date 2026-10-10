import { Buffer } from 'node:buffer'

/** RFC 6350-compatible bounded vCard import/export helpers. */
export class VCard {
  static #MAX_VCARD_BYTES = 5 * 1024 * 1024

  static #MAX_CARDS = 1000

  static #parseCard(source) {
    const properties = VCard.#unfold(source).slice(1, -1).map(VCard.#parseProperty).filter(Boolean)
    const values = name => properties.filter(item => item.name === name)
    const first = name => values(name)[0]?.value || ''
    const structuredName = first('N').split(';').map(VCard.#unescapeText)
    const name = VCard.#unescapeText(first('FN')) || [structuredName[1], structuredName[0]].filter(Boolean).join(' ')
    const emails = values('EMAIL').map(item => VCard.#typedValue(item, value => value.trim().toLowerCase())).filter(item => item.value)
    if (!name && emails.length === 0) return null
    return {
      name,
      givenName: structuredName[1] || '',
      familyName: structuredName[0] || '',
      company: VCard.#unescapeText(first('ORG').split(';')[0] || ''),
      title: VCard.#unescapeText(first('TITLE')),
      emails,
      phones: values('TEL').map(item => VCard.#typedValue(item, VCard.#unescapeText)).filter(item => item.value),
      addresses: values('ADR').map(item => {
        const parts = item.value.split(';').map(VCard.#unescapeText)
        return {
          extendedAddress: parts[1] || '', streetAddress: parts[2] || '', city: parts[3] || '',
          region: parts[4] || '', postalCode: parts[5] || '', country: parts[6] || '',
          ...(item.type ? { type: item.type } : {}),
        }
      }),
      urls: values('URL').map(item => VCard.#typedValue(item, VCard.#unescapeText)).filter(item => item.value),
      nicknames: values('NICKNAME').flatMap(item => item.value.split(',').map(VCard.#unescapeText)).filter(Boolean),
      birthday: VCard.#unescapeText(first('BDAY')),
      notes: VCard.#unescapeText(first('NOTE')),
    }
  }

  static #serializeCard(contact) {
    const lines = ['BEGIN:VCARD', 'VERSION:4.0', `UID:${VCard.#escapeText(contact.id)}`]
    const display = contact.name || [contact.givenName, contact.familyName].filter(Boolean).join(' ') || contact.emails[0]?.value || 'Contact'
    lines.push(`FN:${VCard.#escapeText(display)}`)
    lines.push(`N:${VCard.#escapeText(contact.familyName || '')};${VCard.#escapeText(contact.givenName || '')};;;`)
    if (contact.company) lines.push(`ORG:${VCard.#escapeText(contact.company)}`)
    if (contact.title) lines.push(`TITLE:${VCard.#escapeText(contact.title)}`)
    for (const item of contact.emails || []) lines.push(`EMAIL${VCard.#typeParam(item.type)}:${VCard.#escapeText(item.value)}`)
    for (const item of contact.phones || []) lines.push(`TEL${VCard.#typeParam(item.type)}:${VCard.#escapeText(item.value)}`)
    for (const address of contact.addresses || []) {
      const value = ['', address.extendedAddress || '', address.streetAddress || '', address.city || '', address.region || '', address.postalCode || '', address.country || '']
        .map(VCard.#escapeText).join(';')
      lines.push(`ADR${VCard.#typeParam(address.type)}:${value}`)
    }
    for (const item of contact.urls || []) lines.push(`URL${VCard.#typeParam(item.type)}:${VCard.#escapeText(item.value)}`)
    if (contact.nicknames?.length) lines.push(`NICKNAME:${contact.nicknames.map(VCard.#escapeText).join(',')}`)
    if (contact.birthday) lines.push(`BDAY:${VCard.#escapeText(contact.birthday)}`)
    if (contact.notes) lines.push(`NOTE:${VCard.#escapeText(contact.notes)}`)
    lines.push('END:VCARD')
    return lines.map(VCard.#foldLine).join('\r\n')
  }

  static #unfold(value) {
    return value.replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/\n[ \t]/g, '').split('\n').map(line => line.trimEnd())
  }

  static #parseProperty(line) {
    const colon = VCard.#findUnescapedColon(line)
    if (colon <= 0) return null
    const head = line.slice(0, colon).split(';')
    const name = head.shift().split('.').pop().toUpperCase()
    const params = Object.fromEntries(head.map(value => {
      const [key, ...rest] = value.split('=')
      return [String(key).toUpperCase(), rest.join('=').replace(/^\x22|\x22$/g, '')]
    }))
    return { name, value: line.slice(colon + 1), type: String(params.TYPE || '').split(',')[0].toLowerCase() || undefined }
  }

  static #findUnescapedColon(line) {
    let quoted = false
    for (let index = 0; index < line.length; index++) {
      if (line[index] === '"') quoted = !quoted
      if (line[index] === ':' && !quoted) return index
    }
    return -1
  }

  static #typedValue(item, transform) {
    return { value: transform(item.value), ...(item.type ? { type: item.type } : {}) }
  }

  static #unescapeText(value) {
    return String(value || '').replace(/\\n/gi, '\n').replace(/\\,/g, ',').replace(/\\;/g, ';').replace(/\\\\/g, '\\')
  }

  static #escapeText(value) {
    return String(value || '').replace(/\\/g, '\\\\').replace(/\r?\n/g, '\\n').replace(/,/g, '\\,').replace(/;/g, '\\;')
  }

  static #typeParam(type) {
    const safe = String(type || '').toLowerCase().replace(/[^a-z0-9-]/g, '')
    return safe ? `;TYPE=${safe}` : ''
  }

  static #foldLine(line) {
    const chunks = []
    let current = ''
    for (const character of line) {
      if (Buffer.byteLength(current + character, 'utf8') > 73) {
        chunks.push(current)
        current = character
      } else current += character
    }
    chunks.push(current)
    return chunks.join('\r\n ')
  }

  static parseFile(input) {
    const content = String(input || '')
    if (Buffer.byteLength(content, 'utf8') > VCard.#MAX_VCARD_BYTES) throw new Error('vCard file exceeds 5MB')
    const cards = content.match(/BEGIN:VCARD[\s\S]*?END:VCARD/gi) || []
    if (cards.length > VCard.#MAX_CARDS) throw new Error('vCard file contains more than 1000 contacts')
    return cards.map(VCard.#parseCard).filter(Boolean)
  }

  static serializeContacts(contacts) {
    return contacts.map(VCard.#serializeCard).join('\r\n')
  }
}
