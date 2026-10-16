// @ts-check
/** @import { QuoteProof } from '/models/QuoteProof.js' */
import { safeHtmlFrameContentKey } from './safe-html-frame.js'

/** @import { MatchedQuote } from '/models/MatchedQuote.js' */
const WRAPPERS = /** @type {const} */ (['section', 'signature'])

/** @param {string} html @returns {Element[]} */
function sections(html) {
  return Array.from(new DOMParser().parseFromString(html, 'text/html').body.querySelectorAll('section'))
}

/** Canonical sanitized descendant structure/text/URLs/styles, with raw markers removed. @param {Element} section @returns {string} */
function contentKey(section) {
  return safeHtmlFrameContentKey(section.innerHTML, WRAPPERS)
}

/** @param {string} key @returns {Promise<string>} */
async function digest(key) {
  const bytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(key))
  return Array.from(new Uint8Array(bytes), byte => byte.toString(16).padStart(2, '0')).join('')
}

/** @param {QuoteProof|null|undefined} proof @returns {boolean} */
function validProof(proof) {
  return Boolean(proof && Object.keys(proof).length === 2 && ['reply', 'forward'].includes(proof.mode) && /^[a-f0-9]{64}$/.test(proof.sha256))
}

/** Only call with the separately typed app-generated quote record, never raw Body. @param {string} trustedHTML @returns {Promise<QuoteProof|null>} */
export async function proofFromTrustedQuote(trustedHTML) {
  const section = sections(trustedHTML)[0]
  const mode = section?.getAttribute('data-caduceus-quote')
  if (!section || (mode !== 'reply' && mode !== 'forward')) return null
  return {mode, sha256:await digest(contentKey(section))}
}

/**
 * A proof from an app-owned draft/active frame grants metadata only to its matching content.
 * Missing/changed/removed sections revoke authority; raw data/class/IDs are irrelevant.
 * @param {string} html
 * @param {QuoteProof|null|undefined} proof
 * @returns {Promise<MatchedQuote|null>}
 */
export async function matchQuoteProof(html, proof) {
  if (!proof || !validProof(proof)) return null
  // Establish the existing sanitizer's complete input budget before candidate hashing.
  safeHtmlFrameContentKey(html, WRAPPERS)
  for (const section of sections(html)) {
    const key = contentKey(section)
    if (await digest(key) === proof.sha256) return {key, proof:{mode:proof.mode,sha256:proof.sha256}}
  }
  return null
}
