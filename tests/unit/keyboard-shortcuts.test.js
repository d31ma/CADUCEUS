import { describe, expect, it } from 'bun:test'
import { KeyboardShortcuts } from '../../server/shared/users/KeyboardShortcuts.js'

describe('keyboard command contract', () => {
  it('loads one unique, normalized action catalog including every upstream calendar command', () => {
    expect(new Set(KeyboardShortcuts.KEYBOARD_ACTIONS.map(action => action.id)).size).toBe(KeyboardShortcuts.KEYBOARD_ACTIONS.length)
    expect(KeyboardShortcuts.KEYBOARD_ACTIONS.length).toBe(51)
    expect(KeyboardShortcuts.KEYBOARD_ACTIONS.filter(action => action.id.startsWith('calendar:')).map(action => action.id)).toEqual([
      'calendar:move-event-up', 'calendar:move-event-down', 'calendar:move-event-left', 'calendar:move-event-right',
      'calendar:resize-event-up', 'calendar:resize-event-down', 'calendar:resize-event-left', 'calendar:resize-event-right',
      'calendar:view-day', 'calendar:view-week', 'calendar:view-month', 'calendar:view-agenda',
      'calendar:go-to-today', 'calendar:navigate-next', 'calendar:navigate-previous', 'calendar:refresh-calendars',
    ])
  })

  it('normalizes aliases, shifted keys, named keys, and two-step sequences', () => {
    expect(KeyboardShortcuts.normalizeKeyboardBinding(' Command + Shift + P ')).toBe('meta+shift+p')
    expect(KeyboardShortcuts.normalizeKeyboardBinding('g   I')).toBe('g i')
    expect(KeyboardShortcuts.normalizeKeyboardBinding('Escape')).toBe('esc')
    expect(KeyboardShortcuts.normalizeKeyboardBinding('?')).toBe('?')
    expect(KeyboardShortcuts.normalizeKeyboardBinding('g i x')).toBeNull()
    expect(KeyboardShortcuts.normalizeKeyboardBinding('ctrl+alt')).toBeNull()
  })

  it('keeps context-exclusive duplicates and rejects collisions that can fire together', () => {
    expect(KeyboardShortcuts.validateKeyboardShortcutOverrides({ 'core:refresh': 'x', 'email:reply-all': 'x' })).toEqual({
      ok: true,
      overrides: { 'core:refresh': 'x', 'email:reply-all': 'x' },
    })
    expect(KeyboardShortcuts.validateKeyboardShortcutOverrides({ 'core:compose': 'mod+1' })).toMatchObject({
      ok: false,
      error: expect.stringContaining('both use mod+1'),
    })
  })

  it('drops unsafe historical overrides instead of weakening defaults', () => {
    expect(KeyboardShortcuts.normalizeKeyboardShortcutOverrides({
      'core:compose': 'mod+k',
      'calendar:view-day': 'mod+k',
      'unknown:command': 'x',
      'composer:discard': 'g i x',
    })).toEqual({ 'core:compose': 'mod+k' })
  })
})
