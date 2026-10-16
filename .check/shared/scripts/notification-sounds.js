// @ts-check

export const SOUND_EVENTS = Object.freeze([
  Object.freeze({ id: 'new-mail', label: 'New mail', description: 'A message arrives while CADUCEUS is visible.' }),
  Object.freeze({ id: 'hit-send', label: 'Send pressed', description: 'The send action is accepted for processing.' }),
  Object.freeze({ id: 'send', label: 'Message sent', description: 'The server accepts the outgoing message.' }),
  Object.freeze({ id: 'confirm', label: 'Action confirmed', description: 'An undo or similar action completes.' }),
])

export const SOUND_OPTIONS = Object.freeze([
  Object.freeze({ id: 'none', label: 'None' }),
  Object.freeze({ id: 'chime', label: 'Chime' }),
  Object.freeze({ id: 'bell', label: 'Bell' }),
  Object.freeze({ id: 'pop', label: 'Pop' }),
  Object.freeze({ id: 'pulse', label: 'Pulse' }),
])

export const DEFAULT_NOTIFICATION_SOUNDS = Object.freeze({
  'new-mail': 'chime',
  'hit-send': 'pop',
  send: 'pulse',
  confirm: 'bell',
})

export const NOTIFICATION_SOUNDS_SESSION_KEY = 'caduceus_notification_sounds'

const EVENT_IDS = new Set(SOUND_EVENTS.map(event => event.id))
const SOUND_IDS = new Set(SOUND_OPTIONS.map(sound => sound.id))

/** @param {unknown} value @returns {value is SoundEventId} */
export function isSoundEventId(value) {
  return typeof value === 'string' && EVENT_IDS.has(/** @type {SoundEventId} */ (value))
}

/** @param {unknown} value @returns {value is SoundId} */
export function isSoundId(value) {
  return typeof value === 'string' && SOUND_IDS.has(/** @type {SoundId} */ (value))
}

/** @param {unknown} value @returns {Record<SoundEventId, SoundId>} */
export function normalizeNotificationSounds(value) {
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  return /** @type {Record<SoundEventId, SoundId>} */ (Object.fromEntries(SOUND_EVENTS.map(({ id }) => [
    id,
    isSoundId(input[id]) ? input[id] : DEFAULT_NOTIFICATION_SOUNDS[id],
  ])))
}

/** @param {unknown} value */
export function persistNotificationSounds(value) {
  const normalized = normalizeNotificationSounds(value)
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.setItem(NOTIFICATION_SOUNDS_SESSION_KEY, JSON.stringify(normalized))
  }
  return normalized
}

export function restoreNotificationSounds() {
  let stored
  try {
    stored = typeof sessionStorage === 'undefined'
      ? null
      : JSON.parse(sessionStorage.getItem(NOTIFICATION_SOUNDS_SESSION_KEY) || 'null')
  } catch { stored = null }
  return normalizeNotificationSounds(stored)
}

export function clearNotificationSounds() {
  if (typeof sessionStorage !== 'undefined') sessionStorage.removeItem(NOTIFICATION_SOUNDS_SESSION_KEY)
  return normalizeNotificationSounds(null)
}

const PATTERNS = Object.freeze({
  chime: Object.freeze([[659.25, 0, 0.13], [783.99, 0.11, 0.18], [987.77, 0.23, 0.24]]),
  bell: Object.freeze([[880, 0, 0.28], [1320, 0.02, 0.18]]),
  pop: Object.freeze([[420, 0, 0.07], [620, 0.055, 0.09]]),
  pulse: Object.freeze([[523.25, 0, 0.08], [523.25, 0.12, 0.08]]),
})

/** @type {AudioContext | null} */
let sharedContext = null

/**
 * Synthesize a short local cue. Playback failure is intentionally non-fatal:
 * autoplay policy, unavailable audio hardware, or a suspended background tab
 * must never interrupt mail delivery or user actions.
 *
 * @param {unknown} event
 * @param {{ preferences?: unknown, soundId?: unknown, windowObject?: Window }} [options]
 * @returns {Promise<boolean>}
 */
export async function playNotificationSound(event, options = {}) {
  if (!isSoundEventId(event)) return false
  const preferences = normalizeNotificationSounds(options.preferences ?? restoreNotificationSounds())
  const soundId = isSoundId(options.soundId) ? options.soundId : preferences[event]
  if (soundId === 'none') return false
  const pattern = PATTERNS[soundId]
  if (!pattern) return false
  const windowObject = options.windowObject ?? (typeof window !== 'undefined' ? window : undefined)
  const audioWindow = /** @type {Window & { AudioContext?: typeof AudioContext, webkitAudioContext?: typeof AudioContext }} */ (windowObject)
  const AudioContextClass = audioWindow?.AudioContext || audioWindow?.webkitAudioContext
  if (!AudioContextClass) return false
  try {
    const context = sharedContext && sharedContext.state !== 'closed'
      ? sharedContext
      : new AudioContextClass()
    sharedContext = context
    if (context.state === 'suspended') await context.resume()
    const start = context.currentTime + 0.01
    for (const [frequency, offset, duration] of pattern) {
      const oscillator = context.createOscillator()
      const gain = context.createGain()
      oscillator.type = soundId === 'bell' ? 'sine' : 'triangle'
      oscillator.frequency.setValueAtTime(frequency, start + offset)
      gain.gain.setValueAtTime(0.0001, start + offset)
      gain.gain.exponentialRampToValueAtTime(0.11, start + offset + 0.008)
      gain.gain.exponentialRampToValueAtTime(0.0001, start + offset + duration)
      oscillator.connect(gain)
      gain.connect(context.destination)
      oscillator.start(start + offset)
      oscillator.stop(start + offset + duration + 0.01)
    }
    windowObject?.dispatchEvent?.(new CustomEvent('caduceus:sound-played', {
      detail: { event, soundId },
    }))
    return true
  } catch {
    return false
  }
}
