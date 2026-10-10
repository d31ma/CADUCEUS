import { describe, expect, it } from 'bun:test'
import { ProviderCalendarService } from '../../server/features/calendar/services/ProviderCalendarService.js'
import { ProviderCalendarRules } from '../../server/shared/calendar/ProviderCalendarRules.js'

describe('provider calendar projection', () => {
  it('parses recurring CalDAV resources without exposing raw calendar data', async () => {
    const boundary = { calendarResources: async token => ({ resources: [{
      id: 'caldav_resource', accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', accountLabel: 'Work', accountColor: '#336699',
      mailboxAddress: 'alice@remote.example', calendarId: 'calendar_work', calendarName: 'Work calendar', calendarColor: '#112233',
      updatedAt: '2026-08-22T12:00:00.000Z', rawICalendar: [
        'BEGIN:VCALENDAR', 'BEGIN:VEVENT', 'UID:weekly', 'SUMMARY:Weekly', 'DTSTART:20990301T090000Z', 'DTEND:20990301T100000Z',
        'RRULE:FREQ=WEEKLY;COUNT=2', 'END:VEVENT', 'END:VCALENDAR', '',
      ].join('\r\n'),
    }] }) }
    const events = await new ProviderCalendarService(boundary).list('secret-token')
    expect(events).toHaveLength(1)
    expect(events[0]).toMatchObject({ source: 'provider', title: 'Weekly', recurrence: 'FREQ=WEEKLY;COUNT=2', readOnly: false, accountLabel: 'Work', calendarName: 'Work calendar' })
    expect(events[0].rawICalendar).toBeUndefined()
  })

  it('prefers authoritative provider events over imported invite copies', () => {
    const local = [{ id: 'invite', source: 'invite', icsUid: 'same' }, { id: 'manual', source: 'manual', icsUid: '' }]
    const provider = [{ id: 'provider', source: 'provider', icsUid: 'same' }]
    expect(ProviderCalendarRules.mergeCalendarEvents(local, provider).map(event => event.id)).toEqual(['manual', 'provider'])
  })

  it('assigns independent stable identities to multiple UIDs in one resource', async () => {
    const boundary = { calendarResources: async () => ({ resources: [{
      id: 'caldav_shared_resource', accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', mailboxAddress: 'alice@remote.example',
      calendarId: 'calendar_work', updatedAt: '2026-08-22T12:00:00.000Z', rawICalendar: [
        'BEGIN:VCALENDAR',
        'BEGIN:VEVENT', 'UID:first', 'SUMMARY:First', 'DTSTART:20990301T090000Z', 'DTEND:20990301T100000Z', 'END:VEVENT',
        'BEGIN:VEVENT', 'UID:second', 'SUMMARY:Second', 'DTSTART:20990301T110000Z', 'DTEND:20990301T120000Z', 'END:VEVENT',
        'END:VCALENDAR', '',
      ].join('\r\n'),
    }] }) }
    const events = await new ProviderCalendarService(boundary).list('secret-token')
    expect(events.map(event => event.icsUid)).toEqual(['first', 'second'])
    expect(new Set(events.map(event => event.id)).size).toBe(2)
  })

  it('queues conditional writes and waits for the durable calendar journal', async () => {
    const raw = [
      'BEGIN:VCALENDAR', 'VERSION:2.0', 'BEGIN:VEVENT', 'UID:one', 'SEQUENCE:2', 'SUMMARY:Before',
      'DTSTART:20990301T090000Z', 'DTEND:20990301T100000Z', 'X-PROVIDER:keep', 'END:VEVENT', 'END:VCALENDAR', '',
    ].join('\r\n')
    const calls = []
    const boundary = {
      calendarResources: async () => ({
        calendars: [{ accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', calendarId: 'calendar_work', calendarName: 'Work', writable: true }],
        resources: [{ id: 'caldav_resource', etag: '"2"', accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', mailboxAddress: 'alice@example.com', calendarId: 'calendar_work', calendarName: 'Work', updatedAt: '2026-08-22T12:00:00.000Z', rawICalendar: raw }],
      }),
      mutateCalendar: async (token, accountId, input) => { calls.push({ token, accountId, input }); return { id: 'calmut_one', state: 'queued' } },
      calendarMutation: async () => ({ id: 'calmut_one', state: 'applied' }),
    }
    const service = new ProviderCalendarService(boundary)
    const [event] = await service.list('token')
    await service.update('token', event.id, { title: 'After', start: event.start, end: event.end }, '', 'request-1')
    expect(calls).toHaveLength(1)
    expect(calls[0]).toMatchObject({ accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', input: { kind: 'update', resourceId: 'caldav_resource', expectedEtag: '"2"', idempotencyKey: 'request-1' } })
    expect(calls[0].input.rawICalendar).toContain('SUMMARY:After')
    expect(calls[0].input.rawICalendar).toContain('SEQUENCE:3')
    expect(calls[0].input.rawICalendar).toContain('X-PROVIDER:keep')
  })
})
