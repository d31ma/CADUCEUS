import { describe, it, expect } from 'bun:test'
import { Ics } from '@/features/calendar/services/Ics.js'

const vevent = (body, method = 'METHOD:REQUEST\r\n') => [
  'BEGIN:VCALENDAR',
  'VERSION:2.0',
  'PRODID:-//Test//EN',
  method.trim(),
  'BEGIN:VEVENT',
  body,
  'END:VEVENT',
  'END:VCALENDAR',
].join('\r\n')

const MINIMAL = [
  'UID:evt-1@example.com',
  'SUMMARY:Standup',
  'DTSTART:20260301T090000Z',
  'DTEND:20260301T093000Z',
].join('\r\n')

describe('Ics.looksLikeIcs', () => {
  it('rejects non-strings and empty input', () => {
    expect(Ics.looksLikeIcs('')).toBe(false)
    expect(Ics.looksLikeIcs(null)).toBe(false)
    expect(Ics.looksLikeIcs(42)).toBe(false)
  })

  it('rejects text without a VCALENDAR header', () => {
    expect(Ics.looksLikeIcs('hello world')).toBe(false)
  })

  it('accepts a VCALENDAR payload on any line', () => {
    expect(Ics.looksLikeIcs('BEGIN:VCALENDAR\r\nEND:VCALENDAR')).toBe(true)
    expect(Ics.looksLikeIcs('preamble\nbegin:vcalendar')).toBe(true)
  })
})

describe('Ics.parseIcs', () => {
  it('returns null for a non-calendar payload', () => {
    expect(Ics.parseIcs('nope')).toBeNull()
  })

  it('reads the method and prodid', () => {
    const cal = Ics.parseIcs(vevent(MINIMAL))
    expect(cal.method).toBe('REQUEST')
    expect(cal.prodid).toBe('-//Test//EN')
  })

  it('parses a minimal VEVENT', () => {
    const [event] = Ics.parseIcs(vevent(MINIMAL)).events
    expect(event.uid).toBe('evt-1@example.com')
    expect(event.summary).toBe('Standup')
    expect(event.start).toBe('2026-03-01T09:00:00.000Z')
    expect(event.end).toBe('2026-03-01T09:30:00.000Z')
    expect(event.allDay).toBe(false)
  })

  it('drops a VEVENT missing required fields', () => {
    expect(Ics.parseIcs(vevent('UID:only-a-uid')).events).toEqual([])
  })

  it('parses multiple events', () => {
    const text = [
      'BEGIN:VCALENDAR', 'METHOD:REQUEST',
      'BEGIN:VEVENT', MINIMAL, 'END:VEVENT',
      'BEGIN:VEVENT', MINIMAL.replace('evt-1', 'evt-2'), 'END:VEVENT',
      'END:VCALENDAR',
    ].join('\r\n')
    expect(Ics.parseIcs(text).events).toHaveLength(2)
  })

  it('unfolds continuation lines', () => {
    const folded = vevent([MINIMAL, 'DESCRIPTION:first part', ' second part'].join('\r\n'))
    expect(Ics.parseIcs(folded).events[0].description).toBe('first partsecond part')
  })

  it('handles bare LF line endings', () => {
    expect(Ics.parseIcs(vevent(MINIMAL).replace(/\r\n/g, '\n')).events).toHaveLength(1)
  })

  it('ignores properties outside a VEVENT', () => {
    const text = 'BEGIN:VCALENDAR\r\nSUMMARY:not an event\r\nEND:VCALENDAR'
    expect(Ics.parseIcs(text).events).toEqual([])
  })
})

describe('Ics VEVENT fields', () => {
  const field = (extra) => Ics.parseIcs(vevent([MINIMAL, extra].join('\r\n'))).events[0]

  it('unescapes text in summary, description, and location', () => {
    const e = field('DESCRIPTION:line one\\nline two\\, with comma\\; and semi\\\\slash')
    expect(e.description).toBe('line one\nline two, with comma; and semi\\slash')
    expect(field('LOCATION:Room\\, 2').location).toBe('Room, 2')
  })

  it('marks DATE-valued DTSTART and DTEND as all-day', () => {
    const start = Ics.parseIcs(vevent([
      'UID:u', 'SUMMARY:s', 'DTSTART;VALUE=DATE:20260301', 'DTEND;VALUE=DATE:20260302',
    ].join('\r\n'))).events[0]
    expect(start.allDay).toBe(true)
    expect(start.start).toBe('2026-03-01')
    expect(start.end).toBe('2026-03-02')
  })

  it('keeps DATE-valued recurrence identities, additions, and exclusions as dates', () => {
    const event = Ics.parseIcs(vevent([
      'UID:u', 'SUMMARY:s', 'DTSTART;VALUE=DATE:20260301', 'DTEND;VALUE=DATE:20260303',
      'RDATE;VALUE=DATE:20260310', 'EXDATE;VALUE=DATE:20260312',
      'RECURRENCE-ID;VALUE=DATE:20260315',
    ].join('\r\n'))).events[0]
    expect(event).toMatchObject({
      start: '2026-03-01', end: '2026-03-03', recurrenceId: '2026-03-15',
      recurrenceDates: ['2026-03-10'], exceptionDates: ['2026-03-12'], allDay: true,
    })
  })

  it('defaults a zoneless DATE-TIME to UTC', () => {
    expect(field('DTSTAMP:20260301T080000').dtstamp).toBe('2026-03-01T08:00:00.000Z')
  })

  it('falls back to Date.parse for other formats', () => {
    expect(field('DTSTAMP:2026-03-01T08:00:00.000Z').dtstamp).toBe('2026-03-01T08:00:00.000Z')
  })

  it('keeps an unparseable date verbatim', () => {
    expect(field('DTSTAMP:whenever').dtstamp).toBe('whenever')
  })

  it('strips mailto: and lowercases the organizer', () => {
    expect(field('ORGANIZER:mailto:BOSS@Example.com').organizer).toBe('boss@example.com')
  })

  it('collects attendees with their participation status', () => {
    const e = field([
      'ATTENDEE;CN=Alice;PARTSTAT=ACCEPTED:mailto:alice@example.com',
      'ATTENDEE;PARTSTAT=DECLINED:mailto:bob@example.com',
      'ATTENDEE:mailto:carol@example.com',
    ].join('\r\n'))
    expect(e.attendees).toEqual([
      { email: 'alice@example.com', name: 'Alice', status: 'accepted' },
      { email: 'bob@example.com', name: '', status: 'declined' },
      { email: 'carol@example.com', name: '', status: 'needsAction' },
    ])
  })

  it('skips an attendee without an address', () => {
    expect(field('ATTENDEE;CN=Nobody:').attendees).toEqual([])
  })

  it('maps an unknown PARTSTAT to needsAction', () => {
    expect(field('ATTENDEE;PARTSTAT=SNOOZING:mailto:a@b.com').attendees[0].status).toBe('needsAction')
  })

  it('preserves delegated and completed reply states', () => {
    expect(field('ATTENDEE;PARTSTAT=DELEGATED:mailto:a@b.com').attendees[0].status).toBe('delegated')
    expect(field('ATTENDEE;PARTSTAT=COMPLETED:mailto:a@b.com').attendees[0].status).toBe('completed')
  })

  it('reads a quoted parameter value containing a colon', () => {
    const e = field('ATTENDEE;CN="Doe, Jane: VP";PARTSTAT=TENTATIVE:mailto:jane@example.com')
    expect(e.attendees[0].name).toBe('Doe, Jane: VP')
    expect(e.attendees[0].status).toBe('tentative')
  })

  it('keeps the RRULE verbatim', () => {
    expect(field('RRULE:FREQ=WEEKLY;BYDAY=MO').rrule).toBe('FREQ=WEEKLY;BYDAY=MO')
  })

  it('resolves TZID wall time and preserves recurrence exception fields', () => {
    const parsed = Ics.parseIcs(vevent([
      'UID:u', 'SUMMARY:Zoned',
      'DTSTART;TZID=America/Toronto:20260308T090000',
      'DTEND;TZID=America/Toronto:20260308T100000',
      'RRULE:FREQ=WEEKLY;COUNT=3',
      'RDATE;TZID=America/Toronto:20260312T090000',
      'EXDATE;TZID=America/Toronto:20260315T090000,20260322T090000',
      'SEQUENCE:4',
    ].join('\r\n'))).events[0]
    expect(parsed).toMatchObject({
      start: '2026-03-08T13:00:00.000Z',
      end: '2026-03-08T14:00:00.000Z',
      timeZone: 'America/Toronto',
      recurrenceDates: ['2026-03-12T13:00:00.000Z'],
      exceptionDates: ['2026-03-15T13:00:00.000Z', '2026-03-22T13:00:00.000Z'],
      sequence: 4,
    })
  })

  it('parses RECURRENCE-ID and derives DTEND from DURATION', () => {
    const parsed = Ics.parseIcs(vevent([
      'UID:u', 'SUMMARY:Moved',
      'RECURRENCE-ID;RANGE=THISANDFUTURE;TZID=America/Toronto:20260308T090000',
      'DTSTART;TZID=America/Toronto:20260308T110000',
      'DURATION:PT90M',
    ].join('\r\n'))).events[0]
    expect(parsed).toMatchObject({
      recurrenceId: '2026-03-08T13:00:00.000Z',
      recurrenceRange: 'THISANDFUTURE',
      start: '2026-03-08T15:00:00.000Z',
      end: '2026-03-08T16:30:00.000Z',
    })
  })

  it('maps known statuses and drops unknown ones', () => {
    expect(field('STATUS:CANCELLED').status).toBe('cancelled')
    expect(field('STATUS:CONFIRMED').status).toBe('confirmed')
    expect(field('STATUS:tentative').status).toBe('tentative')
    expect(field('STATUS:WHATEVER').status).toBeUndefined()
  })

  it('ignores unrecognised properties', () => {
    expect(field('X-CUSTOM-PROP:whatever').uid).toBe('evt-1@example.com')
  })
})

describe('Ics.buildIcsReply', () => {
  const input = {
    uid: 'evt-1@example.com',
    organizer: 'boss@example.com',
    attendee: 'alice@example.com',
    status: 'accepted',
    summary: 'Standup',
    start: '2026-03-01T09:00:00Z',
    end: '2026-03-01T09:30:00Z',
  }

  it('emits a METHOD:REPLY calendar', () => {
    const out = Ics.buildIcsReply(input)
    expect(out).toContain('METHOD:REPLY')
    expect(out).toContain('UID:evt-1@example.com')
    expect(out).toContain('DTSTART:20260301T090000Z')
    expect(out).toContain('PARTSTAT=ACCEPTED:mailto:alice@example.com')
  })

  it('maps each RSVP status to its PARTSTAT', () => {
    expect(Ics.buildIcsReply({ ...input, status: 'declined' })).toContain('PARTSTAT=DECLINED')
    expect(Ics.buildIcsReply({ ...input, status: 'tentative' })).toContain('PARTSTAT=TENTATIVE')
  })

  it('preserves date-only values in all-day replies', () => {
    const out = Ics.buildIcsReply({
      ...input, start: '2026-03-01', end: '2026-03-03', allDay: true,
    })
    expect(out).toContain('DTSTART;VALUE=DATE:20260301')
    expect(out).toContain('DTEND;VALUE=DATE:20260303')
    expect(Ics.parseIcs(out).events[0]).toMatchObject({
      start: '2026-03-01', end: '2026-03-03', allDay: true,
    })
  })

  it('includes an escaped CN when a name is given', () => {
    const out = Ics.buildIcsReply({ ...input, attendeeName: 'Jane "JD" Doe' })
    expect(out).toContain('CN="Jane \\"JD\\" Doe"')
  })

  it('round-trips through the parser', () => {
    const cal = Ics.parseIcs(Ics.buildIcsReply(input))
    expect(cal.method).toBe('REPLY')
    expect(cal.events[0].attendees[0]).toEqual({ email: 'alice@example.com', name: '', status: 'accepted' })
  })
})

describe('Ics writable CalDAV resources', () => {
  const event = {
    title: 'Planning, review', start: '2026-08-24T14:00:00.000Z', end: '2026-08-24T15:00:00.000Z',
    allDay: false, location: 'Room 1', description: 'Line one\nLine two', organizer: 'alice@example.com',
    attendees: [{ email: 'bob@example.com', name: 'Bob Example', status: 'accepted' }], recurrence: 'FREQ=WEEKLY;COUNT=3',
    recurrenceDates: [], exceptionDates: [], status: 'confirmed',
  }

  it('builds a parseable event resource with escaped text and attendees', () => {
    const raw = Ics.buildCalendarResource(event, 'uid-1@example.com')
    const parsed = Ics.parseIcs(raw).events[0]
    expect(parsed).toMatchObject({ uid: 'uid-1@example.com', summary: event.title, start: event.start, end: event.end, location: event.location, description: event.description, rrule: event.recurrence, sequence: 0 })
    expect(parsed.attendees[0]).toEqual({ email: 'bob@example.com', name: 'Bob Example', status: 'accepted' })
  })

  it('updates one event while preserving provider properties and nested alarms', () => {
    const raw = Ics.buildCalendarResource(event, 'uid-1@example.com').replace('END:VEVENT', 'X-PROVIDER:keep\r\nBEGIN:VALARM\r\nTRIGGER:-PT10M\r\nACTION:DISPLAY\r\nEND:VALARM\r\nEND:VEVENT')
    const updated = Ics.updateEventResource(raw, { uid: 'uid-1@example.com', event: { ...event, title: 'Changed' } })
    expect(updated).toContain('SUMMARY:Changed')
    expect(updated).toContain('SEQUENCE:1')
    expect(updated).toContain('X-PROVIDER:keep')
    expect(updated).toContain('BEGIN:VALARM')
    expect(Ics.parseIcs(updated).events[0].summary).toBe('Changed')
  })

  it('adds and then excludes one recurrence exception without deleting the series', () => {
    const raw = Ics.buildCalendarResource(event, 'uid-1@example.com')
    const recurrenceId = '2026-08-31T14:00:00.000Z'
    const moved = Ics.updateEventResource(raw, { uid: 'uid-1@example.com', recurrenceId, event: { ...event, recurrence: '', start: '2026-08-31T16:00:00.000Z', end: '2026-08-31T17:00:00.000Z' } })
    expect(Ics.parseIcs(moved).events.some(candidate => candidate.recurrenceId === recurrenceId && candidate.start === '2026-08-31T16:00:00.000Z')).toBe(true)
    const excluded = Ics.deleteEventResource(raw, { uid: 'uid-1@example.com', recurrenceId })
    expect(Ics.parseIcs(excluded).events[0].exceptionDates).toEqual([recurrenceId])
  })

  it('deletes only the selected UID from a shared resource', () => {
    const first = Ics.buildCalendarResource(event, 'uid-1@example.com')
    const secondBlock = Ics.buildCalendarResource({ ...event, title: 'Second' }, 'uid-2@example.com').match(/BEGIN:VEVENT[\s\S]*END:VEVENT/)[0]
    const shared = first.replace('END:VCALENDAR', `${secondBlock}\r\nEND:VCALENDAR`)
    const remaining = Ics.deleteEventResource(shared, { uid: 'uid-1@example.com' })
    expect(remaining).not.toBeNull()
    expect(Ics.parseIcs(remaining).events.map(candidate => candidate.uid)).toEqual(['uid-2@example.com'])
    expect(Ics.deleteEventResource(first, { uid: 'uid-1@example.com' })).toBeNull()
  })
})
