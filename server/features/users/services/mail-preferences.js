import { Security } from '../../../platform/security/security.js'

export const WORKSPACE_LAYOUTS = Object.freeze(['split', 'list', 'split-vertical'])
export const INTERFACE_SCALES = Object.freeze([0.8, 0.9, 1, 1.1, 1.2])
export const MARK_AS_READ_DELAYS = Object.freeze([0, 500, 2000, -1])
export const BACKSPACE_ACTIONS = Object.freeze(['archive', 'trash'])
export const CONVERSATION_ORDERS = Object.freeze(['ascending', 'descending'])
export const EMAIL_RENDER_MODES = Object.freeze(['theme', 'light', 'dark'])
export const DEFAULT_SEND_ACTIONS = Object.freeze(['send', 'send-and-archive'])
export const DEFAULT_REPLY_ACTIONS = Object.freeze(['reply', 'reply-all'])
export const UNDO_SEND_DELAYS = Object.freeze([0, 5000, 15000, 30000, 60000])
export const SPELLCHECK_LANGUAGES = Object.freeze([
  '', 'af', 'bg', 'ca', 'cs', 'da', 'de', 'el', 'en-AU', 'en-CA', 'en-GB', 'en-US',
  'es', 'et', 'fo', 'fr', 'he', 'hi', 'hr', 'hu', 'it', 'ko', 'lt', 'lv', 'nb', 'nl',
  'pl', 'pt', 'pt-BR', 'ro', 'ru', 'sk', 'sl', 'sq', 'sr', 'sv', 'tr', 'uk', 'vi',
])

export const DEFAULT_MAIL_PREFERENCES = Object.freeze({
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

const FIELDS = new Set(Object.keys(DEFAULT_MAIL_PREFERENCES))

/**
 * Normalize persisted preferences without trusting their shape.
 * @param {unknown} value
 * @param {{email?: string, aliases?: string[], domains?: string[]}} [user]
 * @returns {import('../../../types').MailPreferences}
 */
export function normalizeMailPreferences(value, user = {}) {
  /** @type {Record<string, unknown>} */
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, unknown>} */ (value)
    : {}
  const allowedSenders = ownedSenders(user)
  const allowedDomains = new Set((user.domains || []).map(domain => Security.normalizeDomain(domain)).filter(Boolean))
  const from = normalizeOwnedSender(input.defaultFrom, allowedSenders)
  return {
    workspaceLayout: pick(input.workspaceLayout, WORKSPACE_LAYOUTS, DEFAULT_MAIL_PREFERENCES.workspaceLayout),
    use24HourClock: bool(input.use24HourClock, DEFAULT_MAIL_PREFERENCES.use24HourClock),
    interfaceScale: pickNumber(input.interfaceScale, INTERFACE_SCALES, DEFAULT_MAIL_PREFERENCES.interfaceScale),
    showUnreadForAllFolders: bool(input.showUnreadForAllFolders, DEFAULT_MAIL_PREFERENCES.showUnreadForAllFolders),
    markAsReadDelay: pickNumber(input.markAsReadDelay, MARK_AS_READ_DELAYS, DEFAULT_MAIL_PREFERENCES.markAsReadDelay),
    detailedHeaders: bool(input.detailedHeaders, DEFAULT_MAIL_PREFERENCES.detailedHeaders),
    detailedNames: bool(input.detailedNames, DEFAULT_MAIL_PREFERENCES.detailedNames),
    restrictMessageWidth: bool(input.restrictMessageWidth, DEFAULT_MAIL_PREFERENCES.restrictMessageWidth),
    backspaceAction: pick(input.backspaceAction, BACKSPACE_ACTIONS, DEFAULT_MAIL_PREFERENCES.backspaceAction),
    swipeEnabled: bool(input.swipeEnabled, DEFAULT_MAIL_PREFERENCES.swipeEnabled),
    conversationOrder: pick(input.conversationOrder, CONVERSATION_ORDERS, DEFAULT_MAIL_PREFERENCES.conversationOrder),
    emailRenderMode: pick(input.emailRenderMode, EMAIL_RENDER_MODES, DEFAULT_MAIL_PREFERENCES.emailRenderMode),
    richText: bool(input.richText, DEFAULT_MAIL_PREFERENCES.richText),
    spellcheck: bool(input.spellcheck, DEFAULT_MAIL_PREFERENCES.spellcheck),
    spellcheckLanguage: pick(input.spellcheckLanguage, SPELLCHECK_LANGUAGES, DEFAULT_MAIL_PREFERENCES.spellcheckLanguage),
    defaultSendAction: pick(input.defaultSendAction, DEFAULT_SEND_ACTIONS, DEFAULT_MAIL_PREFERENCES.defaultSendAction),
    defaultReplyAction: pick(input.defaultReplyAction, DEFAULT_REPLY_ACTIONS, DEFAULT_MAIL_PREFERENCES.defaultReplyAction),
    undoSendDelay: pickNumber(input.undoSendDelay, UNDO_SEND_DELAYS, DEFAULT_MAIL_PREFERENCES.undoSendDelay),
    defaultFrom: from,
    autoCcBySender: normalizeRecipientMap(input.autoCcBySender, allowedSenders),
    autoBccBySender: normalizeRecipientMap(input.autoBccBySender, allowedSenders),
    displayAttachmentPreviews: bool(input.displayAttachmentPreviews, DEFAULT_MAIL_PREFERENCES.displayAttachmentPreviews),
    unsnoozeToTop: bool(input.unsnoozeToTop, DEFAULT_MAIL_PREFERENCES.unsnoozeToTop),
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
 * @param {import('../../../types').User} user
 * @returns {{ok: true, preferences: import('../../../types').MailPreferences} | {ok: false, error: string}}
 */
export function validateMailPreferencePatch(value, user) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'mailPreferences must be an object' }
  }
  const unknown = Object.keys(value).find(key => !FIELDS.has(key))
  if (unknown) return { ok: false, error: `Unknown mail preference: ${unknown}` }

  const patch = /** @type {Record<string, unknown>} */ (value)
  for (const [key, fieldValue] of Object.entries(patch)) {
    if (!isValidPreferenceField(key, fieldValue, user)) {
      return { ok: false, error: `Invalid mail preference: ${key}` }
    }
  }
  const candidate = { ...normalizeMailPreferences(user.mailPreferences, user), ...patch }
  const normalized = normalizeMailPreferences(candidate, user)
  return { ok: true, preferences: normalized }
}

function isValidPreferenceField(key, value, user) {
  if (['use24HourClock', 'showUnreadForAllFolders', 'detailedHeaders', 'detailedNames',
    'restrictMessageWidth', 'swipeEnabled', 'richText', 'spellcheck',
    'displayAttachmentPreviews', 'unsnoozeToTop'].includes(key)) return typeof value === 'boolean'
  const catalogs = {
    workspaceLayout: WORKSPACE_LAYOUTS,
    interfaceScale: INTERFACE_SCALES,
    markAsReadDelay: MARK_AS_READ_DELAYS,
    backspaceAction: BACKSPACE_ACTIONS,
    conversationOrder: CONVERSATION_ORDERS,
    emailRenderMode: EMAIL_RENDER_MODES,
    spellcheckLanguage: SPELLCHECK_LANGUAGES,
    defaultSendAction: DEFAULT_SEND_ACTIONS,
    defaultReplyAction: DEFAULT_REPLY_ACTIONS,
    undoSendDelay: UNDO_SEND_DELAYS,
  }
  if (key in catalogs) return catalogs[key].includes(/** @type {never} */ (value))
  if (key === 'defaultFrom') {
    if (value === '') return true
    const sender = Security.normalizeEmailAddress(/** @type {string} */ (value))
    return typeof value === 'string' && Boolean(sender && ownedSenders(user).has(sender))
  }
  if (key === 'autoCcBySender' || key === 'autoBccBySender') {
    return validRecipientMap(value, ownedSenders(user))
  }
  if (key === 'findInMailDisabledDomains') {
    if (!Array.isArray(value) || value.length > 100) return false
    const allowed = new Set((user.domains || []).map(domain => Security.normalizeDomain(domain)).filter(Boolean))
    return value.every(domain => typeof domain === 'string' && allowed.has(Security.normalizeDomain(domain)))
  }
  return false
}

function validRecipientMap(value, allowedSenders) {
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

function ownedSenders(user) {
  return new Set([user.email, ...(user.aliases || [])].map(Security.normalizeEmailAddress).filter(Boolean))
}

function normalizeOwnedSender(value, allowed) {
  if (value == null || value === '') return ''
  const sender = Security.normalizeEmailAddress(String(value))
  return sender && allowed.has(sender) ? sender : ''
}

function normalizeRecipientMap(value, allowedSenders) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const output = {}
  let total = 0
  for (const [rawSender, rawRecipients] of Object.entries(value)) {
    const sender = Security.normalizeEmailAddress(rawSender)
    if (!sender || !allowedSenders.has(sender) || !Array.isArray(rawRecipients)) continue
    const recipients = [...new Set(rawRecipients.map(recipient => Security.normalizeEmailAddress(recipient)).filter(Boolean))].slice(0, 50)
    total += recipients.length
    if (total > 200) break
    if (recipients.length) output[sender] = recipients
  }
  return output
}

function pick(value, allowed, fallback) { return allowed.includes(value) ? value : fallback }
function pickNumber(value, allowed, fallback) { return typeof value === 'number' && allowed.includes(value) ? value : fallback }
function bool(value, fallback) { return typeof value === 'boolean' ? value : fallback }
