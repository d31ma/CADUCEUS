export const SOUND_EVENT_IDS = Object.freeze(['new-mail', 'hit-send', 'send', 'confirm'])
export const SOUND_IDS = Object.freeze(['none', 'chime', 'bell', 'pop', 'pulse'])

export const DEFAULT_NOTIFICATION_SOUNDS = Object.freeze({
  'new-mail': 'chime',
  'hit-send': 'pop',
  send: 'pulse',
  confirm: 'bell',
})

const SOUND_ID_SET = new Set(SOUND_IDS)
const SOUND_EVENT_ID_SET = new Set(SOUND_EVENT_IDS)

/** @param {unknown} value @returns {value is 'none'|'chime'|'bell'|'pop'|'pulse'} */
export function isSoundId(value) {
  return typeof value === 'string' && SOUND_ID_SET.has(value)
}

/** @param {unknown} value @returns {value is 'new-mail'|'hit-send'|'send'|'confirm'} */
export function isSoundEventId(value) {
  return typeof value === 'string' && SOUND_EVENT_ID_SET.has(value)
}

/**
 * Repository normalization is deliberately forgiving for historical rows;
 * the HTTP boundary uses strict validation before anything is persisted.
 *
 * @param {unknown} value
 */
export function normalizeNotificationSounds(value) {
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  return Object.fromEntries(SOUND_EVENT_IDS.map(event => [
    event,
    isSoundId(input[event]) ? input[event] : DEFAULT_NOTIFICATION_SOUNDS[event],
  ]))
}

/**
 * @param {unknown} value
 * @returns {{ ok: true, sounds: Record<string, string> } | { ok: false, error: string }}
 */
export function validateNotificationSounds(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'notificationSounds must be an object' }
  }
  const input = /** @type {Record<string, unknown>} */ (value)
  const unknown = Object.keys(input).filter(key => !isSoundEventId(key))
  if (unknown.length > 0) return { ok: false, error: `Unknown notification sound event: ${unknown[0]}` }
  for (const event of SOUND_EVENT_IDS) {
    if (input[event] !== undefined && !isSoundId(input[event])) {
      return { ok: false, error: `${event} sound must be one of: none, chime, bell, pop, pulse` }
    }
  }
  return { ok: true, sounds: normalizeNotificationSounds(input) }
}
