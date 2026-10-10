// @ts-check

import { formatPreferenceDate } from './mail-preferences.js'

export const SNOOZE_PRESETS = Object.freeze([
  { id: 'later-today', label: 'Later Today' },
  { id: 'tonight', label: 'Tonight' },
  { id: 'tomorrow', label: 'Tomorrow' },
  { id: 'this-weekend', label: 'This Weekend' },
  { id: 'next-week', label: 'Next Week' },
  { id: 'next-month', label: 'Next Month' },
])

/** Match Mailspring DateUtils using the browser's local timezone. */
export function resolveSnoozePreset(id, now = new Date()) {
  const date = new Date(now.getTime())
  if (id === 'later-today') {
    date.setHours(date.getHours() + 3, 0, 0, 0)
  } else if (id === 'tonight') {
    if (date.getHours() >= 20) date.setHours(24, 0, 0, 0)
    else date.setHours(20, 0, 0, 0)
  } else if (id === 'tomorrow') {
    date.setDate(date.getDate() + 1); date.setHours(9, 0, 0, 0)
  } else if (id === 'this-weekend') {
    const day = date.getDay()
    date.setDate(date.getDate() + (day === 6 ? 7 : (6 - day || 7))); date.setHours(9, 0, 0, 0)
  } else if (id === 'next-week') {
    const day = date.getDay()
    date.setDate(date.getDate() + (day === 0 ? 1 : 8 - day)); date.setHours(9, 0, 0, 0)
  } else if (id === 'next-month') {
    date.setMonth(date.getMonth() + 1, 1); date.setHours(9, 0, 0, 0)
  } else {
    return null
  }
  return date
}

/** @param {string} id */
export function snoozePresetLabel(id) {
  return SNOOZE_PRESETS.find(option => option.id === id)?.label || 'Custom'
}

/** @param {string | number | Date} value @param {Date} [now] */
export function snoozedUntilLabel(value, now = new Date()) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return 'Snoozed'
  const options = date.getTime() - now.getTime() < 24 * 60 * 60 * 1000
    ? { hour: 'numeric', minute: date.getMinutes() ? '2-digit' : undefined }
    : { month: 'short', day: 'numeric', hour: 'numeric', minute: date.getMinutes() ? '2-digit' : undefined }
  return `Snoozed until ${formatPreferenceDate(date, /** @type {Intl.DateTimeFormatOptions} */ (options))}`
}
