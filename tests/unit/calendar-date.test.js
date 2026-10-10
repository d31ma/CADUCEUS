import { describe, expect, it } from 'bun:test'
import { CalendarDate } from '@/features/calendar/services/CalendarDate.js'
import {
  allDayRangeForDate,
  calendarCursorForEvent,
  eventCoversCalendarDate,
  moveTimedRangeToDate,
  shiftAllDayRangeToDate,
  timedRangeForAllDayDates,
} from '../../client/shared/scripts/calendar-date.js'

describe('CalendarDate', () => {
  it('accepts only canonical, real Gregorian dates', () => {
    expect(CalendarDate.isDate('2026-02-28')).toBe(true)
    expect(CalendarDate.isDate('2026-02-29')).toBe(false)
    expect(CalendarDate.isDate('2028-02-29')).toBe(true)
    expect(CalendarDate.isDate('2026-2-8')).toBe(false)
    expect(CalendarDate.isDate('2026-03-01T00:00:00.000Z')).toBe(false)
  })

  it('does calendar arithmetic without elapsed-millisecond drift', () => {
    expect(CalendarDate.addDays('2026-03-07', 1)).toBe('2026-03-08')
    expect(CalendarDate.addDays('2026-03-08', 1)).toBe('2026-03-09')
    expect(CalendarDate.daysBetween('2026-03-07', '2026-03-10')).toBe(3)
  })

  it('validates exclusive all-day ranges separately from timed ranges', () => {
    expect(CalendarDate.validateRange('2026-03-08', '2026-03-09', true)).toEqual({
      start: '2026-03-08', end: '2026-03-09',
    })
    expect(CalendarDate.validateRange('2026-03-08', '2026-03-08', true)).toBeInstanceOf(Error)
    expect(CalendarDate.validateRange(
      '2026-03-08T14:00:00.000Z', '2026-03-08T15:00:00.000Z', true,
    )).toBeInstanceOf(Error)
    expect(CalendarDate.validateRange(
      '2026-03-08T14:00:00.000Z', '2026-03-08T15:00:00.000Z', false,
    )).toEqual({ start: '2026-03-08T14:00:00.000Z', end: '2026-03-08T15:00:00.000Z' })
  })
})

describe('browser calendar-date operations', () => {
  it('keeps date-only events on their declared dates in every browser timezone', () => {
    const event = { allDay: true, start: '2026-03-08', end: '2026-03-10' }
    expect(eventCoversCalendarDate(event, '2026-03-07')).toBe(false)
    expect(eventCoversCalendarDate(event, '2026-03-08')).toBe(true)
    expect(eventCoversCalendarDate(event, '2026-03-09')).toBe(true)
    expect(eventCoversCalendarDate(event, '2026-03-10')).toBe(false)
  })

  it('moves an all-day span in date space and preserves its covered-day count', () => {
    expect(shiftAllDayRangeToDate({ start: '2026-03-07', end: '2026-03-10' }, '2026-11-01'))
      .toEqual({ start: '2026-11-01', end: '2026-11-04', allDay: true })
  })

  it('preserves local clock time for month-cell timed moves across DST', () => {
    const start = new Date(2026, 2, 7, 10, 30)
    const end = new Date(2026, 2, 7, 11, 15)
    const moved = moveTimedRangeToDate({ start: start.toISOString(), end: end.toISOString() }, '2026-03-09')
    expect(new Date(moved.start).getHours()).toBe(10)
    expect(new Date(moved.start).getMinutes()).toBe(30)
    expect(Date.parse(moved.end) - Date.parse(moved.start)).toBe(45 * 60_000)
    expect(moved.allDay).toBe(false)
  })

  it('converts a timed event dropped on an all-day row to one exclusive date span', () => {
    expect(allDayRangeForDate('2026-08-20')).toEqual({
      start: '2026-08-20', end: '2026-08-21', allDay: true,
    })
  })

  it('converts an exclusive all-day span to local editable times without changing covered dates', () => {
    expect(timedRangeForAllDayDates({ start: '2026-08-20', end: '2026-08-23' })).toEqual({
      start: '2026-08-20T09:00', end: '2026-08-22T10:00', allDay: false,
    })
  })

  it('resolves a date-stable cursor for selected all-day and timed events', () => {
    expect(calendarCursorForEvent({ allDay: true, start: '2026-03-08' })?.getDate()).toBe(8)
    expect(calendarCursorForEvent({ start: '2026-03-08T14:30:00.000Z' })?.toISOString())
      .toBe('2026-03-08T14:30:00.000Z')
    expect(calendarCursorForEvent({ start: 'not-a-date' })).toBeNull()
  })
})
