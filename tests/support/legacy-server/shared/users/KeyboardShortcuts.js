import { readFileSync } from 'node:fs'

export class KeyboardShortcuts {
  static #CATALOG_URL = new URL('../../deps/data/keymap.json', import.meta.url)

  static #VALID_SCOPES = new Set(['global', 'mailbox', 'mail', 'reader', 'reader-find', 'reader-no-find', 'composer', 'calendar'])

  static #VALID_MODIFIERS = new Set(['alt', 'ctrl', 'meta', 'mod', 'shift'])

  /** @type {Readonly<Record<string, string>>} */
  static #MODIFIER_ALIASES = Object.freeze({ control: 'ctrl', option: 'alt', cmd: 'meta', command: 'meta' })

  static #NAMED_KEYS = new Set(['backspace', 'delete', 'down', 'enter', 'esc', 'left', 'right', 'space', 'tab', 'up'])

  static #MAX_BINDING_LENGTH = 64

  static #MAX_SEQUENCE_LENGTH = 2

  /**
   * The browser and HTTP boundary consume one checked-in catalog. A malformed
   * catalog is a deployment error and therefore fails module startup rather than
   * silently widening the accepted preference surface.
   */
  static #payload = KeyboardShortcuts.#readCatalog()

  /** @returns {{ schemaVersion: number, actions: any[] }} */
  static #readCatalog() {
    const payload = JSON.parse(readFileSync(KeyboardShortcuts.#CATALOG_URL, 'utf8'))
    if (payload?.schemaVersion !== 1 || !Array.isArray(payload.actions)) {
      throw new Error('Keyboard command catalog is invalid')
    }
    return payload
  }

  /** @type {ReadonlyArray<Readonly<{id:string,label:string,group:string,scope:string,binding:string,allowInInput:boolean}>>} */
  static KEYBOARD_ACTIONS = Object.freeze(KeyboardShortcuts.#payload.actions.map((entry, index, entries) => {
    const binding = KeyboardShortcuts.normalizeKeyboardBinding(entry?.binding)
    if (!entry || typeof entry.id !== 'string' || entries.findIndex(candidate => candidate?.id === entry.id) !== index ||
        !KeyboardShortcuts.#VALID_SCOPES.has(entry.scope) || !binding || binding === 'off') {
      throw new Error('Keyboard command catalog is invalid')
    }
    return Object.freeze({
      id: entry.id,
      label: String(entry.label || entry.id),
      group: String(entry.group || 'Other'),
      scope: entry.scope,
      binding,
      allowInInput: entry.allowInInput === true,
    })
  }))

  static #ACTION_BY_ID = new Map(KeyboardShortcuts.KEYBOARD_ACTIONS.map(action => [action.id, action]))

  /** Normalize one portable shortcut or return null when its grammar is unsafe. @param {unknown} value */
  static normalizeKeyboardBinding(value) {
    if (typeof value !== 'string') return null
    const source = value.trim().toLowerCase().replace(/\s*\+\s*/g, '+').replace(/\s+/g, ' ')
    if (!source || source.length > KeyboardShortcuts.#MAX_BINDING_LENGTH) return null
    if (source === 'off') return 'off'
    const sequence = source.split(' ')
    if (sequence.length > KeyboardShortcuts.#MAX_SEQUENCE_LENGTH) return null
    const normalized = []
    for (const chord of sequence) {
      const pieces = chord.split('+').map(piece => piece.trim())
      if (pieces.some(piece => !piece)) return null
      /** @type {string[]} */
      const modifiers = []
      let key = ''
      for (const rawPiece of pieces) {
        const piece = KeyboardShortcuts.#MODIFIER_ALIASES[rawPiece] || rawPiece
        if (KeyboardShortcuts.#VALID_MODIFIERS.has(piece)) {
          if (modifiers.includes(piece)) return null
          modifiers.push(piece)
        } else {
          if (key) return null
          key = piece === 'escape' ? 'esc' : piece
        }
      }
      if (!key || (!KeyboardShortcuts.#NAMED_KEYS.has(key) && !/^[a-z0-9?!*#/,._-]$/.test(key))) return null
      modifiers.sort()
      normalized.push(`${modifiers.length ? `${modifiers.join('+')}+` : ''}${key}`)
    }
    return normalized.join(' ')
  }

  /** @param {string} scope */
  static #contextsForScope(scope) {
    if (scope === 'global') return ['other', 'mailbox', 'reader', 'reader-find', 'composer', 'calendar']
    if (scope === 'mail') return ['mailbox', 'reader', 'reader-find']
    if (scope === 'reader') return ['reader', 'reader-find']
    if (scope === 'reader-find') return ['reader-find']
    if (scope === 'reader-no-find') return ['reader']
    return [scope]
  }

  /** @param {Record<string,string>} bindings */
  static #findConflicts(bindings) {
    const conflicts = []
    for (let leftIndex = 0; leftIndex < KeyboardShortcuts.KEYBOARD_ACTIONS.length; leftIndex++) {
      const left = KeyboardShortcuts.KEYBOARD_ACTIONS[leftIndex]
      const binding = bindings[left.id]
      if (!binding || binding === 'off') continue
      for (let rightIndex = leftIndex + 1; rightIndex < KeyboardShortcuts.KEYBOARD_ACTIONS.length; rightIndex++) {
        const right = KeyboardShortcuts.KEYBOARD_ACTIONS[rightIndex]
        if (bindings[right.id] !== binding) continue
        const rightContexts = KeyboardShortcuts.#contextsForScope(right.scope)
        if (KeyboardShortcuts.#contextsForScope(left.scope).some(context => rightContexts.includes(context))) {
          conflicts.push({ binding, actionIds: [left.id, right.id], labels: [left.label, right.label] })
        }
      }
    }
    return conflicts
  }

  /** @param {Record<string,string>} overrides */
  static #resolvedBindings(overrides) {
    return Object.fromEntries(KeyboardShortcuts.KEYBOARD_ACTIONS.map(action => [action.id, overrides[action.id] || action.binding]))
  }

  /**
   * Strict HTTP-boundary validation. Empty strings mean disabled so the native
   * settings input can be cleared without inventing a second transport shape.
   * Defaults are removed from storage, keeping future catalog additions safe.
   *
   * @param {unknown} value
   * @returns {{ok:true,overrides:Record<string,string>}|{ok:false,error:string}}
   */
  static validateKeyboardShortcutOverrides(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'keyboardShortcutOverrides must be an object' }
    }
    const input = /** @type {Record<string, unknown>} */ (value)
    if (Object.keys(input).length > KeyboardShortcuts.KEYBOARD_ACTIONS.length) {
      return { ok: false, error: `keyboardShortcutOverrides may contain at most ${KeyboardShortcuts.KEYBOARD_ACTIONS.length} commands` }
    }
    /** @type {Record<string,string>} */
    const overrides = {}
    for (const [actionId, rawBinding] of Object.entries(input)) {
      const action = KeyboardShortcuts.#ACTION_BY_ID.get(actionId)
      if (!action) return { ok: false, error: `Unknown keyboard command: ${actionId}` }
      const binding = KeyboardShortcuts.normalizeKeyboardBinding(rawBinding === '' ? 'off' : rawBinding)
      if (!binding) return { ok: false, error: `Invalid shortcut for ${action.label}` }
      if (binding !== action.binding) overrides[actionId] = binding
    }
    const conflicts = KeyboardShortcuts.#findConflicts(KeyboardShortcuts.#resolvedBindings(overrides))
    if (conflicts.length) {
      const conflict = conflicts[0]
      return { ok: false, error: `${conflict.labels.join(' and ')} both use ${conflict.binding}` }
    }
    return { ok: true, overrides }
  }

  /**
   * Historical-row normalization is forgiving but never preserves an invalid or
   * conflicting override. Each candidate is admitted only if the complete
   * effective keymap remains deterministic.
   *
   * @param {unknown} value
   * @returns {Record<string,string>}
   */
  static normalizeKeyboardShortcutOverrides(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
    const input = /** @type {Record<string, unknown>} */ (value)
    /** @type {Record<string,string>} */
    const accepted = {}
    for (const action of KeyboardShortcuts.KEYBOARD_ACTIONS) {
      if (!(action.id in input)) continue
      const binding = KeyboardShortcuts.normalizeKeyboardBinding(input[action.id] === '' ? 'off' : input[action.id])
      if (!binding || binding === action.binding) continue
      const candidate = { ...accepted, [action.id]: binding }
      if (KeyboardShortcuts.#findConflicts(KeyboardShortcuts.#resolvedBindings(candidate)).length === 0) Object.assign(accepted, candidate)
    }
    return accepted
  }
}
