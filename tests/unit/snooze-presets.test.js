import { describe, expect, it } from 'bun:test'
import { SnoozePresetsUtils } from '../../client/shared/utils/SnoozePresetsUtils.js'

const resolveSnoozePreset = (...args) => SnoozePresetsUtils.resolveSnoozePreset(...args)
const snoozePresetLabel = (...args) => SnoozePresetsUtils.snoozePresetLabel(...args)
const snoozedUntilLabel = (...args) => SnoozePresetsUtils.snoozedUntilLabel(...args)

describe('Mailspring-compatible snooze presets', () => {
  const at = (day, hour, minute = 17) => new Date(2026, 7, day, hour, minute, 42, 99)

  it('rounds Later Today to three hours from now and Tonight to 8 PM', () => {
    const later = resolveSnoozePreset('later-today', at(12, 10))
    expect([later.getHours(), later.getMinutes(), later.getSeconds()]).toEqual([13, 0, 0])
    const tonight = resolveSnoozePreset('tonight', at(12, 10))
    expect([tonight.getDate(), tonight.getHours(), tonight.getMinutes()]).toEqual([12, 20, 0])
  })

  it('uses midnight for Tonight after 8 PM and 9 AM for Tomorrow', () => {
    const tonight = resolveSnoozePreset('tonight', at(12, 21))
    expect([tonight.getDate(), tonight.getHours()]).toEqual([13, 0])
    const tomorrow = resolveSnoozePreset('tomorrow', at(12, 21))
    expect([tomorrow.getDate(), tomorrow.getHours()]).toEqual([13, 9])
  })

  it('uses the next Saturday, Monday, and first day of the next month', () => {
    const friday = new Date(2026, 7, 14, 12) // Friday
    expect(resolveSnoozePreset('this-weekend', friday).getDate()).toBe(15)
    expect(resolveSnoozePreset('next-week', friday).getDate()).toBe(17)
    const nextMonth = resolveSnoozePreset('next-month', friday)
    expect([nextMonth.getMonth(), nextMonth.getDate(), nextMonth.getHours()]).toEqual([8, 1, 9])
  })

  it('moves a Saturday weekend preset to the following Saturday', () => {
    const saturday = new Date(2026, 7, 15, 12)
    expect(resolveSnoozePreset('this-weekend', saturday).getDate()).toBe(22)
  })

  it('rejects unknown presets and formats accessible labels', () => {
    expect(resolveSnoozePreset('never', at(12, 10))).toBeNull()
    expect(snoozePresetLabel('next-month')).toBe('Next Month')
    expect(snoozePresetLabel('never')).toBe('Custom')
    expect(snoozedUntilLabel('not-a-date')).toBe('Snoozed')
    expect(snoozedUntilLabel(at(12, 11), at(12, 10))).toMatch(/^Snoozed until /)
  })
})
