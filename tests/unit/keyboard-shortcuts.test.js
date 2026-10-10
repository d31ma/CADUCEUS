import { describe, expect, it } from 'bun:test'
import {
  KEYBOARD_ACTIONS,
  normalizeKeyboardBinding,
  normalizeKeyboardShortcutOverrides,
  validateKeyboardShortcutOverrides,
} from '../../server/shared/users/keyboard-shortcuts.js'

describe('keyboard command contract', () => {
  it('loads one unique, normalized action catalog including every upstream calendar command', () => {
    expect(new Set(KEYBOARD_ACTIONS.map(action => action.id)).size).toBe(KEYBOARD_ACTIONS.length)
    expect(KEYBOARD_ACTIONS.length).toBe(51)
    expect(KEYBOARD_ACTIONS.filter(action => action.id.startsWith('calendar:')).map(action => action.id)).toEqual([
      'calendar:move-event-up', 'calendar:move-event-down', 'calendar:move-event-left', 'calendar:move-event-right',
      'calendar:resize-event-up', 'calendar:resize-event-down', 'calendar:resize-event-left', 'calendar:resize-event-right',
      'calendar:view-day', 'calendar:view-week', 'calendar:view-month', 'calendar:view-agenda',
      'calendar:go-to-today', 'calendar:navigate-next', 'calendar:navigate-previous', 'calendar:refresh-calendars',
    ])
  })

  it('normalizes aliases, shifted keys, named keys, and two-step sequences', () => {
    expect(normalizeKeyboardBinding(' Command + Shift + P ')).toBe('meta+shift+p')
    expect(normalizeKeyboardBinding('g   I')).toBe('g i')
    expect(normalizeKeyboardBinding('Escape')).toBe('esc')
    expect(normalizeKeyboardBinding('?')).toBe('?')
    expect(normalizeKeyboardBinding('g i x')).toBeNull()
    expect(normalizeKeyboardBinding('ctrl+alt')).toBeNull()
  })

  it('keeps context-exclusive duplicates and rejects collisions that can fire together', () => {
    expect(validateKeyboardShortcutOverrides({ 'core:refresh': 'x', 'email:reply-all': 'x' })).toEqual({
      ok: true,
      overrides: { 'core:refresh': 'x', 'email:reply-all': 'x' },
    })
    expect(validateKeyboardShortcutOverrides({ 'core:compose': 'mod+1' })).toMatchObject({
      ok: false,
      error: expect.stringContaining('both use mod+1'),
    })
  })

  it('drops unsafe historical overrides instead of weakening defaults', () => {
    expect(normalizeKeyboardShortcutOverrides({
      'core:compose': 'mod+k',
      'calendar:view-day': 'mod+k',
      'unknown:command': 'x',
      'composer:discard': 'g i x',
    })).toEqual({ 'core:compose': 'mod+k' })
  })
})
