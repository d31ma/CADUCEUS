// @ts-check

import { formatDate as formatLocalizedDate } from './localization.js'

/** @type {Readonly<MailPreferences>} */
export const DEFAULT_MAIL_PREFERENCES = Object.freeze({
  workspaceLayout: 'split', use24HourClock: false, interfaceScale: 1,
  showUnreadForAllFolders: false, markAsReadDelay: 500,
  detailedHeaders: false, detailedNames: false, restrictMessageWidth: false,
  backspaceAction: 'archive', swipeEnabled: true, conversationOrder: 'ascending',
  emailRenderMode: 'theme', richText: true, spellcheck: true, spellcheckLanguage: '',
  defaultSendAction: 'send', defaultReplyAction: 'reply', undoSendDelay: 5000,
  defaultFrom: '', autoCcBySender: {}, autoBccBySender: {},
  displayAttachmentPreviews: true, unsnoozeToTop: true, findInMailDisabledDomains: [],
})

/** @type {Readonly<Record<string, readonly any[]>>} */
const CATALOGS = Object.freeze({
  workspaceLayout: ['split', 'list', 'split-vertical'],
  interfaceScale: [0.8, 0.9, 1, 1.1, 1.2],
  markAsReadDelay: [0, 500, 2000, -1],
  backspaceAction: ['archive', 'trash'],
  conversationOrder: ['ascending', 'descending'],
  emailRenderMode: ['theme', 'light', 'dark'],
  spellcheckLanguage: ['', 'af', 'bg', 'ca', 'cs', 'da', 'de', 'el', 'en-AU', 'en-CA', 'en-GB', 'en-US', 'es', 'et', 'fo', 'fr', 'he', 'hi', 'hr', 'hu', 'it', 'ko', 'lt', 'lv', 'nb', 'nl', 'pl', 'pt', 'pt-BR', 'ro', 'ru', 'sk', 'sl', 'sq', 'sr', 'sv', 'tr', 'uk', 'vi'],
  defaultSendAction: ['send', 'send-and-archive'],
  defaultReplyAction: ['reply', 'reply-all'],
  undoSendDelay: [0, 5000, 15000, 30000, 60000],
})

const BOOLEANS = [
  'use24HourClock', 'showUnreadForAllFolders', 'detailedHeaders', 'detailedNames',
  'restrictMessageWidth', 'swipeEnabled', 'richText', 'spellcheck',
  'displayAttachmentPreviews', 'unsnoozeToTop',
]
const SESSION_KEY = 'caduceus_mail_preferences'

/** @param {unknown} value @returns {MailPreferences} */
export function normalizeMailPreferences(value) {
  const input = value && typeof value === 'object' && !Array.isArray(value)
    ? /** @type {Record<string, any>} */ (value)
    : {}
  const normalized = /** @type {MailPreferences} */ ({ ...DEFAULT_MAIL_PREFERENCES })
  for (const [key, allowed] of Object.entries(CATALOGS)) {
    if (allowed.includes(input[key])) normalized[key] = input[key]
  }
  for (const key of BOOLEANS) {
    if (typeof input[key] === 'boolean') normalized[key] = input[key]
  }
  normalized.defaultFrom = typeof input.defaultFrom === 'string' ? input.defaultFrom.trim().toLowerCase() : ''
  normalized.autoCcBySender = normalizeRecipientMap(input.autoCcBySender)
  normalized.autoBccBySender = normalizeRecipientMap(input.autoBccBySender)
  normalized.findInMailDisabledDomains = Array.isArray(input.findInMailDisabledDomains)
    ? [...new Set(input.findInMailDisabledDomains.map(value => String(value).trim().toLowerCase()).filter(Boolean))]
    : []
  return normalized
}

/** @param {unknown} value */
function normalizeRecipientMap(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  /** @type {Record<string, string[]>} */
  const output = {}
  for (const [sender, recipients] of Object.entries(value)) {
    if (!Array.isArray(recipients)) continue
    const address = sender.trim().toLowerCase()
    if (!address) continue
    output[address] = [...new Set(recipients.map(item => String(item).trim().toLowerCase()).filter(Boolean))]
  }
  return output
}

export function restoreMailPreferences() {
  try { return normalizeMailPreferences(JSON.parse(globalThis.sessionStorage?.getItem(SESSION_KEY) || '{}')) }
  catch { return normalizeMailPreferences({}) }
}

/** @param {unknown} value */
export function persistMailPreferences(value) {
  const preferences = normalizeMailPreferences(value)
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(preferences))
  applyMailPreferences(preferences)
  return preferences
}

export function clearMailPreferences() {
  sessionStorage.removeItem(SESSION_KEY)
  applyMailPreferences(DEFAULT_MAIL_PREFERENCES)
}

/** @param {unknown} value */
export function applyMailPreferences(value) {
  const preferences = normalizeMailPreferences(value)
  const root = document.documentElement
  root.dataset.mailLayout = preferences.workspaceLayout
  root.dataset.messageWidth = preferences.restrictMessageWidth ? 'restricted' : 'fluid'
  root.dataset.messageTheme = preferences.emailRenderMode
  root.dataset.conversationOrder = preferences.conversationOrder
  root.dataset.attachmentPreviews = preferences.displayAttachmentPreviews ? 'enabled' : 'disabled'
  root.dataset.richComposer = preferences.richText ? 'enabled' : 'disabled'
  root.dataset.mailSwipe = preferences.swipeEnabled ? 'enabled' : 'disabled'
  root.style.setProperty('--caduceus-interface-scale', String(preferences.interfaceScale))
  return preferences
}

/** @param {string | number | Date} value @param {Intl.DateTimeFormatOptions} [options] */
export function formatPreferenceDate(value, options = {}) {
  const preferences = globalThis.window?._caduceus?.mailPreferences || restoreMailPreferences()
  return formatLocalizedDate(value, { ...options, hour12: !preferences.use24HourClock })
}
