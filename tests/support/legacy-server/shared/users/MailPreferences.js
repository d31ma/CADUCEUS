import { Security } from '../security/Security.js'

export class MailPreferences {
  /** @type {ReadonlyArray<import('../../types').MailPreferences['workspaceLayout']>} */
  static WORKSPACE_LAYOUTS = Object.freeze(['split', 'list', 'split-vertical'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['interfaceScale']>} */
  static INTERFACE_SCALES = Object.freeze([0.8, 0.9, 1, 1.1, 1.2])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['markAsReadDelay']>} */
  static MARK_AS_READ_DELAYS = Object.freeze([0, 500, 2000, -1])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['backspaceAction']>} */
  static BACKSPACE_ACTIONS = Object.freeze(['archive', 'trash'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['conversationOrder']>} */
  static CONVERSATION_ORDERS = Object.freeze(['ascending', 'descending'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['emailRenderMode']>} */
  static EMAIL_RENDER_MODES = Object.freeze(['theme', 'light', 'dark'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['defaultSendAction']>} */
  static DEFAULT_SEND_ACTIONS = Object.freeze(['send', 'send-and-archive'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['defaultReplyAction']>} */
  static DEFAULT_REPLY_ACTIONS = Object.freeze(['reply', 'reply-all'])

  /** @type {ReadonlyArray<import('../../types').MailPreferences['undoSendDelay']>} */
  static UNDO_SEND_DELAYS = Object.freeze([0, 5000, 15000, 30000, 60000])

  static SPELLCHECK_LANGUAGES = Object.freeze([
    '', 'af', 'bg', 'ca', 'cs', 'da', 'de', 'el', 'en-AU', 'en-CA', 'en-GB', 'en-US',
    'es', 'et', 'fo', 'fr', 'he', 'hi', 'hr', 'hu', 'it', 'ko', 'lt', 'lv', 'nb', 'nl',
    'pl', 'pt', 'pt-BR', 'ro', 'ru', 'sk', 'sl', 'sq', 'sr', 'sv', 'tr', 'uk', 'vi',
  ])

  /** @type {Readonly<import('../../types').MailPreferences>} */
  static DEFAULT_MAIL_PREFERENCES = Object.freeze({
    workspaceLayout: 'split',
    use24HourClock: false,
    interfaceScale: 1,
    showUnreadForAllFolders: false,
    markAsReadDelay: 500,
    detailedHeaders: false,
    detailedNames: false,
    restrictMessageWidth: false,
    backspaceAction: 'archive',
    swipeEnabled: true,
    conversationOrder: 'ascending',
    emailRenderMode: 'theme',
    richText: true,
    spellcheck: true,
    spellcheckLanguage: '',
    defaultSendAction: 'send',
    defaultReplyAction: 'reply',
    undoSendDelay: 5000,
    defaultFrom: '',
    autoCcBySender: {},
    autoBccBySender: {},
    displayAttachmentPreviews: true,
    unsnoozeToTop: true,
    findInMailDisabledDomains: [],
  })

  static #FIELDS = new Set(Object.keys(MailPreferences.DEFAULT_MAIL_PREFERENCES))

  /**
   * Normalize persisted preferences without trusting their shape.
   * @param {unknown} value
   * @param {{email?: string, aliases?: string[], domains?: string[]}} [user]
   * @returns {import('../../types').MailPreferences}
   */
  static normalizeMailPreferences(value, user = {}) {
    /** @type {Record<string, unknown>} */
    const input = value && typeof value === 'object' && !Array.isArray(value)
      ? /** @type {Record<string, unknown>} */ (value)
      : {}
    const allowedSenders = MailPreferences.#ownedSenders(user)
    const allowedDomains = new Set((user.domains || []).map(domain => Security.normalizeDomain(domain)).filter(Boolean))
    const from = MailPreferences.#normalizeOwnedSender(input.defaultFrom, allowedSenders)
    return {
      workspaceLayout: MailPreferences.#pick(input.workspaceLayout, MailPreferences.WORKSPACE_LAYOUTS, MailPreferences.DEFAULT_MAIL_PREFERENCES.workspaceLayout),
      use24HourClock: MailPreferences.#bool(input.use24HourClock, MailPreferences.DEFAULT_MAIL_PREFERENCES.use24HourClock),
      interfaceScale: MailPreferences.#pickNumber(input.interfaceScale, MailPreferences.INTERFACE_SCALES, MailPreferences.DEFAULT_MAIL_PREFERENCES.interfaceScale),
      showUnreadForAllFolders: MailPreferences.#bool(input.showUnreadForAllFolders, MailPreferences.DEFAULT_MAIL_PREFERENCES.showUnreadForAllFolders),
      markAsReadDelay: MailPreferences.#pickNumber(input.markAsReadDelay, MailPreferences.MARK_AS_READ_DELAYS, MailPreferences.DEFAULT_MAIL_PREFERENCES.markAsReadDelay),
      detailedHeaders: MailPreferences.#bool(input.detailedHeaders, MailPreferences.DEFAULT_MAIL_PREFERENCES.detailedHeaders),
      detailedNames: MailPreferences.#bool(input.detailedNames, MailPreferences.DEFAULT_MAIL_PREFERENCES.detailedNames),
      restrictMessageWidth: MailPreferences.#bool(input.restrictMessageWidth, MailPreferences.DEFAULT_MAIL_PREFERENCES.restrictMessageWidth),
      backspaceAction: MailPreferences.#pick(input.backspaceAction, MailPreferences.BACKSPACE_ACTIONS, MailPreferences.DEFAULT_MAIL_PREFERENCES.backspaceAction),
      swipeEnabled: MailPreferences.#bool(input.swipeEnabled, MailPreferences.DEFAULT_MAIL_PREFERENCES.swipeEnabled),
      conversationOrder: MailPreferences.#pick(input.conversationOrder, MailPreferences.CONVERSATION_ORDERS, MailPreferences.DEFAULT_MAIL_PREFERENCES.conversationOrder),
      emailRenderMode: MailPreferences.#pick(input.emailRenderMode, MailPreferences.EMAIL_RENDER_MODES, MailPreferences.DEFAULT_MAIL_PREFERENCES.emailRenderMode),
      richText: MailPreferences.#bool(input.richText, MailPreferences.DEFAULT_MAIL_PREFERENCES.richText),
      spellcheck: MailPreferences.#bool(input.spellcheck, MailPreferences.DEFAULT_MAIL_PREFERENCES.spellcheck),
      spellcheckLanguage: MailPreferences.#pick(input.spellcheckLanguage, MailPreferences.SPELLCHECK_LANGUAGES, MailPreferences.DEFAULT_MAIL_PREFERENCES.spellcheckLanguage),
      defaultSendAction: MailPreferences.#pick(input.defaultSendAction, MailPreferences.DEFAULT_SEND_ACTIONS, MailPreferences.DEFAULT_MAIL_PREFERENCES.defaultSendAction),
      defaultReplyAction: MailPreferences.#pick(input.defaultReplyAction, MailPreferences.DEFAULT_REPLY_ACTIONS, MailPreferences.DEFAULT_MAIL_PREFERENCES.defaultReplyAction),
      undoSendDelay: MailPreferences.#pickNumber(input.undoSendDelay, MailPreferences.UNDO_SEND_DELAYS, MailPreferences.DEFAULT_MAIL_PREFERENCES.undoSendDelay),
      defaultFrom: from,
      autoCcBySender: MailPreferences.#normalizeRecipientMap(input.autoCcBySender, allowedSenders),
      autoBccBySender: MailPreferences.#normalizeRecipientMap(input.autoBccBySender, allowedSenders),
      displayAttachmentPreviews: MailPreferences.#bool(input.displayAttachmentPreviews, MailPreferences.DEFAULT_MAIL_PREFERENCES.displayAttachmentPreviews),
      unsnoozeToTop: MailPreferences.#bool(input.unsnoozeToTop, MailPreferences.DEFAULT_MAIL_PREFERENCES.unsnoozeToTop),
      findInMailDisabledDomains: Array.isArray(input.findInMailDisabledDomains)
        ? /** @type {string[]} */ ([...new Set(input.findInMailDisabledDomains
          .map(domain => Security.normalizeDomain(domain))
          .filter(domain => domain && allowedDomains.has(domain)))])
        : [],
    }
  }

  /**
   * Validate a complete or partial client patch and merge it over current state.
   * @param {unknown} value
   * @param {import('../../types').User} user
   * @returns {{ok: true, preferences: import('../../types').MailPreferences} | {ok: false, error: string}}
   */
  static validateMailPreferencePatch(value, user) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return { ok: false, error: 'mailPreferences must be an object' }
    }
    const unknown = Object.keys(value).find(key => !MailPreferences.#FIELDS.has(key))
    if (unknown) return { ok: false, error: `Unknown mail preference: ${unknown}` }

    const patch = /** @type {Record<string, unknown>} */ (value)
    for (const [key, fieldValue] of Object.entries(patch)) {
      if (!MailPreferences.#isValidPreferenceField(key, fieldValue, user)) {
        return { ok: false, error: `Invalid mail preference: ${key}` }
      }
    }
    const candidate = { ...MailPreferences.normalizeMailPreferences(user.mailPreferences, user), ...patch }
    const normalized = MailPreferences.normalizeMailPreferences(candidate, user)
    return { ok: true, preferences: normalized }
  }

  /** @param {string} key @param {unknown} value @param {{email?: string, aliases?: string[], domains?: string[]}} user */
  static #isValidPreferenceField(key, value, user) {
    if (['use24HourClock', 'showUnreadForAllFolders', 'detailedHeaders', 'detailedNames',
      'restrictMessageWidth', 'swipeEnabled', 'richText', 'spellcheck',
      'displayAttachmentPreviews', 'unsnoozeToTop'].includes(key)) return typeof value === 'boolean'
    /** @type {Record<string, readonly (string | number)[]>} */
    const catalogs = {
      workspaceLayout: MailPreferences.WORKSPACE_LAYOUTS,
      interfaceScale: MailPreferences.INTERFACE_SCALES,
      markAsReadDelay: MailPreferences.MARK_AS_READ_DELAYS,
      backspaceAction: MailPreferences.BACKSPACE_ACTIONS,
      conversationOrder: MailPreferences.CONVERSATION_ORDERS,
      emailRenderMode: MailPreferences.EMAIL_RENDER_MODES,
      spellcheckLanguage: MailPreferences.SPELLCHECK_LANGUAGES,
      defaultSendAction: MailPreferences.DEFAULT_SEND_ACTIONS,
      defaultReplyAction: MailPreferences.DEFAULT_REPLY_ACTIONS,
      undoSendDelay: MailPreferences.UNDO_SEND_DELAYS,
    }
    if (key in catalogs) return catalogs[key].includes(/** @type {never} */ (value))
    if (key === 'defaultFrom') {
      if (value === '') return true
      const sender = Security.normalizeEmailAddress(/** @type {string} */ (value))
      return typeof value === 'string' && Boolean(sender && MailPreferences.#ownedSenders(user).has(sender))
    }
    if (key === 'autoCcBySender' || key === 'autoBccBySender') {
      return MailPreferences.#validRecipientMap(value, MailPreferences.#ownedSenders(user))
    }
    if (key === 'findInMailDisabledDomains') {
      if (!Array.isArray(value) || value.length > 100) return false
      const allowed = new Set((user.domains || []).map(domain => Security.normalizeDomain(domain)).filter(Boolean))
      return value.every(domain => typeof domain === 'string' && allowed.has(Security.normalizeDomain(domain)))
    }
    return false
  }

  /** @param {unknown} value @param {ReadonlySet<string | null>} allowedSenders */
  static #validRecipientMap(value, allowedSenders) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return false
    let total = 0
    for (const [rawSender, rawRecipients] of Object.entries(value)) {
      const sender = Security.normalizeEmailAddress(rawSender)
      if (!sender || !allowedSenders.has(sender) || !Array.isArray(rawRecipients) || rawRecipients.length > 50) return false
      for (const recipient of rawRecipients) {
        if (typeof recipient !== 'string' || !Security.normalizeEmailAddress(recipient)) return false
        total++
        if (total > 200) return false
      }
    }
    return true
  }

  /** @param {{email?: string, aliases?: string[]}} user */
  static #ownedSenders(user) {
    return new Set([user.email, ...(user.aliases || [])].map(Security.normalizeEmailAddress).filter(Boolean))
  }

  /** @param {unknown} value @param {ReadonlySet<string | null>} allowed */
  static #normalizeOwnedSender(value, allowed) {
    if (value == null || value === '') return ''
    const sender = Security.normalizeEmailAddress(String(value))
    return sender && allowed.has(sender) ? sender : ''
  }

  /** @param {unknown} value @param {ReadonlySet<string | null>} allowedSenders */
  static #normalizeRecipientMap(value, allowedSenders) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
    /** @type {Record<string, string[]>} */
    const output = {}
    let total = 0
    for (const [rawSender, rawRecipients] of Object.entries(value)) {
      const sender = Security.normalizeEmailAddress(rawSender)
      if (!sender || !allowedSenders.has(sender) || !Array.isArray(rawRecipients)) continue
      const recipients = /** @type {string[]} */ ([...new Set(rawRecipients.map(recipient => Security.normalizeEmailAddress(recipient)).filter(Boolean))].slice(0, 50))
      total += recipients.length
      if (total > 200) break
      if (recipients.length) output[sender] = recipients
    }
    return output
  }

  /** @template {string} Choice @param {unknown} value @param {readonly Choice[]} allowed @param {Choice} fallback @returns {Choice} */
  static #pick(value, allowed, fallback) { return allowed.includes(/** @type {Choice} */ (value)) ? /** @type {Choice} */ (value) : fallback }

  /** @template {number} Choice @param {unknown} value @param {readonly Choice[]} allowed @param {Choice} fallback @returns {Choice} */
  static #pickNumber(value, allowed, fallback) { return typeof value === 'number' && allowed.includes(/** @type {Choice} */ (value)) ? /** @type {Choice} */ (value) : fallback }

  /** @param {unknown} value @param {boolean} fallback */
  static #bool(value, fallback) { return typeof value === 'boolean' ? value : fallback }
}
