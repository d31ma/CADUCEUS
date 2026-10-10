import { readFileSync } from 'node:fs'

const CATALOG_URL = new URL('../../../deps/data/keymap.json', import.meta.url)
const VALID_SCOPES = new Set(['global', 'mailbox', 'mail', 'reader', 'reader-find', 'reader-no-find', 'composer', 'calendar'])
const VALID_MODIFIERS = new Set(['alt', 'ctrl', 'meta', 'mod', 'shift'])
const MODIFIER_ALIASES = Object.freeze({ control: 'ctrl', option: 'alt', cmd: 'meta', command: 'meta' })
const NAMED_KEYS = new Set(['backspace', 'delete', 'down', 'enter', 'esc', 'left', 'right', 'space', 'tab', 'up'])
const MAX_BINDING_LENGTH = 64
const MAX_SEQUENCE_LENGTH = 2

/**
 * The browser and HTTP boundary consume one checked-in catalog. A malformed
 * catalog is a deployment error and therefore fails module startup rather than
 * silently widening the accepted preference surface.
 */
const payload = JSON.parse(readFileSync(CATALOG_URL, 'utf8'))
if (payload?.schemaVersion !== 1 || !Array.isArray(payload.actions)) {
  throw new Error('Keyboard command catalog is invalid')
}

/** @type {ReadonlyArray<Readonly<{id:string,label:string,group:string,scope:string,binding:string,allowInInput:boolean}>>} */
export const KEYBOARD_ACTIONS = Object.freeze(payload.actions.map((entry, index, entries) => {
  const binding = normalizeKeyboardBinding(entry?.binding)
  if (!entry || typeof entry.id !== 'string' || entries.findIndex(candidate => candidate?.id === entry.id) !== index ||
      !VALID_SCOPES.has(entry.scope) || !binding || binding === 'off') {
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

const ACTION_BY_ID = new Map(KEYBOARD_ACTIONS.map(action => [action.id, action]))

/** Normalize one portable shortcut or return null when its grammar is unsafe. */
export function normalizeKeyboardBinding(value) {
  if (typeof value !== 'string') return null
  const source = value.trim().toLowerCase().replace(/\s*\+\s*/g, '+').replace(/\s+/g, ' ')
  if (!source || source.length > MAX_BINDING_LENGTH) return null
  if (source === 'off') return 'off'
  const sequence = source.split(' ')
  if (sequence.length > MAX_SEQUENCE_LENGTH) return null
  const normalized = []
  for (const chord of sequence) {
    const pieces = chord.split('+').map(piece => piece.trim())
    if (pieces.some(piece => !piece)) return null
    const modifiers = []
    let key = ''
    for (const rawPiece of pieces) {
      const piece = MODIFIER_ALIASES[rawPiece] || rawPiece
      if (VALID_MODIFIERS.has(piece)) {
        if (modifiers.includes(piece)) return null
        modifiers.push(piece)
      } else {
        if (key) return null
        key = piece === 'escape' ? 'esc' : piece
      }
    }
    if (!key || (!NAMED_KEYS.has(key) && !/^[a-z0-9?!*#/,._-]$/.test(key))) return null
    modifiers.sort()
    normalized.push(`${modifiers.length ? `${modifiers.join('+')}+` : ''}${key}`)
  }
  return normalized.join(' ')
}

function contextsForScope(scope) {
  if (scope === 'global') return ['other', 'mailbox', 'reader', 'reader-find', 'composer', 'calendar']
  if (scope === 'mail') return ['mailbox', 'reader', 'reader-find']
  if (scope === 'reader') return ['reader', 'reader-find']
  if (scope === 'reader-find') return ['reader-find']
  if (scope === 'reader-no-find') return ['reader']
  return [scope]
}

/** @param {Record<string,string>} bindings */
function findConflicts(bindings) {
  const conflicts = []
  for (let leftIndex = 0; leftIndex < KEYBOARD_ACTIONS.length; leftIndex++) {
    const left = KEYBOARD_ACTIONS[leftIndex]
    const binding = bindings[left.id]
    if (!binding || binding === 'off') continue
    for (let rightIndex = leftIndex + 1; rightIndex < KEYBOARD_ACTIONS.length; rightIndex++) {
      const right = KEYBOARD_ACTIONS[rightIndex]
      if (bindings[right.id] !== binding) continue
      const rightContexts = contextsForScope(right.scope)
      if (contextsForScope(left.scope).some(context => rightContexts.includes(context))) {
        conflicts.push({ binding, actionIds: [left.id, right.id], labels: [left.label, right.label] })
      }
    }
  }
  return conflicts
}

/** @param {Record<string,string>} overrides */
function resolvedBindings(overrides) {
  return Object.fromEntries(KEYBOARD_ACTIONS.map(action => [action.id, overrides[action.id] || action.binding]))
}

/**
 * Strict HTTP-boundary validation. Empty strings mean disabled so the native
 * settings input can be cleared without inventing a second transport shape.
 * Defaults are removed from storage, keeping future catalog additions safe.
 *
 * @param {unknown} value
 * @returns {{ok:true,overrides:Record<string,string>}|{ok:false,error:string}}
 */
export function validateKeyboardShortcutOverrides(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'keyboardShortcutOverrides must be an object' }
  }
  const input = /** @type {Record<string, unknown>} */ (value)
  if (Object.keys(input).length > KEYBOARD_ACTIONS.length) {
    return { ok: false, error: `keyboardShortcutOverrides may contain at most ${KEYBOARD_ACTIONS.length} commands` }
  }
  /** @type {Record<string,string>} */
  const overrides = {}
  for (const [actionId, rawBinding] of Object.entries(input)) {
    const action = ACTION_BY_ID.get(actionId)
    if (!action) return { ok: false, error: `Unknown keyboard command: ${actionId}` }
    const binding = normalizeKeyboardBinding(rawBinding === '' ? 'off' : rawBinding)
    if (!binding) return { ok: false, error: `Invalid shortcut for ${action.label}` }
    if (binding !== action.binding) overrides[actionId] = binding
  }
  const conflicts = findConflicts(resolvedBindings(overrides))
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
export function normalizeKeyboardShortcutOverrides(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const input = /** @type {Record<string, unknown>} */ (value)
  /** @type {Record<string,string>} */
  const accepted = {}
  for (const action of KEYBOARD_ACTIONS) {
    if (!(action.id in input)) continue
    const binding = normalizeKeyboardBinding(input[action.id] === '' ? 'off' : input[action.id])
    if (!binding || binding === action.binding) continue
    const candidate = { ...accepted, [action.id]: binding }
    if (findConflicts(resolvedBindings(candidate)).length === 0) Object.assign(accepted, candidate)
  }
  return accepted
}
