export class NotificationSounds {
  static SOUND_EVENT_IDS = Object.freeze(['new-mail', 'hit-send', 'send', 'confirm'])

  static SOUND_IDS = Object.freeze(['none', 'chime', 'bell', 'pop', 'pulse'])

  static DEFAULT_NOTIFICATION_SOUNDS = Object.freeze({
    'new-mail': 'chime',
    'hit-send': 'pop',
    send: 'pulse',
    confirm: 'bell',
  })

  static #SOUND_ID_SET = new Set(NotificationSounds.SOUND_IDS)

  static #SOUND_EVENT_ID_SET = new Set(NotificationSounds.SOUND_EVENT_IDS)

  /** @param {unknown} value @returns {value is 'none'|'chime'|'bell'|'pop'|'pulse'} */
  static isSoundId(value) {
    return typeof value === 'string' && NotificationSounds.#SOUND_ID_SET.has(value)
  }

  /** @param {unknown} value @returns {value is 'new-mail'|'hit-send'|'send'|'confirm'} */
  static isSoundEventId(value) {
    return typeof value === 'string' && NotificationSounds.#SOUND_EVENT_ID_SET.has(value)
  }

  /**
   * Repository normalization is deliberately forgiving for historical rows;
   * the HTTP boundary uses strict validation before anything is persisted.
   *
   * @param {unknown} value
   */
  static normalizeNotificationSounds(value) {
    const input = value && typeof value === 'object' && !Array.isArray(value)
      ? /** @type {Record<string, unknown>} */ (value)
      : {}
    return Object.fromEntries(NotificationSounds.SOUND_EVENT_IDS.map(event => [
      event,
      NotificationSounds.isSoundId(input[event]) ? input[event] : NotificationSounds.DEFAULT_NOTIFICATION_SOUNDS[event],
    ]))
  }

  /**
   * @param {unknown} value
   * @returns {{ ok: true, sounds: Record<string, string> } | { ok: false, error: string }}
   */
  static validateNotificationSounds(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'notificationSounds must be an object' }
    }
    const input = /** @type {Record<string, unknown>} */ (value)
    const unknown = Object.keys(input).filter(key => !NotificationSounds.isSoundEventId(key))
    if (unknown.length > 0) return { ok: false, error: `Unknown notification sound event: ${unknown[0]}` }
    for (const event of NotificationSounds.SOUND_EVENT_IDS) {
      if (input[event] !== undefined && !NotificationSounds.isSoundId(input[event])) {
        return { ok: false, error: `${event} sound must be one of: none, chime, bell, pop, pulse` }
      }
    }
    return { ok: true, sounds: NotificationSounds.normalizeNotificationSounds(input) }
  }
}
