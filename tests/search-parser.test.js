import { describe, test, expect } from 'bun:test'
import { SearchParser } from '../server/features/mailbox/services/SearchParser.js'

const message = (overrides = {}) => ({
  id: 'message-1', ownerEmail: 'alice@example.com', domain: 'example.com',
  sender: 'Carol Example <carol@example.net>', recipient: 'alice@example.com',
  to: ['Alice <alice@example.com>'], cc: [], bcc: [], subject: 'Quarterly Planning',
  body: 'The launch checklist is ready for review.',
  htmlBody: '<p>The launch checklist is ready for review.</p>', folder: 'inbox',
  labels: ['projects/atlas'], read: false, starred: true,
  receivedAt: '2026-08-10T12:00:00.000Z', ...overrides,
})

const attachments = [{ filename: 'roadmap final.pdf', contentType: 'application/pdf' }]
const matches = (query, overrides = {}, files = attachments) =>
  SearchParser.matches(SearchParser.parse(query), message(overrides), files, {
    now: new Date('2026-08-15T12:00:00.000Z'),
  })

describe('SearchParser', () => {
  test('parses the complete Mailspring field and boolean vocabulary', () => {
    expect(SearchParser.parse('from:carol to:alice subject:"Quarterly Planning"')).toEqual({
      type: 'and', left: { type: 'term', field: 'from', value: 'carol' },
      right: { type: 'and', left: { type: 'term', field: 'to', value: 'alice' },
        right: { type: 'term', field: 'subject', value: 'Quarterly Planning' } },
    })
    expect(matches('is:unread is:starred has:attachment')).toBe(true)
    expect(matches('is:read')).toBe(false)
    expect(matches('is:unstarred')).toBe(false)
    expect(matches('has:attachments')).toBe(true)
  })

  test('supports quoted phrases, implicit AND, explicit AND, OR, NOT, and parentheses', () => {
    expect(matches('(from:carol OR from:grace) AND "launch checklist" NOT in:trash')).toBe(true)
    expect(matches('(from:grace OR from:ada) AND "launch checklist"')).toBe(false)
    expect(matches('NOT in:trash')).toBe(true)
    expect(matches('from:carol OR from:grace AND is:read')).toBe(false)
  })

  test('searches sender, all recipients, subject, message bodies, folders, labels, and filenames', () => {
    for (const query of [
      'carol', 'alice@example.com', 'Quarterly', 'checklist', 'in:inbox',
      'in:"projects/atlas"', 'filename:"roadmap final.pdf"', 'attachment:roadmap',
    ]) expect(matches(query)).toBe(true)
    expect(matches('in:trash')).toBe(false)
    expect(matches('filename:invoice')).toBe(false)
    expect(matches('provider preview', { body: '', htmlBody: '', snippet: 'Provider preview text' }, [])).toBe(true)
  })

  test('supports since/after, before, relative dates, and ignores invalid dates', () => {
    expect(matches('after:2026-08-01 before:2026-09-01')).toBe(true)
    expect(matches('since:"2 days ago"')).toBe(false)
    expect(matches('before:yesterday')).toBe(true)
    expect(matches('before:not-a-date')).toBe(true)
  })

  test('preserves term capitalization and treats unknown fields as literal generic text', () => {
    expect(SearchParser.parse('subject:HeLlO')).toEqual({ type: 'term', field: 'subject', value: 'HeLlO' })
    expect(matches('x-custom:value', { body: 'contains x-custom:value verbatim' }, [])).toBe(true)
  })

  test('rejects structurally incomplete input and returns no expression for blank input', () => {
    expect(SearchParser.parse('')).toBeNull()
    expect(SearchParser.parse('   ')).toBeNull()
    expect(() => SearchParser.parse('subject:"unterminated')).toThrow(/quoted/i)
    expect(() => SearchParser.parse('(from:carol')).toThrow(/\)/)
    expect(() => SearchParser.parse('from:')).toThrow(/value/i)
  })

  test('reports explicit category fields anywhere in a boolean expression', () => {
    expect(SearchParser.hasField(SearchParser.parse('subject:hello OR NOT in:trash'), 'in')).toBe(true)
    expect(SearchParser.hasField(SearchParser.parse('subject:hello is:unread'), 'in')).toBe(false)
  })
})
