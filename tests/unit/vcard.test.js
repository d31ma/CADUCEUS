import { describe, expect, it } from 'bun:test'
import { VCard } from '@/shared/contacts/VCard.js'

describe('vCard interchange', () => {
  it('imports folded RFC 6350 contact details and skips empty cards', () => {
    const contacts = VCard.parseFile([
      'BEGIN:VCARD', 'VERSION:4.0', 'FN:Ada Lovelace', 'N:Lovelace;Ada;;;',
      'EMAIL;TYPE=work:ADA@EXAMPLE.COM', 'TEL;TYPE=cell:+1 555 0100',
      'ADR;TYPE=work:;;12 Computing Ln;London;;SW1;UK',
      'NOTE:First line\\nsecond ', '\tline', 'END:VCARD',
      'BEGIN:VCARD', 'VERSION:4.0', 'END:VCARD',
    ].join('\r\n'))
    expect(contacts).toHaveLength(1)
    expect(contacts[0]).toMatchObject({
      name: 'Ada Lovelace', givenName: 'Ada', familyName: 'Lovelace',
      emails: [{ value: 'ada@example.com', type: 'work' }],
      phones: [{ value: '+1 555 0100', type: 'cell' }],
      notes: 'First line\nsecond line',
    })
    expect(contacts[0].addresses[0].city).toBe('London')
  })

  it('exports UTF-8 contacts with CRLF and round-trips structured fields', () => {
    const source = {
      id: 'contact-one', name: 'Zoë Example', givenName: 'Zoë', familyName: 'Example',
      company: 'R&D, Inc.', title: 'Engineer', emails: [{ value: 'zoe@example.com', type: 'work' }],
      phones: [], addresses: [], urls: [{ value: 'https://example.com/', type: 'home' }],
      nicknames: ['Z'], birthday: '2000-02-03', notes: 'Line 1\nLine 2',
    }
    const output = VCard.serializeContacts([source])
    expect(output).toContain('\r\n')
    expect(output).toContain('FN:Zoë Example')
    expect(output).toContain('ORG:R&D\\, Inc.')
    expect(output).not.toMatch(/(^|[^\r])\n/)
    expect(VCard.parseFile(output)[0]).toMatchObject({
      name: 'Zoë Example', company: 'R&D, Inc.', birthday: '2000-02-03', notes: 'Line 1\nLine 2',
    })
  })

  it('rejects oversized and pathological files', () => {
    expect(() => VCard.parseFile('x'.repeat(5 * 1024 * 1024 + 1))).toThrow(/5MB/)
    expect(() => VCard.parseFile('BEGIN:VCARD\nFN:X\nEND:VCARD\n'.repeat(1001))).toThrow(/1000/)
  })
})
