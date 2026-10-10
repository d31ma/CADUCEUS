import { Buffer } from 'node:buffer'

const MAX_VCARD_BYTES = 5 * 1024 * 1024
const MAX_CARDS = 1000

/** RFC 6350-compatible bounded vCard import/export helpers. */
export class VCard {
  static parseFile(input) {
    const content = String(input || '')
    if (Buffer.byteLength(content, 'utf8') > MAX_VCARD_BYTES) throw new Error('vCard file exceeds 5MB')
    const cards = content.match(/BEGIN:VCARD[\s\S]*?END:VCARD/gi) || []
    if (cards.length > MAX_CARDS) throw new Error('vCard file contains more than 1000 contacts')
    return cards.map(parseCard).filter(Boolean)
  }

  static serializeContacts(contacts) {
    return contacts.map(serializeCard).join('\r\n')
  }
}

function parseCard(source) {
  const properties = unfold(source).slice(1, -1).map(parseProperty).filter(Boolean)
  const values = name => properties.filter(item => item.name === name)
  const first = name => values(name)[0]?.value || ''
  const structuredName = first('N').split(';').map(unescapeText)
  const name = unescapeText(first('FN')) || [structuredName[1], structuredName[0]].filter(Boolean).join(' ')
  const emails = values('EMAIL').map(item => typedValue(item, value => value.trim().toLowerCase())).filter(item => item.value)
  if (!name && emails.length === 0) return null
  return {
    name,
    givenName: structuredName[1] || '',
    familyName: structuredName[0] || '',
    company: unescapeText(first('ORG').split(';')[0] || ''),
    title: unescapeText(first('TITLE')),
    emails,
    phones: values('TEL').map(item => typedValue(item, unescapeText)).filter(item => item.value),
    addresses: values('ADR').map(item => {
      const parts = item.value.split(';').map(unescapeText)
      return {
        extendedAddress: parts[1] || '', streetAddress: parts[2] || '', city: parts[3] || '',
        region: parts[4] || '', postalCode: parts[5] || '', country: parts[6] || '',
        ...(item.type ? { type: item.type } : {}),
      }
    }),
    urls: values('URL').map(item => typedValue(item, unescapeText)).filter(item => item.value),
    nicknames: values('NICKNAME').flatMap(item => item.value.split(',').map(unescapeText)).filter(Boolean),
    birthday: unescapeText(first('BDAY')),
    notes: unescapeText(first('NOTE')),
  }
}

function serializeCard(contact) {
  const lines = ['BEGIN:VCARD', 'VERSION:4.0', `UID:${escapeText(contact.id)}`]
  const display = contact.name || [contact.givenName, contact.familyName].filter(Boolean).join(' ') || contact.emails[0]?.value || 'Contact'
  lines.push(`FN:${escapeText(display)}`)
  lines.push(`N:${escapeText(contact.familyName || '')};${escapeText(contact.givenName || '')};;;`)
  if (contact.company) lines.push(`ORG:${escapeText(contact.company)}`)
  if (contact.title) lines.push(`TITLE:${escapeText(contact.title)}`)
  for (const item of contact.emails || []) lines.push(`EMAIL${typeParam(item.type)}:${escapeText(item.value)}`)
  for (const item of contact.phones || []) lines.push(`TEL${typeParam(item.type)}:${escapeText(item.value)}`)
  for (const address of contact.addresses || []) {
    const value = ['', address.extendedAddress || '', address.streetAddress || '', address.city || '', address.region || '', address.postalCode || '', address.country || '']
      .map(escapeText).join(';')
    lines.push(`ADR${typeParam(address.type)}:${value}`)
  }
  for (const item of contact.urls || []) lines.push(`URL${typeParam(item.type)}:${escapeText(item.value)}`)
  if (contact.nicknames?.length) lines.push(`NICKNAME:${contact.nicknames.map(escapeText).join(',')}`)
  if (contact.birthday) lines.push(`BDAY:${escapeText(contact.birthday)}`)
  if (contact.notes) lines.push(`NOTE:${escapeText(contact.notes)}`)
  lines.push('END:VCARD')
  return lines.map(foldLine).join('\r\n')
}

function unfold(value) {
  return value.replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/\n[ \t]/g, '').split('\n').map(line => line.trimEnd())
}

function parseProperty(line) {
  const colon = findUnescapedColon(line)
  if (colon <= 0) return null
  const head = line.slice(0, colon).split(';')
  const name = head.shift().split('.').pop().toUpperCase()
  const params = Object.fromEntries(head.map(value => {
    const [key, ...rest] = value.split('=')
    return [String(key).toUpperCase(), rest.join('=').replace(/^"|"$/g, '')]
  }))
  return { name, value: line.slice(colon + 1), type: String(params.TYPE || '').split(',')[0].toLowerCase() || undefined }
}

function findUnescapedColon(line) {
  let quoted = false
  for (let index = 0; index < line.length; index++) {
    if (line[index] === '"') quoted = !quoted
    if (line[index] === ':' && !quoted) return index
  }
  return -1
}

function typedValue(item, transform) {
  return { value: transform(item.value), ...(item.type ? { type: item.type } : {}) }
}

function unescapeText(value) {
  return String(value || '').replace(/\\n/gi, '\n').replace(/\\,/g, ',').replace(/\\;/g, ';').replace(/\\\\/g, '\\')
}

function escapeText(value) {
  return String(value || '').replace(/\\/g, '\\\\').replace(/\r?\n/g, '\\n').replace(/,/g, '\\,').replace(/;/g, '\\;')
}

function typeParam(type) {
  const safe = String(type || '').toLowerCase().replace(/[^a-z0-9-]/g, '')
  return safe ? `;TYPE=${safe}` : ''
}

function foldLine(line) {
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
