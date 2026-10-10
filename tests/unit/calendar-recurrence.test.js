import { describe, expect, it } from 'bun:test'
import { CalendarRecurrence } from '@/features/calendar/services/recurrence.js'

const event = (over = {}) => ({
  id: 'series-1',
  userEmail: 'alice@example.com',
  title: 'Standup',
  start: '2026-03-01T14:00:00.000Z',
  end: '2026-03-01T14:30:00.000Z',
  timeZone: 'America/Toronto',
  source: 'manual',
  recurrence: 'FREQ=WEEKLY;COUNT=4',
  createdAt: '2026-01-01T00:00:00.000Z',
  updatedAt: '2026-01-01T00:00:00.000Z',
  ...over,
})

const range = {
  start: '2026-02-28T00:00:00.000Z',
  end: '2026-03-31T23:59:59.999Z',
}

describe('CalendarRecurrence.validate', () => {
  it('normalizes a supported RFC 5545 recurrence rule', () => {
    expect(CalendarRecurrence.validate(' rrule:FREQ=weekly;interval=2;byday=mo,we ', event()))
      .toEqual({ ok: true, rule: 'FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE' })
  })

  it('rejects malformed, unboundedly expensive, and unsupported rules', () => {
    expect(CalendarRecurrence.validate('FREQ=HOURLY', event()).ok).toBe(false)
    expect(CalendarRecurrence.validate('FREQ=DAILY;INTERVAL=0', event()).ok).toBe(false)
    expect(CalendarRecurrence.validate('FREQ=DAILY;COUNT=10001', event()).ok).toBe(false)
    expect(CalendarRecurrence.validate('FREQ=DAILY;BYSECOND=1', event()).ok).toBe(false)
  })

  it('accepts an empty rule as a non-recurring event', () => {
    expect(CalendarRecurrence.validate('', event())).toEqual({ ok: true, rule: '' })
  })
})

describe('CalendarRecurrence.expand', () => {
  it('expands all-day recurrence as date-only exclusive ranges across DST', () => {
    const occurrences = CalendarRecurrence.expand([event({
      allDay: true,
      start: '2026-03-07',
      end: '2026-03-10',
      recurrence: 'FREQ=DAILY;COUNT=3',
      recurrenceDates: ['2026-03-12'],
      exceptionDates: ['2026-03-08'],
    })], range)
    expect(occurrences.map(item => ({
      start: item.start, end: item.end, recurrenceId: item.recurrenceId,
    }))).toEqual([
      { start: '2026-03-07', end: '2026-03-10', recurrenceId: '2026-03-07' },
      { start: '2026-03-09', end: '2026-03-12', recurrenceId: '2026-03-09' },
      { start: '2026-03-12', end: '2026-03-15', recurrenceId: '2026-03-12' },
    ])
    expect(occurrences.every(item => item.allDay === true)).toBe(true)
  })

  it('matches date-only recurrence exceptions without timestamp coercion', () => {
    const master = event({
      allDay: true, start: '2026-03-07', end: '2026-03-08', recurrence: 'FREQ=DAILY;COUNT=3',
    })
    const exception = event({
      id: 'all-day-exception', seriesId: 'series-1', recurrenceId: '2026-03-08',
      allDay: true, start: '2026-03-10', end: '2026-03-11', recurrence: '', title: 'Moved day',
    })
    const occurrences = CalendarRecurrence.expand([master, exception], range)
    expect(occurrences.find(item => item.recurrenceId === '2026-03-08')).toMatchObject({
      id: 'all-day-exception', start: '2026-03-10', end: '2026-03-11', isException: true,
    })
  })

  it('keeps recurrence identity when an explicit occurrence edit changes representation', () => {
    const allDayMaster = event({
      allDay: true, start: '2026-03-07', end: '2026-03-08', recurrence: 'FREQ=DAILY;COUNT=3',
    })
    const timedException = event({
      id: 'timed-exception', seriesId: 'series-1', recurrenceId: '2026-03-08',
      allDay: false, start: '2026-03-10T14:00:00.000Z', end: '2026-03-10T15:00:00.000Z',
      recurrence: '', title: 'Timed occurrence',
    })
    expect(CalendarRecurrence.expand([allDayMaster, timedException], range)
      .find(item => item.recurrenceId === '2026-03-08')).toMatchObject({
      id: 'timed-exception', allDay: false, isException: true,
    })

    const allDayException = event({
      id: 'date-exception', seriesId: 'series-1', recurrenceId: '2026-03-08T13:00:00.000Z',
      allDay: true, start: '2026-03-10', end: '2026-03-11', recurrence: '', title: 'All-day occurrence',
    })
    expect(CalendarRecurrence.expand([event(), allDayException], range)
      .find(item => item.recurrenceId === '2026-03-08T13:00:00.000Z')).toMatchObject({
      id: 'date-exception', allDay: true, isException: true,
    })
  })

  it('expands a weekly series into stable occurrence identities', () => {
    const occurrences = CalendarRecurrence.expand([event()], range)
    expect(occurrences.map(item => item.start)).toEqual([
      '2026-03-01T14:00:00.000Z',
      '2026-03-08T13:00:00.000Z',
      '2026-03-15T13:00:00.000Z',
      '2026-03-22T13:00:00.000Z',
    ])
    expect(occurrences.every(item => item.seriesId === 'series-1')).toBe(true)
    expect(new Set(occurrences.map(item => item.id)).size).toBe(4)
    expect(occurrences[0]).toMatchObject({ isRecurring: true, isException: false })
  })

  it('supports BYDAY and preserves duration through a DST transition', () => {
    const occurrences = CalendarRecurrence.expand([event({
      start: '2026-03-02T14:15:00.000Z',
      end: '2026-03-02T15:45:00.000Z',
      recurrence: 'FREQ=WEEKLY;COUNT=3;BYDAY=MO,WE',
    })], range)
    expect(occurrences.map(item => item.start)).toEqual([
      '2026-03-02T14:15:00.000Z',
      '2026-03-04T14:15:00.000Z',
      '2026-03-09T13:15:00.000Z',
    ])
    expect(occurrences.every(item => Date.parse(item.end) - Date.parse(item.start) === 90 * 60_000)).toBe(true)
  })

  it('applies explicit recurrence dates and exclusions', () => {
    const occurrences = CalendarRecurrence.expand([event({
      recurrence: 'FREQ=WEEKLY;COUNT=3',
      recurrenceDates: ['2026-03-05T14:00:00.000Z'],
      exceptionDates: ['2026-03-08T13:00:00.000Z'],
    })], range)
    expect(occurrences.map(item => item.start)).toEqual([
      '2026-03-01T14:00:00.000Z',
      '2026-03-05T14:00:00.000Z',
      '2026-03-15T13:00:00.000Z',
    ])
  })

  it('replaces one generated occurrence with an edited exception', () => {
    const override = event({
      id: 'exception-1',
      seriesId: 'series-1',
      recurrenceId: '2026-03-08T13:00:00.000Z',
      recurrence: '',
      start: '2026-03-10T16:00:00.000Z',
      end: '2026-03-10T17:00:00.000Z',
      title: 'Moved standup',
    })
    const occurrences = CalendarRecurrence.expand([event(), override], range)
    expect(occurrences).toHaveLength(4)
    expect(occurrences.find(item => item.recurrenceId === override.recurrenceId)).toMatchObject({
      id: 'exception-1',
      title: 'Moved standup',
      isException: true,
      seriesId: 'series-1',
    })
  })

  it('removes a generated occurrence with a cancelled exception', () => {
    const cancelled = event({
      id: 'exception-cancelled',
      seriesId: 'series-1',
      recurrenceId: '2026-03-08T13:00:00.000Z',
      recurrence: '',
      status: 'cancelled',
    })
    const occurrences = CalendarRecurrence.expand([event(), cancelled], range)
    expect(occurrences).toHaveLength(3)
    expect(occurrences.some(item => item.recurrenceId === cancelled.recurrenceId)).toBe(false)
  })

  it('clips ordinary and recurring events to the requested window', () => {
    const ordinary = event({ id: 'single', recurrence: '', start: '2026-04-01T09:00:00.000Z', end: '2026-04-01T10:00:00.000Z' })
    expect(CalendarRecurrence.expand([ordinary, event()], {
      start: '2026-03-08T00:00:00.000Z',
      end: '2026-03-09T23:59:59.999Z',
    }).map(item => item.start)).toEqual(['2026-03-08T13:00:00.000Z'])
  })

  it('rejects invalid and excessive query windows', () => {
    expect(() => CalendarRecurrence.expand([event()], { start: 'bad', end: range.end })).toThrow(/range/)
    expect(() => CalendarRecurrence.expand([event()], {
      start: '2020-01-01T00:00:00.000Z',
      end: '2030-01-01T00:00:00.000Z',
    })).toThrow(/range/)
  })
})
